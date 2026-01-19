package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Status handles GET /api/status - Get download service status and plan progress.
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if !svcManager.IsRunning() {
		// Service not running, return idle state
		response := map[string]interface{}{
			"state":      "idle",
			"phase":      "idle",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Service not running",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		response := map[string]interface{}{
			"state":      "error",
			"phase":      "error",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Failed to connect to download service",
			"error":      err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get status via gRPC
	statusResp, err := client.GetStatus(ctx)
	if err != nil {
		h.logError("Status", err)
		response := map[string]interface{}{
			"state":      "error",
			"phase":      "error",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Failed to get status",
			"error":      err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get plan items for plan data
	var planData interface{}
	planItemsResp, err := client.GetPlanItems(ctx, nil)
	if err == nil && planItemsResp != nil {
		planData = map[string]interface{}{
			"item_count": len(planItemsResp.Items),
		}
	}

	// Convert proto status to response format
	statistics := map[string]interface{}{
		"total":        statusResp.TotalItems,
		"completed":    statusResp.CompletedItems,
		"failed":       statusResp.FailedItems,
		"pending":      statusResp.PendingItems,
		"in_progress":  statusResp.InProgressItems,
	}

	response := map[string]interface{}{
		"state":      statusResp.State.String(),
		"phase":      statusResp.Phase.String(),
		"statistics": statistics,
		"plan_file":  nil, // Not available in proto response
		"plan":       planData,
	}

	if statusResp.ErrorMessage != "" {
		response["error"] = statusResp.ErrorMessage
	}

	if statusResp.StartedAt > 0 {
		response["started_at"] = time.Unix(statusResp.StartedAt, 0).Format(time.RFC3339)
	}

	if statusResp.CompletedAt != nil {
		response["completed_at"] = time.Unix(*statusResp.CompletedAt, 0).Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// StatusPage handles GET /status - HTML status dashboard.
func (h *Handlers) StatusPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>musicdl - Status</title>
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
        .info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
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
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
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
        .btn {
            display: inline-block;
            padding: 6px 12px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            font-weight: 500;
            transition: all 0.2s;
            background-color: #007bff;
            color: white;
        }
        .btn:hover { background-color: #0056b3; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 16px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #e0e0e0;
        }
        th {
            background-color: #f8f9fa;
            font-weight: 600;
            color: #333;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>musicdl Control Platform</h1>
            <div class="nav">
                <a href="/">Dashboard</a>
                <a href="/status" class="active">Status</a>
                <a href="/config">Config</a>
                <a href="/logs">Logs</a>
            </div>
        </div>
    </div>
    <div class="container">
        <div class="card">
            <h2>Service Status</h2>
            <div id="status-container">
                <div class="loading">Loading status...</div>
            </div>
        </div>
        <div class="card">
            <h2>Plan Statistics</h2>
            <div id="stats-container">
                <div class="loading">Loading statistics...</div>
            </div>
        </div>
        <div class="card">
            <h2>Plan Items</h2>
            <div id="plan-container">
                <div class="loading">Loading plan items...</div>
            </div>
        </div>
        <div class="card">
            <h2>Configuration</h2>
            <div id="config-container">
                <div class="loading">Loading configuration info...</div>
            </div>
        </div>
    </div>
    <script>
        let statusInterval;
        
        function updateStatus() {
            fetch('/api/status')
                .then(function(res) {
                    return res.json();
                })
                .then(function(data) {
                    const container = document.getElementById('status-container');
                    const state = data.state || 'idle';
                    const stateClass = 'status-' + state;
                    
                    let html = '<span class="status-badge ' + stateClass + '">' + state.toUpperCase() + '</span>';
                    if (data.error) {
                        html += '<span style="color: #d32f2f; margin-left: 10px;">' + data.error + '</span>';
                    }
                    
                    html += '<div class="info-grid" style="margin-top: 16px;">';
                    if (data.started_at) {
                        html += '<div class="info-item"><div class="info-label">Started At</div><div class="info-value">' + new Date(data.started_at).toLocaleString() + '</div></div>';
                    }
                    if (data.completed_at) {
                        html += '<div class="info-item"><div class="info-label">Completed At</div><div class="info-value">' + new Date(data.completed_at).toLocaleString() + '</div></div>';
                    }
                    html += '</div>';
                    
                    container.innerHTML = html;
                    
                    updateStats(data);
                    updatePlan(data);
                })
                .catch(function(err) {
                    document.getElementById('status-container').innerHTML = 
                        '<div style="color: #d32f2f;">Error loading status: ' + err.message + '</div>';
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
        
        function updatePlan(data) {
            fetch('/api/plan/items?type=track')
                .then(function(res) {
                    return res.json();
                })
                .then(function(planData) {
                    const container = document.getElementById('plan-container');
                    if (planData.items && planData.items.length > 0) {
                        let html = '<table><thead><tr><th>Name</th><th>Status</th><th>Progress</th><th>Actions</th></tr></thead><tbody>';
                        planData.items.forEach(function(item) {
                            const statusClass = 'status-' + (item.status || 'pending').toLowerCase().replace('_', '-');
                            const progress = (item.progress || 0).toFixed(1);
                            html += '<tr>';
                            html += '<td>' + (item.name || item.item_id) + '</td>';
                            html += '<td><span class="status-badge ' + statusClass + '">' + (item.status || 'PENDING') + '</span></td>';
                            html += '<td>' + progress + '%</td>';
                            html += '<td>';
                            if (item.status === 'PLAN_ITEM_STATUS_FAILED' || item.status === 'PLAN_ITEM_STATUS_COMPLETED') {
                                html += '<button class="btn btn-primary" style="padding: 4px 8px; font-size: 12px;" onclick="resetDownload()">Reset</button>';
                            }
                            html += '</td>';
                            html += '</tr>';
                        });
                        html += '</tbody></table>';
                        container.innerHTML = html;
                    } else {
                        container.innerHTML = '<p>No plan items available.</p>';
                    }
                })
                .catch(function(err) {
                    document.getElementById('plan-container').innerHTML = 
                        '<div style="color: #d32f2f;">Error loading plan items: ' + err.message + '</div>';
                });
        }
        
        function updateConfig() {
            fetch('/api/config/digest')
                .then(function(res) {
                    return res.json();
                })
                .then(function(data) {
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
                        html += '<div class="message message-info" style="margin-top: 16px; background-color: #fff3cd; color: #856404; border: 1px solid #ffeaa7;">⚠️ Configuration update pending</div>';
                    }
                    container.innerHTML = html;
                })
                .catch(function(err) {
                    document.getElementById('config-container').innerHTML = 
                        '<div style="color: #d32f2f;">Error loading config info: ' + err.message + '</div>';
                });
        }
        
        function resetDownload() {
            if (!confirm('Are you sure you want to reset? This will stop the download, clear all state, and delete plan files.')) return;
            
            fetch('/api/download/reset', { method: 'POST' })
                .then(function(res) {
                    return res.json().then(function(data) {
                        if (res.ok) {
                            alert(data.message || 'Download state reset successfully');
                        } else {
                            alert(data.message || data.error || 'Failed to reset download');
                        }
                        updateStatus();
                        updatePlan();
                        updateConfig();
                    });
                })
                .catch(function(err) {
                    alert('Error: ' + err.message);
                });
        }
        
        updateStatus();
        statusInterval = setInterval(function() {
            updateStatus();
            updatePlan();
            updateConfig();
        }, 2000);
        window.addEventListener('beforeunload', function() {
            if (statusInterval) clearInterval(statusInterval);
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
