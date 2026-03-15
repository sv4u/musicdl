package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sv4u/musicdl/download/plan"
)

const (
	wsPlanPingInterval     = 30 * time.Second
	wsPlanClientBufferSize = 256
	wsPlanWriteWait        = 10 * time.Second
)

var planUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type PlanItemSnapshot struct {
	ItemID      string                 `json:"item_id"`
	ItemType    string                 `json:"item_type"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	Error       string                 `json:"error,omitempty"`
	RawOutput   string                 `json:"raw_output,omitempty"`
	FilePath    string                 `json:"file_path,omitempty"`
	ParentID    string                 `json:"parent_id,omitempty"`
	ChildIDs    []string               `json:"child_ids,omitempty"`
	SpotifyURL  string                 `json:"spotify_url,omitempty"`
	YouTubeURL  string                 `json:"youtube_url,omitempty"`
	Source      string                 `json:"source,omitempty"`
	SourceURL   string                 `json:"source_url,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Progress    float64                `json:"progress"`
	StartedAt   *int64                 `json:"started_at,omitempty"`
	CompletedAt *int64                 `json:"completed_at,omitempty"`
}

type PlanStats struct {
	Total      int `json:"total"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Skipped    int `json:"skipped"`
}

type PlanSnapshot struct {
	Phase       string             `json:"phase"`
	Items       []PlanItemSnapshot `json:"items"`
	GeneratedAt string             `json:"generated_at,omitempty"`
	ConfigHash  string             `json:"config_hash,omitempty"`
	Stats       PlanStats          `json:"stats"`
}

type PlanMessage struct {
	Type       string        `json:"type"`
	Timestamp  int64         `json:"timestamp"`
	ItemID     string        `json:"item_id,omitempty"`
	Status     string        `json:"status,omitempty"`
	Error      string        `json:"error,omitempty"`
	RawOutput  string        `json:"raw_output,omitempty"`
	FilePath   string        `json:"file_path,omitempty"`
	Name       string        `json:"name,omitempty"`
	Phase      string        `json:"phase,omitempty"`
	Message    string        `json:"message,omitempty"`
	ItemsFound int           `json:"items_found,omitempty"`
	Plan       *PlanSnapshot `json:"plan,omitempty"`
}

type planClientReg struct {
	conn *websocket.Conn
	ch   chan []byte
}

type PlanBroadcaster struct {
	clients       map[*websocket.Conn]chan []byte
	register      chan planClientReg
	unregister    chan *websocket.Conn
	broadcast     chan []byte
	mu            sync.RWMutex
	currentPlan   *PlanSnapshot
	currentPlanMu sync.RWMutex
}

func NewPlanBroadcaster() *PlanBroadcaster {
	pb := &PlanBroadcaster{
		clients:    make(map[*websocket.Conn]chan []byte),
		register:   make(chan planClientReg),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan []byte),
	}
	go pb.run()
	return pb
}

func (pb *PlanBroadcaster) run() {
	for {
		select {
		case reg := <-pb.register:
			pb.mu.Lock()
			pb.clients[reg.conn] = reg.ch
			pb.mu.Unlock()

		case conn := <-pb.unregister:
			pb.mu.Lock()
			if ch, ok := pb.clients[conn]; ok {
				delete(pb.clients, conn)
				close(ch)
			}
			pb.mu.Unlock()

		case data := <-pb.broadcast:
			pb.mu.RLock()
			for _, ch := range pb.clients {
				select {
				case ch <- data:
				default:
					log.Printf("WARN: plan websocket client buffer full, dropping message")
				}
			}
			pb.mu.RUnlock()
		}
	}
}

func (pb *PlanBroadcaster) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := planUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: plan websocket upgrade failed: %v", err)
		return
	}

	ch := make(chan []byte, wsPlanClientBufferSize)

	pb.currentPlanMu.RLock()
	snap := pb.currentPlan
	pb.currentPlanMu.RUnlock()

	if snap != nil {
		msg := PlanMessage{
			Type:      "plan_loaded",
			Timestamp: time.Now().Unix(),
			Plan:      snap,
		}
		data, err := json.Marshal(msg)
		if err == nil {
			conn.SetWriteDeadline(time.Now().Add(wsPlanWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				return
			}
		}
	} else {
		msg := PlanMessage{
			Type:      "phase_change",
			Timestamp: time.Now().Unix(),
			Phase:     "idle",
		}
		data, err := json.Marshal(msg)
		if err == nil {
			conn.SetWriteDeadline(time.Now().Add(wsPlanWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				return
			}
		}
	}

	pb.register <- planClientReg{conn: conn, ch: ch}
	go pb.writePump(conn, ch)
	go pb.readPump(conn)
}

func (pb *PlanBroadcaster) writePump(conn *websocket.Conn, ch chan []byte) {
	ticker := time.NewTicker(wsPlanPingInterval)
	defer func() {
		ticker.Stop()
		pb.unregister <- conn
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-ch:
			conn.SetWriteDeadline(time.Now().Add(wsPlanWriteWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			n := len(ch)
			for i := 0; i < n; i++ {
				if err := conn.WriteMessage(websocket.TextMessage, <-ch); err != nil {
					return
				}
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(wsPlanWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (pb *PlanBroadcaster) readPump(conn *websocket.Conn) {
	defer func() {
		pb.unregister <- conn
	}()
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf("WARN: plan websocket unexpected close: %v", err)
			}
			break
		}
	}
}

func (pb *PlanBroadcaster) broadcastMessage(msg PlanMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WARN: failed to marshal plan message: %v", err)
		return
	}
	pb.broadcast <- data
}

func (pb *PlanBroadcaster) SetPlan(downloadPlan *plan.DownloadPlan, configHash string) {
	items := make([]PlanItemSnapshot, 0, len(downloadPlan.Items))
	for _, item := range downloadPlan.Items {
		items = append(items, planItemToSnapshot(item))
	}
	stats := computePlanStats(items)
	snap := &PlanSnapshot{
		Phase:      "ready",
		Items:      items,
		ConfigHash: configHash,
		Stats:      stats,
	}
	pb.currentPlanMu.Lock()
	pb.currentPlan = snap
	pb.currentPlanMu.Unlock()

	pb.broadcastMessage(PlanMessage{
		Type:      "plan_loaded",
		Timestamp: time.Now().Unix(),
		Plan:      snap,
	})
}

func (pb *PlanBroadcaster) BroadcastItemUpdate(item *plan.PlanItem) {
	snap := planItemToSnapshot(item)
	pb.currentPlanMu.Lock()
	if pb.currentPlan != nil {
		for i := range pb.currentPlan.Items {
			if pb.currentPlan.Items[i].ItemID == item.ItemID {
				pb.currentPlan.Items[i] = snap
				break
			}
		}
		pb.currentPlan.Stats = computePlanStats(pb.currentPlan.Items)
	}
	pb.currentPlanMu.Unlock()

	pb.broadcastMessage(PlanMessage{
		Type:      "item_update",
		Timestamp: time.Now().Unix(),
		ItemID:    item.ItemID,
		Status:    string(item.GetStatus()),
		Error:     item.GetError(),
		RawOutput: item.GetRawOutput(),
		FilePath:  item.GetFilePath(),
		Name:      item.Name,
	})
}

func (pb *PlanBroadcaster) BroadcastPhaseChange(phase string) {
	pb.currentPlanMu.Lock()
	if pb.currentPlan != nil {
		pb.currentPlan.Phase = phase
	}
	pb.currentPlanMu.Unlock()

	pb.broadcastMessage(PlanMessage{
		Type:      "phase_change",
		Timestamp: time.Now().Unix(),
		Phase:     phase,
	})
}

func (pb *PlanBroadcaster) BroadcastPlanProgress(message string, itemsFound int) {
	pb.broadcastMessage(PlanMessage{
		Type:       "plan_progress",
		Timestamp:  time.Now().Unix(),
		Message:    message,
		ItemsFound: itemsFound,
	})
}

func (pb *PlanBroadcaster) GetSnapshot() *PlanSnapshot {
	pb.currentPlanMu.RLock()
	defer pb.currentPlanMu.RUnlock()
	if pb.currentPlan == nil {
		return nil
	}
	items := make([]PlanItemSnapshot, len(pb.currentPlan.Items))
	copy(items, pb.currentPlan.Items)
	return &PlanSnapshot{
		Phase:       pb.currentPlan.Phase,
		Items:       items,
		GeneratedAt: pb.currentPlan.GeneratedAt,
		ConfigHash:  pb.currentPlan.ConfigHash,
		Stats:       pb.currentPlan.Stats,
	}
}

func (pb *PlanBroadcaster) ClientCount() int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return len(pb.clients)
}

func planItemToSnapshot(item *plan.PlanItem) PlanItemSnapshot {
	status := item.GetStatus()
	progress := item.GetProgress()
	errMsg := item.GetError()
	filePath := item.GetFilePath()
	rawOutput := item.GetRawOutput()
	metadata := item.GetMetadata()
	_, startedAt, completedAt := item.GetTimestamps()

	var startedAtUnix *int64
	if startedAt != nil {
		t := startedAt.Unix()
		startedAtUnix = &t
	}
	var completedAtUnix *int64
	if completedAt != nil {
		t := completedAt.Unix()
		completedAtUnix = &t
	}

	childIDs := make([]string, len(item.ChildIDs))
	copy(childIDs, item.ChildIDs)

	return PlanItemSnapshot{
		ItemID:      item.ItemID,
		ItemType:    string(item.ItemType),
		Name:        item.Name,
		Status:      string(status),
		Error:       errMsg,
		RawOutput:   rawOutput,
		FilePath:    filePath,
		ParentID:    item.ParentID,
		ChildIDs:    childIDs,
		SpotifyURL:  item.SpotifyURL,
		YouTubeURL:  item.YouTubeURL,
		Source:      string(item.Source),
		SourceURL:   item.SourceURL,
		Metadata:    metadata,
		Progress:    progress,
		StartedAt:   startedAtUnix,
		CompletedAt: completedAtUnix,
	}
}

func computePlanStats(items []PlanItemSnapshot) PlanStats {
	stats := PlanStats{Total: len(items)}
	for _, item := range items {
		switch item.Status {
		case "completed":
			stats.Completed++
		case "failed":
			stats.Failed++
		case "pending":
			stats.Pending++
		case "in_progress":
			stats.InProgress++
		case "skipped":
			stats.Skipped++
		}
	}
	return stats
}
