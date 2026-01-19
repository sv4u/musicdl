package handlers

import (
	"net/http"
)

// Dashboard handles GET / - Main dashboard page.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>musicdl - Control Platform</title>
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
        .status-badge {
            display: inline-block;
            padding: 6px 12px;
            border-radius: 4px;
            font-weight: 600;
            font-size: 14px;
            margin-right: 10px;
        }
        .status-idle { background-color: #e3f2fd; color: #1976d2; }
        .status-running { background-color: #e8f5e9; color: #388e3c; }
        .status-stopping { background-color: #fff3e0; color: #f57c00; }
        .status-error { background-color: #ffebee; color: #d32f2f; }
        .actions {
            margin-top: 16px;
        }
        .btn {
            display: inline-block;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            text-decoration: none;
            transition: all 0.2s;
            margin-right: 10px;
        }
        .btn-primary {
            background-color: #007bff;
            color: white;
        }
        .btn-primary:hover { background-color: #0056b3; }
        .btn-danger {
            background-color: #dc3545;
            color: white;
        }
        .btn-danger:hover { background-color: #c82333; }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
            margin-top: 16px;
        }
        .stat-item {
            text-align: center;
            padding: 16px;
            background-color: #f8f9fa;
            border-radius: 4px;
        }
        .stat-value {
            font-size: 32px;
            font-weight: bold;
            color: #667eea;
        }
        .stat-label {
            font-size: 14px;
            color: #666;
            margin-top: 4px;
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
        .message-info {
            background-color: #d1ecf1;
            color: #0c5460;
            border: 1px solid #bee5eb;
        }
        .loading {
            text-align: center;
            padding: 20px;
            color: #666;
        }
        .info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
            margin-top: 16px;
        }
        .info-item {
            padding: 12px;
            background-color: #f8f9fa;
            border-radius: 4px;
        }
        .info-label {
            font-size: 12px;
            color: #666;
            text-transform: uppercase;
            margin-bottom: 4px;
        }
        .info-value {
            font-size: 16px;
            font-weight: 500;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>musicdl Control Platform</h1>
            <div class="nav">
                <a href="/" class="active">Dashboard</a>
                <a href="/status">Status</a>
                <a href="/config">Config</a>
                <a href="/logs">Logs</a>
            </div>
        </div>
    </div>
    <div class="container">
        <div class="card">
            <h2>Download Service Status</h2>
            <div id="status-container">
                <div class="loading">Loading status...</div>
            </div>
            <div class="actions" id="actions-container"></div>
            <div id="message-container"></div>
        </div>
        <div class="card">
            <h2>Configuration</h2>
            <div id="config-container">
                <div class="loading">Loading configuration info...</div>
            </div>
        </div>
        <div class="card">
            <h2>Statistics</h2>
            <div id="stats-container">
                <div class="loading">Loading statistics...</div>
            </div>
        </div>
    </div>
    <script>
        let statusInterval;
        
        function updateStatus() {
            fetch('/api/status')
                .then(res => res.json())
                .then(data => {
                    const container = document.getElementById('status-container');
                    const state = data.state || 'idle';
                    const stateClass = 'status-' + state;
                    
                    container.innerHTML = 
                        '<span class="status-badge ' + stateClass + '">' + state.toUpperCase() + '</span>' +
                        (data.error ? '<span style="color: #d32f2f;">' + data.error + '</span>' : '') +
                        (data.started_at ? '<div style="margin-top: 8px; color: #666; font-size: 14px;">Started: ' + new Date(data.started_at).toLocaleString() + '</div>' : '') +
                        (data.completed_at ? '<div style="margin-top: 4px; color: #666; font-size: 14px;">Completed: ' + new Date(data.completed_at).toLocaleString() + '</div>' : '');
                    
                    const actionsContainer = document.getElementById('actions-container');
                    if (state === 'idle' || state === 'error') {
                        actionsContainer.innerHTML = '<button class="btn btn-primary" onclick="startDownload()">Start Download</button> <button class="btn btn-danger" onclick="resetDownload()">Reset</button>';
                    } else if (state === 'running') {
                        actionsContainer.innerHTML = '<button class="btn btn-danger" onclick="stopDownload()">Stop Download</button> <button class="btn btn-danger" onclick="resetDownload()">Reset</button>';
                    } else {
                        actionsContainer.innerHTML = '<button class="btn btn-primary" disabled>Stopping...</button> <button class="btn btn-danger" onclick="resetDownload()">Reset</button>';
                    }
                    
                    updateStats(data);
                })
                .catch(err => {
                    document.getElementById('status-container').innerHTML = 
                        '<div class="message message-error">Error loading status: ' + err.message + '</div>';
                });
        }
        
        function updateConfig() {
            fetch('/api/config/digest')
                .then(res => res.json())
                .then(data => {
                    const container = document.getElementById('config-container');
                    let html = '<div class="info-grid">';
                    html += '<div class="info-item"><div class="info-label">Config Version</div><div class="info-value">' + (data.version || 'N/A') + '</div></div>';
                    html += '<div class="info-item"><div class="info-label">Config Digest</div><div class="info-value" style="font-family: monospace; font-size: 12px;">' + (data.digest || 'N/A') + '</div></div>';
                    if (data.config_stats) {
                        html += '<div class="info-item"><div class="info-label">Songs</div><div class="info-value">' + (data.config_stats.songs || 0) + '</div></div>';
                        html += '<div class="info-item"><div class="info-label">Artists</div><div class="info-value">' + (data.config_stats.artists || 0) + '</div></div>';
                        html += '<div class="info-item"><div class="info-label">Playlists</div><div class="info-value">' + (data.config_stats.playlists || 0) + '</div></div>';
                        html += '<div class="info-item"><div class="info-label">Albums</div><div class="info-value">' + (data.config_stats.albums || 0) + '</div></div>';
                    }
                    html += '</div>';
                    if (data.has_pending) {
                        html += '<div class="message message-info" style="margin-top: 16px; background-color: #fff3cd; color: #856404; border: 1px solid #ffeaa7;">⚠️ Configuration update pending - will be applied after current download completes</div>';
                    }
                    container.innerHTML = html;
                })
                .catch(err => {
                    document.getElementById('config-container').innerHTML = 
                        '<div class="message message-error">Error loading config info: ' + err.message + '</div>';
                });
        }
        
        function updateStats(data) {
            const container = document.getElementById('stats-container');
            const stats = data.statistics || {};
            
            container.innerHTML = 
                '<div class="stats-grid">' +
                    '<div class="stat-item">' +
                        '<div class="stat-value">' + (stats.pending || 0) + '</div>' +
                        '<div class="stat-label">Pending</div>' +
                    '</div>' +
                    '<div class="stat-item">' +
                        '<div class="stat-value">' + (stats.completed || 0) + '</div>' +
                        '<div class="stat-label">Completed</div>' +
                    '</div>' +
                    '<div class="stat-item">' +
                        '<div class="stat-value">' + (stats.failed || 0) + '</div>' +
                        '<div class="stat-label">Failed</div>' +
                    '</div>' +
                    '<div class="stat-item">' +
                        '<div class="stat-value">' + (stats.total || 0) + '</div>' +
                        '<div class="stat-label">Total</div>' +
                    '</div>' +
                '</div>';
        }
        
        function showMessage(text, isError) {
            const container = document.getElementById('message-container');
            const className = isError ? 'message-error' : 'message-success';
            container.innerHTML = '<div class="message ' + className + '">' + text + '</div>';
            setTimeout(() => {
                container.innerHTML = '';
            }, 5000);
        }
        
        function startDownload() {
            fetch('/api/download/start', { method: 'POST' })
                .then(function(res) {
                    return res.json().then(function(data) {
                        if (res.ok) {
                            showMessage(data.message || 'Download started successfully', false);
                        } else {
                            showMessage(data.message || data.error || 'Failed to start download', true);
                        }
                        updateStatus();
                    });
                })
                .catch(function(err) {
                    showMessage('Error: ' + err.message, true);
                });
        }
        
        function stopDownload() {
            if (!confirm('Are you sure you want to stop the download?')) return;
            
            fetch('/api/download/stop', { method: 'POST' })
                .then(function(res) {
                    return res.json().then(function(data) {
                        if (res.ok) {
                            showMessage(data.message || 'Download stop requested', false);
                        } else {
                            showMessage(data.message || data.error || 'Failed to stop download', true);
                        }
                        updateStatus();
                    });
                })
                .catch(function(err) {
                    showMessage('Error: ' + err.message, true);
                });
        }
        
        function resetDownload() {
            if (!confirm('Are you sure you want to reset? This will stop the download, clear all state, and delete plan files.')) return;
            
            fetch('/api/download/reset', { method: 'POST' })
                .then(function(res) {
                    return res.json().then(function(data) {
                        if (res.ok) {
                            showMessage(data.message || 'Download state reset successfully', false);
                        } else {
                            showMessage(data.message || data.error || 'Failed to reset download', true);
                        }
                        updateStatus();
                        updateConfig();
                    });
                })
                .catch(function(err) {
                    showMessage('Error: ' + err.message, true);
                });
        }
        
        // Initial load
        updateStatus();
        updateConfig();
        
        // Auto-refresh every 2 seconds
        statusInterval = setInterval(function() {
            updateStatus();
            updateConfig();
        }, 2000);
        
        // Cleanup on page unload
        window.addEventListener('beforeunload', () => {
            if (statusInterval) clearInterval(statusInterval);
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
