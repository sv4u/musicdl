package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Maximum number of buffered messages per client.
	wsClientBufferSize = 256
	// Maximum number of messages retained for reconnecting clients.
	wsHistorySize = 1000
	// Ping interval for keepalive.
	wsPingInterval = 30 * time.Second
	// Write deadline after which a slow client is dropped.
	wsWriteTimeout = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// TODO: In production, validate the Origin header against allowed hosts
	// instead of accepting all origins. The permissive check is fine for local
	// development but should be tightened for network-exposed deployments.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// LogBroadcaster manages WebSocket connections and broadcasts log messages.
type LogBroadcaster struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	history []LogMessage
}

// LogMessage represents a single log entry with metadata.
type LogMessage struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

// wsClient represents a single WebSocket connection.
type wsClient struct {
	conn   *websocket.Conn
	send   chan []byte
	closed bool
	mu     sync.Mutex
}

// NewLogBroadcaster creates a new log broadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		clients: make(map[*wsClient]struct{}),
		history: make([]LogMessage, 0, wsHistorySize),
	}
}

// Broadcast sends a log message to all connected clients and stores in history.
func (lb *LogBroadcaster) Broadcast(msg LogMessage) {
	lb.mu.Lock()
	// Append to history, evict oldest if at capacity
	if len(lb.history) >= wsHistorySize {
		lb.history = lb.history[1:]
	}
	lb.history = append(lb.history, msg)
	lb.mu.Unlock()

	// Use json.Marshal so all fields (Level, Source, Message) are properly escaped.
	// Hand-rolled fmt.Sprintf only escaped Message, leaving Level and Source vulnerable
	// to producing malformed JSON if they contained quotes, backslashes, or newlines.
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WARN: failed to marshal log message: %v", err)
		return
	}
	lb.broadcastRaw(data)
}

// BroadcastString is a convenience method to broadcast a simple string log.
func (lb *LogBroadcaster) BroadcastString(level, message, source string) {
	lb.Broadcast(LogMessage{
		Timestamp: time.Now().Unix(),
		Level:     level,
		Message:   message,
		Source:    source,
	})
}

// GetHistory returns the buffered log history for reconnecting clients.
func (lb *LogBroadcaster) GetHistory() []LogMessage {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	result := make([]LogMessage, len(lb.history))
	copy(result, lb.history)
	return result
}

// broadcastRaw sends raw bytes to all connected clients.
func (lb *LogBroadcaster) broadcastRaw(data []byte) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for client := range lb.clients {
		select {
		case client.send <- data:
		default:
			// Client buffer full, drop message (slow consumer)
			log.Printf("WARN: websocket client buffer full, dropping message")
		}
	}
}

// addClient registers a new WebSocket client.
func (lb *LogBroadcaster) addClient(client *wsClient) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.clients[client] = struct{}{}
}

// removeClient unregisters a WebSocket client.
func (lb *LogBroadcaster) removeClient(client *wsClient) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	delete(lb.clients, client)
}

// ClientCount returns the number of connected WebSocket clients.
func (lb *LogBroadcaster) ClientCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.clients)
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and manages it.
func (lb *LogBroadcaster) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: websocket upgrade failed: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, wsClientBufferSize),
	}

	// Write history directly to the connection rather than queuing into the
	// send channel. The channel buffer (wsClientBufferSize=256) is sized for
	// live message throughput, not for the full history (wsHistorySize=1000).
	// Pushing history through the channel before writePump starts draining
	// would silently drop messages beyond the buffer capacity.
	//
	// History is sent BEFORE adding the client to the broadcast map so that
	// live broadcast messages cannot interleave with the replay. A message
	// arriving between GetHistory() and addClient() may be missed, but a
	// brief gap is far less disruptive than scrambled timestamps.
	history := lb.GetHistory()
	for _, msg := range history {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// Client disconnected during history replay; clean up and bail.
			conn.Close()
			return
		}
	}

	lb.addClient(client)

	// Start writer and reader goroutines
	go lb.writePump(client)
	go lb.readPump(client)
}

// writePump pumps messages from the send channel to the WebSocket connection.
func (lb *LogBroadcaster) writePump(client *wsClient) {
	ticker := time.NewTicker(wsPingInterval)
	defer func() {
		ticker.Stop()
		lb.closeClient(client)
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if !ok {
				// Channel closed
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send each message as its own WebSocket text frame so the
			// client receives valid JSON per frame (no newline-delimited
			// batching that would break JSON.parse on the receiver).
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Drain queued messages, each as a separate frame
			n := len(client.send)
			for i := 0; i < n; i++ {
				if err := client.conn.WriteMessage(websocket.TextMessage, <-client.send); err != nil {
					return
				}
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads messages from the WebSocket (needed to detect disconnections).
func (lb *LogBroadcaster) readPump(client *wsClient) {
	defer lb.closeClient(client)
	client.conn.SetReadLimit(512)
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WARN: websocket unexpected close: %v", err)
			}
			break
		}
	}
}

// closeClient safely closes a client connection.
// Lock ordering: lb.mu (write) THEN client.mu — never reversed.
// Removing the client from the map BEFORE closing the channel guarantees
// that broadcastRaw (which holds lb.mu.RLock while iterating) can never
// send on a closed channel; by the time close(client.send) executes,
// no broadcaster can reach this client.
func (lb *LogBroadcaster) closeClient(client *wsClient) {
	// Step 1: Remove from broadcast map so no new sends can target this client.
	lb.removeClient(client)

	// Step 2: Mark closed and tear down, guarded by client.mu for idempotency
	// (both writePump and readPump may call closeClient).
	client.mu.Lock()
	defer client.mu.Unlock()
	if !client.closed {
		client.closed = true
		close(client.send)
		client.conn.Close()
	}
}
