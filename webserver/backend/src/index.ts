import express, { Express, Request, Response } from 'express';
import cors from 'cors';
import axios from 'axios';
import dotenv from 'dotenv';
import { createProxyMiddleware } from 'http-proxy-middleware';

dotenv.config();

const app: Express = express();
const port = process.env.PORT || 3000;
const goAPIPort = process.env.GO_API_PORT || 5000;
const goAPIHost = process.env.GO_API_HOST || 'localhost';
const goAPIBaseURL = `http://${goAPIHost}:${goAPIPort}`;

// Middleware
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.static('public'));

// WebSocket proxy for real-time logs.
// Uses pathFilter instead of app.use('/api/ws', ...) because Express mount
// paths strip the prefix from req.url before passing to middleware. With
// app.use('/api/ws', proxy), a request to /api/ws/logs would arrive at the
// proxy as /logs, which gets forwarded to ws://host:port/logs — but the Go
// server expects the full path /api/ws/logs. pathFilter matches the request
// internally without stripping, preserving the full URL.
//
// In http-proxy-middleware v3.x, ws:true alone does NOT handle WebSocket
// upgrade requests. We must explicitly bind the proxy's upgrade handler to
// the HTTP server's 'upgrade' event (done below after app.listen()).
const wsProxy = createProxyMiddleware({
  target: `ws://${goAPIHost}:${goAPIPort}`,
  pathFilter: '/api/ws',
  ws: true,
  changeOrigin: true,
});
app.use(wsProxy);

// Helper to proxy GET requests
function proxyGet(path: string, errorMsg: string) {
  app.get(path, async (req: Request, res: Response) => {
    try {
      const response = await axios.get(`${goAPIBaseURL}${path}`);
      res.json(response.data);
    } catch (error) {
      if (axios.isAxiosError(error) && error.response) {
        res.status(error.response.status).json(error.response.data);
      } else {
        res.status(500).json({ error: errorMsg });
      }
    }
  });
}

// Helper to proxy POST requests
function proxyPost(path: string, errorMsg: string) {
  app.post(path, async (req: Request, res: Response) => {
    try {
      const response = await axios.post(`${goAPIBaseURL}${path}`, req.body);
      res.status(response.status).json(response.data);
    } catch (error) {
      if (axios.isAxiosError(error) && error.response) {
        res.status(error.response.status).json(error.response.data);
      } else {
        res.status(500).json({ error: errorMsg });
      }
    }
  });
}

// System
proxyGet('/api/health', 'Go API server is not available');

// Config
proxyGet('/api/config', 'Failed to fetch config');
proxyPost('/api/config', 'Failed to save config');

// Download
proxyPost('/api/download/plan', 'Failed to start plan generation');
proxyPost('/api/download/run', 'Failed to start download');
proxyGet('/api/download/status', 'Failed to fetch status');
proxyGet('/api/rate-limit-status', 'Failed to fetch rate limit status');

// Logs
proxyGet('/api/logs', 'Failed to fetch logs');

// Statistics
proxyGet('/api/stats', 'Failed to fetch statistics');
proxyPost('/api/stats/reset', 'Failed to reset statistics');

// Recovery
proxyGet('/api/recovery/status', 'Failed to fetch recovery status');
proxyPost('/api/recovery/circuit-breaker/reset', 'Failed to reset circuit breaker');
proxyPost('/api/recovery/resume/clear', 'Failed to clear resume state');
proxyPost('/api/recovery/resume/retry-failed', 'Failed to retry failed items');

// Swagger proxy — these endpoints return HTML and JSON respectively, so we
// must NOT use res.json() (which would double-serialize the HTML string into
// "\"<!DOCTYPE html>...\"" with Content-Type: application/json). Instead,
// pipe the upstream response through with its original Content-Type intact.
// Uses pathFilter instead of app.use('/api/docs', ...) for the same reason
// as the WebSocket proxy: Express mount paths strip the prefix from req.url,
// so /api/docs/swagger.json would be forwarded as /swagger.json, which the
// Go server doesn't recognise.
app.use(
  createProxyMiddleware({
    target: goAPIBaseURL,
    pathFilter: '/api/docs',
    changeOrigin: true,
  })
);

// Serve Vue app (catch-all for SPA routing)
app.get('*', (req: Request, res: Response) => {
  res.sendFile('index.html', { root: 'public' });
});

const server = app.listen(port, () => {
  console.log(`Express server running on http://localhost:${port}`);
  console.log(`Go API server: ${goAPIBaseURL}`);
  console.log(`WebSocket proxy: ws://localhost:${port}/api/ws/logs -> ws://${goAPIHost}:${goAPIPort}/api/ws/logs`);
});

// http-proxy-middleware v3.x requires explicit WebSocket upgrade binding.
// Without this, the proxy only handles regular HTTP requests on /api/ws;
// WebSocket upgrade requests would be silently ignored, breaking real-time
// log streaming when accessed through the Express backend (port 3000/80).
// Development with Vite masks this because Vite's own proxy handles
// WebSocket upgrades directly to the Go server.
server.on('upgrade', wsProxy.upgrade);
