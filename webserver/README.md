# musicdl Web Server

A modern web interface for the musicdl music download tool, built with Vue 3, TypeScript, and Express.

## Architecture

- **Backend**: Express.js + TypeScript (proxies to Go API server on port 5000)
- **Frontend**: Vue 3 + TypeScript + Vite + Tailwind CSS
- **Port**: 3000 (Express server) and 80 (production Docker)

## Features

- 🎵 **Download Management** - Plan generation and download execution
- ⚙️ **Configuration Editor** - Edit config.yaml directly from UI
- 📊 **Real-time Progress** - Live progress tracking for downloads
- 🚨 **Rate Limit Alerts** - Clear warnings with countdown timers
- 📝 **Log Viewer** - View recent logs from running operations
- 🌙 **Dark Theme** - Modern dark interface with Tailwind CSS

## Development Setup

### Prerequisites

- Node.js 18+
- npm or yarn
- Go CLI server running on localhost:5000

### Install Dependencies

```bash
cd webserver/backend
npm install

cd ../frontend
npm install
```

### Run Development Mode

Terminal 1 (Backend):
```bash
cd webserver/backend
npm run dev
# Runs on http://localhost:3000
```

Terminal 2 (Frontend - Hot reload):
```bash
cd webserver/frontend
npm run dev
```

Or run both concurrently:
```bash
cd webserver
npm install concurrently
npm run dev
```

### Build for Production

```bash
cd webserver/backend
npm run build

cd ../frontend
npm run build
```

## Environment Variables

### Backend (.env)

```env
PORT=3000
GO_API_PORT=5000
GO_API_HOST=localhost
```

### Frontend

No special environment variables needed for frontend (configured in vite.config.ts).

## API Endpoints

The webserver acts as a proxy to the Go API:

- `GET /api/health` - Health check
- `GET /api/config` - Get config.yaml
- `POST /api/config` - Save config.yaml
- `POST /api/download/plan` - Generate download plan
- `POST /api/download/run` - Execute download
- `POST /api/download/stop` - Pause or stop the current operation
- `GET /api/download/status` - Get current operation status
- `GET /api/rate-limit-status` - Get rate limit information
- `GET /api/logs` - Get recent logs

## Docker Deployment

See the main musicdl Dockerfile for full deployment instructions.

```bash
docker run -p 80:80 -v /path/to/workspace:/download ghcr.io/sv4u/musicdl:latest
# Access at http://localhost
```

## Project Structure

```
webserver/
├── backend/
│   ├── src/
│   │   └── index.ts          # Express server
│   ├── public/               # Built frontend (generated)
│   ├── package.json
│   └── tsconfig.json
├── frontend/
│   ├── src/
│   │   ├── components/       # Vue components
│   │   ├── App.vue
│   │   ├── main.ts
│   │   └── style.css
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── tsconfig.json
└── .gitignore
```

## Vue Components

- **App.vue** - Main application shell with tabs and status cards
- **DownloadRunner.vue** - Plan generation and download control
- **ConfigEditor.vue** - YAML configuration editor
- **LogViewer.vue** - Real-time log display
- **RateLimitAlert.vue** - Rate limit warning with countdown

## Development Tips

1. **Hot Reload**: Frontend supports hot module replacement (HMR)
2. **CORS**: Backend handles CORS for API requests
3. **Proxy**: Vite dev server proxies API calls to Express
4. **Types**: Full TypeScript support throughout

## Building a Release

1. Build backend and frontend
2. Copy frontend dist to backend/public
3. Build Docker image with both services
4. Container runs both on startup

## Contributing

When adding new features:
1. Update Go API endpoints first
2. Add Express proxy endpoints
3. Create Vue components
4. Update types as needed

## License

See LICENSE in parent directory.
