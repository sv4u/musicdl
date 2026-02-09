import express, { Express, Request, Response } from 'express';
import cors from 'cors';
import axios from 'axios';
import dotenv from 'dotenv';

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

// API proxy endpoints
app.get('/api/health', async (req: Request, res: Response) => {
  try {
    const response = await axios.get(`${goAPIBaseURL}/api/health`);
    res.json(response.data);
  } catch (error) {
    res.status(503).json({ error: 'Go API server is not available' });
  }
});

app.get('/api/config', async (req: Request, res: Response) => {
  try {
    const response = await axios.get(`${goAPIBaseURL}/api/config`);
    res.json(response.data);
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 404) {
      res.status(404).json({ error: 'config.yaml not found' });
    } else {
      res.status(500).json({ error: 'Failed to fetch config' });
    }
  }
});

app.post('/api/config', async (req: Request, res: Response) => {
  try {
    const response = await axios.post(`${goAPIBaseURL}/api/config`, req.body);
    res.json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to save config' });
  }
});

app.post('/api/download/plan', async (req: Request, res: Response) => {
  try {
    const response = await axios.post(`${goAPIBaseURL}/api/download/plan`, req.body);
    res.status(response.status).json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to start plan generation' });
  }
});

app.post('/api/download/run', async (req: Request, res: Response) => {
  try {
    const response = await axios.post(`${goAPIBaseURL}/api/download/run`, req.body);
    res.status(response.status).json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to start download' });
  }
});

app.get('/api/download/status', async (req: Request, res: Response) => {
  try {
    const response = await axios.get(`${goAPIBaseURL}/api/download/status`);
    res.json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to fetch status' });
  }
});

app.get('/api/rate-limit-status', async (req: Request, res: Response) => {
  try {
    const response = await axios.get(`${goAPIBaseURL}/api/rate-limit-status`);
    res.json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to fetch rate limit status' });
  }
});

app.get('/api/logs', async (req: Request, res: Response) => {
  try {
    const response = await axios.get(`${goAPIBaseURL}/api/logs`);
    res.json(response.data);
  } catch (error) {
    res.status(500).json({ error: 'Failed to fetch logs' });
  }
});

// Serve Vue app
app.get('/', (req: Request, res: Response) => {
  res.sendFile('index.html', { root: 'public' });
});

app.listen(port, () => {
  console.log(`🚀 Express server running on http://localhost:${port}`);
  console.log(`🔗 Go API server: ${goAPIBaseURL}`);
});
