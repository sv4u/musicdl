package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sv4u/musicdl/download/proto"
)

// Logs handles GET /api/logs - Fetch logs with filtering.
func (h *Handlers) Logs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse query parameters
	query := r.URL.Query()
	logLevel := query.Get("level")
	searchQuery := query.Get("search")
	startTimeStr := query.Get("start_time")
	endTimeStr := query.Get("end_time")

	// Build StreamLogsRequest
	req := &proto.StreamLogsRequest{
		Search: searchQuery,
		Follow: false, // Don't follow for regular fetch
	}

	// Parse log levels
	if logLevel != "" {
		switch strings.ToUpper(logLevel) {
		case "DEBUG":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_DEBUG}
		case "INFO":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_INFO}
		case "WARN", "WARNING":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_WARN}
		case "ERROR":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_ERROR}
		}
	}

	// Parse time filters
	if startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			req.StartTime = &ts
		}
	}
	if endTimeStr != "" {
		if ts, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil {
			req.EndTime = &ts
		}
	}

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	// Check if service is running
	if !svcManager.IsRunning() {
		// Service not running, return empty logs
		response := map[string]interface{}{
			"logs":    []interface{}{},
			"count":   0,
			"filters": map[string]interface{}{
				"level":      logLevel,
				"search":     searchQuery,
				"start_time": startTimeStr,
				"end_time":   endTimeStr,
			},
			"message": "Download service is not running",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		h.logError("Logs", err)
		response := map[string]interface{}{
			"error":   "Failed to connect to download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Stream logs (non-following mode)
	logChan, errChan := client.StreamLogs(ctx, req)

	// Collect entries
	logsJSON := make([]map[string]interface{}, 0)
	maxLines := 1000
	if maxLinesStr := query.Get("max_lines"); maxLinesStr != "" {
		if parsed, err := strconv.Atoi(maxLinesStr); err == nil && parsed > 0 {
			maxLines = parsed
		}
	}

	done := false
	for !done && len(logsJSON) < maxLines {
		select {
		case entry, ok := <-logChan:
			if !ok {
				done = true
				break
			}
			logsJSON = append(logsJSON, map[string]interface{}{
				"timestamp": entry.Timestamp,
				"level":     entry.Level.String(),
				"message":   entry.Message,
				"service":   entry.Service,
				"operation": entry.Operation,
				"error":     entry.Error,
			})
		case err := <-errChan:
			if err != nil {
				h.logError("Logs", err)
				response := map[string]interface{}{
					"error":   "Failed to stream logs",
					"message": err.Error(),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(response)
				return
			}
			done = true
		case <-ctx.Done():
			done = true
		}
	}

	response := map[string]interface{}{
		"logs":    logsJSON,
		"count":   len(logsJSON),
		"filters": map[string]interface{}{
			"level":      logLevel,
			"search":     searchQuery,
			"start_time": startTimeStr,
			"end_time":   endTimeStr,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// LogsStream handles GET /api/logs/stream - Server-Sent Events (SSE) for real-time logs.
func (h *Handlers) LogsStream(w http.ResponseWriter, r *http.Request) {
	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Parse query parameters for filtering
	query := r.URL.Query()
	logLevel := query.Get("level")
	searchQuery := query.Get("search")

	// Send initial connection message
	initialMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Log stream connected",
		"time":    time.Now().Unix(),
	}
	data, _ := json.Marshal(initialMsg)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	ctx := r.Context()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	// Check if service is running
	if !svcManager.IsRunning() {
		msg := map[string]interface{}{
			"type":    "error",
			"message": "Download service is not running",
		}
		data, _ := json.Marshal(msg)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		h.logError("LogsStream", err)
		msg := map[string]interface{}{
			"type":    "error",
			"message": "Failed to connect to download service: " + err.Error(),
		}
		data, _ := json.Marshal(msg)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Build StreamLogsRequest
	req := &proto.StreamLogsRequest{
		Search: searchQuery,
		Follow: true, // Follow mode for streaming
	}

	// Parse log levels
	if logLevel != "" {
		switch strings.ToUpper(logLevel) {
		case "DEBUG":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_DEBUG}
		case "INFO":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_INFO}
		case "WARN", "WARNING":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_WARN}
		case "ERROR":
			req.Levels = []proto.LogLevel{proto.LogLevel_LOG_LEVEL_ERROR}
		}
	}

	// Stream logs via gRPC
	logChan, errChan := client.StreamLogs(ctx, req)

	// Forward log entries to SSE
	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-logChan:
			if !ok {
				return
			}
			logMsg := map[string]interface{}{
				"type":      "log",
				"timestamp": entry.Timestamp,
				"level":     entry.Level.String(),
				"message":   entry.Message,
				"service":   entry.Service,
				"operation": entry.Operation,
				"error":     entry.Error,
			}
			data, err := json.Marshal(logMsg)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case err := <-errChan:
			if err != nil {
				h.logError("LogsStream", err)
				msg := map[string]interface{}{
					"type":    "error",
					"message": "Error streaming logs: " + err.Error(),
				}
				data, _ := json.Marshal(msg)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
			return
		}
	}
}

// LogsPage handles GET /logs - HTML log viewer.
func (h *Handlers) LogsPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>musicdl - Log Viewer</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: #f5f5f5;
            color: #333;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px 0;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header-content {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 20px;
        }
        .header h1 { margin: 0; font-size: 24px; }
        .nav {
            margin-top: 15px;
        }
        .nav a {
            display: inline-block;
            margin-right: 10px;
            padding: 8px 16px;
            background-color: rgba(255,255,255,0.2);
            color: white;
            text-decoration: none;
            border-radius: 4px;
            transition: background-color 0.2s;
        }
        .nav a:hover, .nav a.active {
            background-color: rgba(255,255,255,0.3);
        }
        .container {
            max-width: 1200px;
            margin: 20px auto;
            padding: 0 20px;
        }
        .card {
            background: white;
            border-radius: 8px;
            padding: 24px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .card h2 {
            margin-top: 0;
            margin-bottom: 16px;
            color: #333;
            font-size: 20px;
        }
        .filters {
            display: flex;
            gap: 10px;
            margin-bottom: 16px;
            flex-wrap: wrap;
        }
        .filters input, .filters select {
            padding: 8px 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
        }
        .filters input {
            flex: 1;
            min-width: 200px;
        }
        .log-container {
            background-color: #1e1e1e;
            color: #d4d4d4;
            padding: 16px;
            border-radius: 4px;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 13px;
            max-height: 600px;
            overflow-y: auto;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .log-entry {
            margin-bottom: 4px;
            line-height: 1.5;
        }
        .log-entry.error { color: #f48771; }
        .log-entry.warn { color: #dcdcaa; }
        .log-entry.info { color: #4ec9b0; }
        .log-entry.debug { color: #9cdcfe; }
        .btn {
            display: inline-block;
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            transition: all 0.2s;
            background-color: #007bff;
            color: white;
        }
        .btn:hover { background-color: #0056b3; }
        .btn.active {
            background-color: #28a745;
        }
        .message {
            padding: 12px;
            border-radius: 4px;
            margin-top: 16px;
        }
        .message-info {
            background-color: #d1ecf1;
            color: #0c5460;
            border: 1px solid #bee5eb;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>musicdl Control Platform</h1>
            <div class="nav">
                <a href="/">Dashboard</a>
                <a href="/status">Status</a>
                <a href="/config">Config</a>
                <a href="/logs" class="active">Logs</a>
            </div>
        </div>
    </div>
    <div class="container">
        <div class="card">
            <h2>Log Viewer</h2>
            <div class="filters">
                <input type="text" id="search-input" placeholder="Search logs...">
                <select id="level-filter">
                    <option value="">All Levels</option>
                    <option value="error">Error</option>
                    <option value="warn">Warning</option>
                    <option value="info">Info</option>
                    <option value="debug">Debug</option>
                </select>
                <button class="btn" id="stream-toggle" onclick="toggleStream()">Start Streaming</button>
            </div>
            <div id="log-container" class="log-container">
                <div style="color: #666;">Log viewer is not yet implemented. This will show application logs with filtering and search capabilities.</div>
            </div>
        </div>
    </div>
    <script>
        let streamActive = false;
        let eventSource = null;
        
        function toggleStream() {
            const btn = document.getElementById('stream-toggle');
            if (streamActive) {
                if (eventSource) {
                    eventSource.close();
                    eventSource = null;
                }
                streamActive = false;
                btn.textContent = 'Start Streaming';
                btn.classList.remove('active');
            } else {
                const container = document.getElementById('log-container');
                container.innerHTML = '<div style="color: #666;">Connecting to log stream...</div>';
                
                eventSource = new EventSource('/api/logs/stream');
                eventSource.onmessage = function(event) {
                    const data = JSON.parse(event.data);
                    appendLog(data.message || data.text || JSON.stringify(data));
                };
                eventSource.onerror = function(err) {
                    appendLog('Error: Connection lost', 'error');
                    streamActive = false;
                    btn.textContent = 'Start Streaming';
                    btn.classList.remove('active');
                };
                streamActive = true;
                btn.textContent = 'Stop Streaming';
                btn.classList.add('active');
            }
        }
        
        function appendLog(message, level) {
            const container = document.getElementById('log-container');
            const entry = document.createElement('div');
            entry.className = 'log-entry' + (level ? ' ' + level : '');
            entry.textContent = new Date().toLocaleTimeString() + ' - ' + message;
            container.appendChild(entry);
            container.scrollTop = container.scrollHeight;
        }
        
        function loadLogs() {
            const search = document.getElementById('search-input').value;
            const level = document.getElementById('level-filter').value;
            const params = new URLSearchParams();
            if (search) params.append('search', search);
            if (level) params.append('level', level);
            
            fetch('/api/logs?' + params.toString())
                .then(function(res) {
                    return res.json();
                })
                .then(function(data) {
                    const container = document.getElementById('log-container');
                    if (data.logs && data.logs.length > 0) {
                        container.innerHTML = '';
                        data.logs.forEach(function(log) {
                            appendLog(log.message || JSON.stringify(log), log.level);
                        });
                    } else {
                        container.innerHTML = '<div style="color: #666;">No logs available.</div>';
                    }
                })
                .catch(function(err) {
                    document.getElementById('log-container').innerHTML = 
                        '<div style="color: #f48771;">Error loading logs: ' + err.message + '</div>';
                });
        }
        
        document.getElementById('search-input').addEventListener('keyup', function(e) {
            if (e.key === 'Enter') {
                loadLogs();
            }
        });
        
        document.getElementById('level-filter').addEventListener('change', loadLogs);
        
        // Initial load
        loadLogs();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
