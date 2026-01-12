package handlers

import (
	"encoding/json"
	"net/http"
)

// Status handles GET /api/status - Get download service status and plan progress.
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	// Get service (may not be initialized yet)
	service, err := h.getService()
	if err != nil || service == nil {
		// Service not initialized yet, return idle state
		response := map[string]interface{}{
			"state":      "idle",
			"phase":      nil,
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Service not initialized",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get status from service
	status := service.GetStatus()

	// Get plan if available
	var planData interface{}
	if plan := service.GetPlan(); plan != nil {
		stats := plan.GetStatistics()
		planData = map[string]interface{}{
			"statistics": stats,
			"item_count": len(plan.Items),
		}
	}

	// Safely extract values with defaults
	state, ok := status["state"].(string)
	if !ok || state == "" {
		state = "idle"
	}

	phase := status["phase"]
	if phase == nil {
		phase = "idle"
	}

	planFile := status["plan_file"]
	if planFile == nil {
		planFile = nil
	}

	// Safely extract plan_stats, default to empty map if not present
	statistics := status["plan_stats"]
	if statistics == nil {
		statistics = map[string]interface{}{}
	}

	response := map[string]interface{}{
		"state":      state,
		"phase":      phase,
		"statistics": statistics,
		"plan_file":  planFile,
		"plan":       planData,
	}

	if errorMsg, ok := status["error"].(string); ok && errorMsg != "" {
		response["error"] = errorMsg
	}

	if startedAt, ok := status["started_at"].(string); ok {
		response["started_at"] = startedAt
	}

	if completedAt, ok := status["completed_at"].(string); ok {
		response["completed_at"] = completedAt
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
            <h2>Plan Details</h2>
            <div id="plan-container">
                <div class="loading">Loading plan details...</div>
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
            const container = document.getElementById('plan-container');
            if (data.plan && data.plan.item_count) {
                container.innerHTML = '<p>Plan contains ' + data.plan.item_count + ' items.</p>';
            } else {
                container.innerHTML = '<p>No plan available.</p>';
            }
        }
        
        updateStatus();
        statusInterval = setInterval(updateStatus, 2000);
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
