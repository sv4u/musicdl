package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Logs handles GET /api/logs - Stream/fetch logs with filtering.
func (h *Handlers) Logs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	logLevel := query.Get("level")
	searchQuery := query.Get("search")
	startTime := query.Get("start_time")
	endTime := query.Get("end_time")
	maxLines := 1000 // Default max lines
	if maxLinesStr := query.Get("max_lines"); maxLinesStr != "" {
		if parsed, err := fmt.Sscanf(maxLinesStr, "%d", &maxLines); err != nil || parsed != 1 {
			maxLines = 1000
		}
	}

	// Read and filter logs
	reader := NewLogReader(h.logPath)
	entries, err := reader.ReadLogs(logLevel, searchQuery, startTime, endTime, maxLines)
	if err != nil {
		response := map[string]interface{}{
			"error":   "Failed to read logs",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert entries to JSON-serializable format
	logsJSON := make([]map[string]interface{}, len(entries))
	for i, entry := range entries {
		logsJSON[i] = map[string]interface{}{
			"timestamp": entry.Timestamp.Unix(),
			"level":     entry.Level,
			"message":   entry.Message,
			"fields":    entry.Fields,
			"raw":       entry.Raw,
		}
	}

	response := map[string]interface{}{
		"logs":    logsJSON,
		"count":   len(logsJSON),
		"filters": map[string]interface{}{
			"level":      logLevel,
			"search":     searchQuery,
			"start_time": startTime,
			"end_time":   endTime,
			"max_lines":  maxLines,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// LogsStream handles GET /api/logs/stream - Server-Sent Events (SSE) for real-time logs.
func (h *Handlers) LogsStream(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	query := r.URL.Query()
	logLevel := query.Get("level")
	searchQuery := query.Get("search")

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

	// Send initial connection message
	initialMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Log stream connected",
		"time":    time.Now().Unix(),
	}
	data, _ := json.Marshal(initialMsg)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// Check if log file exists
	if _, err := os.Stat(h.logPath); os.IsNotExist(err) {
		// File doesn't exist, send message and close
		msg := map[string]interface{}{
			"type":    "error",
			"message": "Log file does not exist",
		}
		data, _ := json.Marshal(msg)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Channel to signal when client disconnects
	ctx := r.Context()
	done := make(chan bool)
	var mu sync.Mutex
	closed := false

	// Monitor client connection
	go func() {
		<-ctx.Done()
		mu.Lock()
		closed = true
		mu.Unlock()
		done <- true
	}()

	reader := NewLogReader(h.logPath)
	
	// Poll for new lines (simple approach - can be optimized with file watching)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Track last file position
	var lastPos int64 = 0
	fileInfo, err := os.Stat(h.logPath)
	if err == nil {
		lastPos = fileInfo.Size()
	}

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			mu.Lock()
			if closed {
				mu.Unlock()
				return
			}
			mu.Unlock()

			// Check current file size
			fileInfo, err := os.Stat(h.logPath)
			if err != nil {
				continue
			}

			currentSize := fileInfo.Size()
			
			// If file was truncated or reset, start from beginning
			if currentSize < lastPos {
				lastPos = 0
			}

			// If file has grown, read new content
			if currentSize > lastPos {
				file, err := os.Open(h.logPath)
				if err != nil {
					continue
				}

				// Ensure file is always closed
				defer file.Close()

				// Seek to last known position
				if _, err := file.Seek(lastPos, io.SeekStart); err != nil {
					log.Printf("WARN: failed_to_seek_log_file pos=%d error=%v", lastPos, err)
					continue
				}

				scanner := bufio.NewScanner(file)

				// Read new lines
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}

					entry := reader.parseLogLine(line)
					if entry == nil {
						continue
					}

					// Apply filters
					if logLevel != "" && !strings.EqualFold(entry.Level, logLevel) {
						continue
					}

					if searchQuery != "" {
						if !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(searchQuery)) &&
							!strings.Contains(strings.ToLower(entry.Raw), strings.ToLower(searchQuery)) {
							continue
						}
					}

					// Send log entry via SSE
					logMsg := map[string]interface{}{
						"type":      "log",
						"timestamp": entry.Timestamp.Unix(),
						"level":     entry.Level,
						"message":   entry.Message,
						"fields":    entry.Fields,
						"raw":       entry.Raw,
					}
					data, err := json.Marshal(logMsg)
					if err != nil {
						continue
					}

					mu.Lock()
					if !closed {
						fmt.Fprintf(w, "data: %s\n\n", data)
						flusher.Flush()
					}
					mu.Unlock()
				}

				// Check for scanner errors
				if err := scanner.Err(); err != nil {
					log.Printf("WARN: error_reading_log_file error=%v", err)
				}

				lastPos = currentSize
			}
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
