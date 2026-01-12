package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/sv4u/musicdl/download/config"
)

// ConfigGet handles GET /api/config - Read current config file.
func (h *Handlers) ConfigGet(w http.ResponseWriter, r *http.Request) {
	// Read config file
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		h.logError("ConfigGet", err)
		response := map[string]interface{}{
			"error":   "Failed to read config file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return as YAML
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ConfigPut handles PUT /api/config - Update config file (with validation).
func (h *Handlers) ConfigPut(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logError("ConfigPut", err)
		response := map[string]interface{}{
			"error":   "Failed to read request body",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer r.Body.Close()

	// Validate config by loading it
	// Create temporary file for validation
	tmpFile, err := os.CreateTemp("", "musicdl-config-*.yaml")
	if err != nil {
		h.logError("ConfigPut", err)
		response := map[string]interface{}{
			"error":   "Failed to create temporary file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write to temp file
	if _, err := tmpFile.Write(body); err != nil {
		h.logError("ConfigPut", err)
		response := map[string]interface{}{
			"error":   "Failed to write temporary file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	tmpFile.Close()

	// Validate by loading
	_, err = config.LoadConfig(tmpFile.Name())
	if err != nil {
		// Validation failed
		response := map[string]interface{}{
			"error":   "Config validation failed",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Backup original config (optional, but good practice)
	backupPath := h.configPath + ".backup"
	if _, err := os.Stat(h.configPath); err == nil {
		// File exists, create backup
		originalData, err := os.ReadFile(h.configPath)
		if err == nil {
			if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
				// Log warning but continue - backup is optional
				h.logError("ConfigPut backup", err)
			}
		}
	}

	// Write validated config to file
	if err := os.WriteFile(h.configPath, body, 0644); err != nil {
		h.logError("ConfigPut", err)
		response := map[string]interface{}{
			"error":   "Failed to write config file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Invalidate service so it reinitializes with new config
	h.serviceMu.Lock()
	h.service = nil
	h.serviceInit = sync.Once{}
	h.serviceMu.Unlock()

	response := map[string]interface{}{
		"message": "Config updated successfully",
		"path":    h.configPath,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ConfigValidate handles POST /api/config/validate - Validate config without saving.
func (h *Handlers) ConfigValidate(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logError("ConfigValidate", err)
		response := map[string]interface{}{
			"error":   "Failed to read request body",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer r.Body.Close()

	// Create temporary file for validation
	tmpFile, err := os.CreateTemp("", "musicdl-config-*.yaml")
	if err != nil {
		h.logError("ConfigValidate", err)
		response := map[string]interface{}{
			"error":   "Failed to create temporary file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write to temp file
	if _, err := tmpFile.Write(body); err != nil {
		h.logError("ConfigValidate", err)
		response := map[string]interface{}{
			"error":   "Failed to write temporary file",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	tmpFile.Close()

	// Validate by loading
	cfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		// Validation failed
		response := map[string]interface{}{
			"valid":   false,
			"error":   "Config validation failed",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // 200 OK even if invalid (validation endpoint)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validation succeeded
	response := map[string]interface{}{
		"valid":   true,
		"message": "Config is valid",
		"version": cfg.Version,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ConfigPage handles GET /config - HTML config editor.
func (h *Handlers) ConfigPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>musicdl - Config Editor</title>
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
        textarea {
            width: 100%;
            min-height: 500px;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            font-size: 14px;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
            resize: vertical;
        }
        .actions {
            margin-top: 16px;
            display: flex;
            gap: 10px;
        }
        .btn {
            display: inline-block;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            transition: all 0.2s;
        }
        .btn-primary {
            background-color: #007bff;
            color: white;
        }
        .btn-primary:hover { background-color: #0056b3; }
        .btn-secondary {
            background-color: #6c757d;
            color: white;
        }
        .btn-secondary:hover { background-color: #5a6268; }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .message {
            padding: 12px;
            border-radius: 4px;
            margin-top: 16px;
        }
        .message-success {
            background-color: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .message-error {
            background-color: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .loading {
            text-align: center;
            padding: 20px;
            color: #666;
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
                <a href="/config" class="active">Config</a>
                <a href="/logs">Logs</a>
            </div>
        </div>
    </div>
    <div class="container">
        <div class="card">
            <h2>Configuration Editor</h2>
            <div id="editor-container">
                <div class="loading">Loading configuration...</div>
            </div>
            <div class="actions">
                <button class="btn btn-primary" onclick="saveConfig()">Save Configuration</button>
                <button class="btn btn-secondary" onclick="validateConfig()">Validate</button>
                <button class="btn btn-secondary" onclick="loadConfig()">Reload</button>
            </div>
            <div id="message-container"></div>
        </div>
    </div>
    <script>
        let configData = '';
        
        function loadConfig() {
            fetch('/api/config')
                .then(function(res) {
                    if (!res.ok) throw new Error('Failed to load config');
                    return res.text();
                })
                .then(function(data) {
                    configData = data;
                    const container = document.getElementById('editor-container');
                    container.innerHTML = '<textarea id="config-editor" spellcheck="false">' + 
                        data.replace(/</g, '&lt;').replace(/>/g, '&gt;') + 
                        '</textarea>';
                })
                .catch(function(err) {
                    document.getElementById('editor-container').innerHTML = 
                        '<div class="message message-error">Error loading config: ' + err.message + '</div>';
                });
        }
        
        function getEditorValue() {
            const editor = document.getElementById('config-editor');
            return editor ? editor.value : '';
        }
        
        function validateConfig() {
            const content = getEditorValue();
            if (!content) {
                showMessage('No configuration to validate', true);
                return;
            }
            
            fetch('/api/config/validate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-yaml' },
                body: content
            })
            .then(function(res) {
                return res.json();
            })
            .then(function(data) {
                if (data.valid) {
                    showMessage('Configuration is valid', false);
                } else {
                    showMessage('Validation failed: ' + (data.message || data.error), true);
                }
            })
            .catch(function(err) {
                showMessage('Error validating config: ' + err.message, true);
            });
        }
        
        function saveConfig() {
            const content = getEditorValue();
            if (!content) {
                showMessage('No configuration to save', true);
                return;
            }
            
            if (!confirm('Are you sure you want to save this configuration? This will update the config file.')) {
                return;
            }
            
            fetch('/api/config', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/x-yaml' },
                body: content
            })
            .then(function(res) {
                return res.json().then(function(data) {
                    if (res.ok) {
                        showMessage('Configuration saved successfully', false);
                        configData = content;
                    } else {
                        showMessage('Failed to save: ' + (data.message || data.error), true);
                    }
                });
            })
            .catch(function(err) {
                showMessage('Error saving config: ' + err.message, true);
            });
        }
        
        function showMessage(text, isError) {
            const container = document.getElementById('message-container');
            const className = isError ? 'message-error' : 'message-success';
            container.innerHTML = '<div class="message ' + className + '">' + text + '</div>';
            setTimeout(function() {
                container.innerHTML = '';
            }, 5000);
        }
        
        loadConfig();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
