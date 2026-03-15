package main

import "fmt"

// swaggerUIHTML returns the Swagger UI HTML page.
func swaggerUIHTML(_ int) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>musicdl API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" integrity="sha384-+yyzNgM3K92sROwsXxYCxaiLWxWJ0G+v/9A+qIZ2rgefKgkdcmJI+L601cqPD/Ut" crossorigin="anonymous">
    <style>
        body { margin: 0; background: #1a1a2e; }
        .swagger-ui .topbar { display: none; }
        .swagger-ui { max-width: 1200px; margin: 0 auto; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" integrity="sha384-qn5tagrAjZi8cSmvZ+k3zk4+eDEEUcP9myuR2J6V+/H6rne++v6ChO7EeHAEzqxQ" crossorigin="anonymous"></script>
    <script>
        SwaggerUIBundle({
            url: '/api/docs/swagger.json',
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout",
            tryItOutEnabled: true,
        });
    </script>
</body>
</html>`
}

// generateOpenAPISpec returns the OpenAPI 3.0 specification as JSON.
func generateOpenAPISpec(port int) string {
	return fmt.Sprintf(`{
  "openapi": "3.0.3",
  "info": {
    "title": "musicdl API",
    "description": "HTTP API for the musicdl music download tool. Provides endpoints for download management, configuration, real-time logs, statistics, and error recovery.",
    "version": "1.0.0",
    "contact": {
      "name": "musicdl",
      "url": "https://github.com/sv4u/musicdl"
    }
  },
  "servers": [
    {
      "url": "http://localhost:%d",
      "description": "Local API server"
    }
  ],
  "tags": [
    {"name": "system", "description": "Health and system status"},
    {"name": "config", "description": "Configuration management"},
    {"name": "download", "description": "Plan generation and download execution"},
    {"name": "logs", "description": "Log retrieval and real-time streaming"},
    {"name": "stats", "description": "Download statistics and metrics"},
    {"name": "recovery", "description": "Error recovery, circuit breaker, and resume"},
    {"name": "plan", "description": "Real-time plan state and WebSocket streaming"},
    {"name": "history", "description": "Run history and activity log"}
  ],
  "paths": {
    "/api/health": {
      "get": {
        "tags": ["system"],
        "summary": "Health check",
        "description": "Check API server health, WebSocket connections, and circuit breaker state",
        "responses": {
          "200": {
            "description": "Server is healthy",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {"type": "string", "example": "healthy"},
                    "time": {"type": "integer", "description": "Unix timestamp"},
                    "wsClients": {"type": "integer", "description": "Connected WebSocket clients"},
                    "circuitBreakerState": {"type": "string", "enum": ["closed", "open", "half_open"]}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/version": {
      "get": {
        "tags": ["system"],
        "summary": "Get version info",
        "description": "Returns musicdl and spotigo (Spotify API client) versions",
        "responses": {
          "200": {
            "description": "Version information",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "musicdl": {"type": "string", "description": "musicdl version"},
                    "spotigo": {"type": "string", "description": "spotigo (Spotify API client) version"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/config": {
      "get": {
        "tags": ["config"],
        "summary": "Get configuration",
        "description": "Retrieve the current config.yaml content",
        "responses": {
          "200": {
            "description": "Config content",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "config": {"type": "string", "description": "YAML config content"}
                  }
                }
              }
            }
          },
          "404": {
            "description": "Config file not found"
          }
        }
      },
      "post": {
        "tags": ["config"],
        "summary": "Save configuration",
        "description": "Update the config.yaml content",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["config"],
                "properties": {
                  "config": {"type": "string", "description": "YAML config content"}
                }
              }
            }
          }
        },
        "responses": {
          "200": {"description": "Config saved successfully"},
          "400": {"description": "Invalid request"}
        }
      }
    },
    "/api/download/plan": {
      "post": {
        "tags": ["download"],
        "summary": "Generate download plan",
        "description": "Generate a download plan from config. Checks circuit breaker before starting.",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "configPath": {"type": "string", "default": "/download/config.yaml"}
                }
              }
            }
          }
        },
        "responses": {
          "202": {"description": "Plan generation started"},
          "503": {"description": "Circuit breaker is open"}
        }
      }
    },
    "/api/download/run": {
      "post": {
        "tags": ["download"],
        "summary": "Run download",
        "description": "Execute download from existing plan. Supports resume from interrupted downloads.",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "configPath": {"type": "string", "default": "/download/config.yaml"},
                  "resume": {"type": "string", "enum": ["true", "false"], "description": "Resume from last interrupted run"}
                }
              }
            }
          }
        },
        "responses": {
          "202": {"description": "Download started"},
          "503": {"description": "Circuit breaker is open"}
        }
      }
    },
    "/api/download/status": {
      "get": {
        "tags": ["download"],
        "summary": "Get download status",
        "description": "Get current progress of plan generation or download, including classified error details",
        "responses": {
          "200": {
            "description": "Current status",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "isRunning": {"type": "boolean"},
                    "operationType": {"type": "string"},
                    "startedAt": {"type": "integer"},
                    "progress": {"type": "integer"},
                    "total": {"type": "integer"},
                    "logs": {"type": "array", "items": {"type": "string"}},
                    "error": {
                      "type": "object",
                      "nullable": true,
                      "properties": {
                        "code": {"type": "string"},
                        "message": {"type": "string"},
                        "explanation": {"type": "string"},
                        "suggestion": {"type": "string"},
                        "retryable": {"type": "boolean"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/rate-limit-status": {
      "get": {
        "tags": ["download"],
        "summary": "Get rate limit status",
        "description": "Get current Spotify rate limit status with countdown information",
        "responses": {
          "200": {
            "description": "Rate limit status",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "active": {"type": "boolean"},
                    "retryAfterSeconds": {"type": "integer"},
                    "retryAfterTimestamp": {"type": "integer"},
                    "detectedAt": {"type": "integer"},
                    "remainingSeconds": {"type": "integer"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/logs": {
      "get": {
        "tags": ["logs"],
        "summary": "Get log history",
        "description": "Retrieve recent log history via HTTP. For real-time streaming, use the WebSocket endpoint at /api/ws/logs",
        "responses": {
          "200": {
            "description": "Log history",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "logs": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "timestamp": {"type": "integer"},
                          "level": {"type": "string"},
                          "message": {"type": "string"},
                          "source": {"type": "string"}
                        }
                      }
                    },
                    "wsUrl": {"type": "string", "description": "WebSocket URL for real-time streaming"},
                    "wsHint": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/ws/logs": {
      "get": {
        "tags": ["logs"],
        "summary": "WebSocket log stream",
        "description": "Real-time log streaming via WebSocket. On connect, receives buffered history (up to 1000 messages), then live updates. Supports auto-reconnect. Messages are JSON: {timestamp, level, message, source}."
      }
    },
    "/api/stats": {
      "get": {
        "tags": ["stats"],
        "summary": "Get statistics",
        "description": "Get per-run and cumulative download statistics including success rates, timing, and resource usage",
        "responses": {
          "200": {
            "description": "Statistics",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "cumulative": {
                      "type": "object",
                      "properties": {
                        "totalDownloaded": {"type": "integer"},
                        "totalFailed": {"type": "integer"},
                        "totalSkipped": {"type": "integer"},
                        "totalPlansGenerated": {"type": "integer"},
                        "totalRuns": {"type": "integer"},
                        "totalRateLimits": {"type": "integer"},
                        "totalRetries": {"type": "integer"},
                        "totalBytesWritten": {"type": "integer"},
                        "totalTimeSpentSec": {"type": "number"},
                        "planTimeSpentSec": {"type": "number"},
                        "downloadTimeSpentSec": {"type": "number"},
                        "firstRunAt": {"type": "integer"},
                        "lastRunAt": {"type": "integer"},
                        "successRate": {"type": "number"}
                      }
                    },
                    "currentRun": {
                      "type": "object",
                      "properties": {
                        "runId": {"type": "string"},
                        "operationType": {"type": "string"},
                        "startedAt": {"type": "integer"},
                        "downloaded": {"type": "integer"},
                        "failed": {"type": "integer"},
                        "skipped": {"type": "integer"},
                        "retries": {"type": "integer"},
                        "rateLimits": {"type": "integer"},
                        "bytesWritten": {"type": "integer"},
                        "elapsedSec": {"type": "number"},
                        "tracksPerHour": {"type": "number"},
                        "isRunning": {"type": "boolean"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/stats/reset": {
      "post": {
        "tags": ["stats"],
        "summary": "Reset statistics",
        "description": "Reset all cumulative statistics to zero. Current run stats are not affected.",
        "responses": {
          "200": {"description": "Statistics reset successfully"}
        }
      }
    },
    "/api/recovery/status": {
      "get": {
        "tags": ["recovery"],
        "summary": "Get recovery status",
        "description": "Get combined circuit breaker and resume state for error recovery monitoring",
        "responses": {
          "200": {
            "description": "Recovery status",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "circuitBreaker": {
                      "type": "object",
                      "properties": {
                        "state": {"type": "string", "enum": ["closed", "open", "half_open"]},
                        "failureCount": {"type": "integer"},
                        "successCount": {"type": "integer"},
                        "failureThreshold": {"type": "integer"},
                        "successThreshold": {"type": "integer"},
                        "resetTimeoutSec": {"type": "integer"},
                        "lastFailureAt": {"type": "integer"},
                        "lastStateChange": {"type": "integer"},
                        "canRetry": {"type": "boolean"}
                      }
                    },
                    "resume": {
                      "type": "object",
                      "properties": {
                        "hasResumeData": {"type": "boolean"},
                        "completedCount": {"type": "integer"},
                        "failedCount": {"type": "integer"},
                        "totalItems": {"type": "integer"},
                        "remainingCount": {"type": "integer"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/recovery/circuit-breaker/reset": {
      "post": {
        "tags": ["recovery"],
        "summary": "Reset circuit breaker",
        "description": "Manually reset the circuit breaker to closed state, allowing requests through again",
        "responses": {
          "200": {"description": "Circuit breaker reset"}
        }
      }
    },
    "/api/recovery/resume/clear": {
      "post": {
        "tags": ["recovery"],
        "summary": "Clear resume state",
        "description": "Clear all resume/checkpoint data. Use before starting a completely fresh download run.",
        "responses": {
          "200": {"description": "Resume state cleared"}
        }
      }
    },
    "/api/recovery/resume/retry-failed": {
      "post": {
        "tags": ["recovery"],
        "summary": "Retry failed items",
        "description": "Get the list of retryable failed items from the last interrupted run",
        "responses": {
          "200": {
            "description": "Retryable items",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": {"type": "string"},
                    "retryableItems": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "url": {"type": "string"},
                          "name": {"type": "string"},
                          "error": {"type": "string"},
                          "attempts": {"type": "integer"},
                          "lastAttempt": {"type": "integer"},
                          "retryable": {"type": "boolean"}
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/download/stop": {
      "post": {
        "tags": ["download"],
        "summary": "Stop operation",
        "description": "Cancel the currently running plan generation or download. Progress is saved for downloads; use Resume to continue.",
        "responses": {
          "200": {
            "description": "Operation cancelled",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": {"type": "string", "example": "cancelled"},
                    "message": {"type": "string"}
                  }
                }
              }
            }
          },
          "409": {
            "description": "No operation is running",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "error": {"type": "string", "example": "No operation is currently running"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/plan": {
      "get": {
        "tags": ["plan"],
        "summary": "Get plan snapshot",
        "description": "Returns the current plan state including all items, their statuses, and aggregate stats",
        "responses": {
          "200": {
            "description": "Current plan snapshot",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "phase": {"type": "string"},
                    "items": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "item_id": {"type": "string"},
                          "item_type": {"type": "string"},
                          "name": {"type": "string"},
                          "status": {"type": "string"},
                          "error": {"type": "string"},
                          "file_path": {"type": "string"},
                          "parent_id": {"type": "string"},
                          "child_ids": {"type": "array", "items": {"type": "string"}},
                          "spotify_url": {"type": "string"},
                          "youtube_url": {"type": "string"},
                          "source": {"type": "string"},
                          "source_url": {"type": "string"},
                          "progress": {"type": "number"},
                          "started_at": {"type": "integer"},
                          "completed_at": {"type": "integer"}
                        }
                      }
                    },
                    "generated_at": {"type": "string"},
                    "config_hash": {"type": "string"},
                    "stats": {
                      "type": "object",
                      "properties": {
                        "total": {"type": "integer"},
                        "completed": {"type": "integer"},
                        "failed": {"type": "integer"},
                        "pending": {"type": "integer"},
                        "in_progress": {"type": "integer"},
                        "skipped": {"type": "integer"}
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "/api/ws/plan": {
      "get": {
        "tags": ["plan"],
        "summary": "WebSocket plan stream",
        "description": "Real-time plan updates via WebSocket. On connect, receives the full plan snapshot, then incremental item-level updates. Messages are JSON with type field: snapshot, item_update, phase_change."
      }
    },
    "/api/history/runs": {
      "get": {
        "tags": ["history"],
        "summary": "List run history",
        "description": "Returns a summary list of past download/plan runs, sorted newest-first",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "schema": {"type": "integer", "default": 20},
            "description": "Max runs to return"
          }
        ],
        "responses": {
          "200": {
            "description": "Run history list",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "runs": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "runId": {"type": "string"},
                          "startedAt": {"type": "string", "format": "date-time"},
                          "completedAt": {"type": "string", "format": "date-time"},
                          "state": {"type": "string"},
                          "phase": {"type": "string"},
                          "statistics": {"type": "object"},
                          "error": {"type": "string"},
                          "snapshotCount": {"type": "integer"}
                        }
                      }
                    },
                    "totalRuns": {"type": "integer"}
                  }
                }
              }
            }
          },
          "503": {"description": "History tracking not available"}
        }
      }
    },
    "/api/history/runs/{runID}": {
      "get": {
        "tags": ["history"],
        "summary": "Get run detail",
        "description": "Returns the full run history including all progress snapshots",
        "parameters": [
          {
            "name": "runID",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "Run ID"
          }
        ],
        "responses": {
          "200": {
            "description": "Full run history with snapshots",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "runId": {"type": "string"},
                    "startedAt": {"type": "string", "format": "date-time"},
                    "completedAt": {"type": "string", "format": "date-time"},
                    "state": {"type": "string"},
                    "phase": {"type": "string"},
                    "statistics": {"type": "object"},
                    "error": {"type": "string"},
                    "snapshots": {"type": "array", "items": {"type": "object"}}
                  }
                }
              }
            }
          },
          "404": {"description": "Run not found"},
          "503": {"description": "History tracking not available"}
        }
      }
    },
    "/api/history/activity": {
      "get": {
        "tags": ["history"],
        "summary": "Get activity log",
        "description": "Returns recent activity entries (download started, completed, failed, etc.)",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "schema": {"type": "integer", "default": 50},
            "description": "Max entries to return"
          }
        ],
        "responses": {
          "200": {
            "description": "Activity log",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "entries": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "timestamp": {"type": "string", "format": "date-time"},
                          "type": {"type": "string"},
                          "message": {"type": "string"},
                          "details": {"type": "object"}
                        }
                      }
                    }
                  }
                }
              }
            }
          },
          "503": {"description": "History tracking not available"}
        }
      }
    }
  }
}`, port)
}
