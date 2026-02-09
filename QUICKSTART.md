# Quick Start Guide - musicdl Web Interface

## For Users (Docker)

### Access Web Interface

```bash
docker run -p 80:3000 \
  -v /path/to/workspace:/download \
  -v /path/to/config.yaml:/download/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest
```

Then open `http://localhost` in your browser.

### Use CLI (Traditional)

```bash
docker run --rm \
  -v /path/to/workspace:/download \
  -v /path/to/config.yaml:/download/config.yaml:ro \
  ghcr.io/sv4u/musicdl:latest musicdl plan config.yaml
```

## For Developers (Local Setup)

### Prerequisites
- Go 1.24+
- Node.js 18+
- npm or yarn

### Start Development Environment

```bash
# Clone and navigate
git clone git@github.com:sv4u/musicdl.git
cd musicdl

# Copy the example env file (optional)
cp webserver/backend/.env.example webserver/backend/.env

# One command to start everything
./dev.sh
```

This starts three services:
- **Go API**: `http://localhost:5000`
- **Express Backend**: `http://localhost:3000`
- **Vue Frontend**: `http://localhost:5173`

### Manual Start (if dev.sh doesn't work)

**Terminal 1 - Go API:**
```bash
go build -o musicdl ./control
./musicdl api
```

**Terminal 2 - Express Backend:**
```bash
cd webserver/backend
npm install
npm run dev
```

**Terminal 3 - Vue Frontend:**
```bash
cd webserver/frontend
npm install
npm run dev
```

## Web Interface Features

### Download Tab
- **Generate Plan**: Creates a download plan from your config
- **Run Download**: Executes the download plan
- **Progress Bar**: Shows real-time progress
- **Status**: Current operation status

### Configuration Tab
- **Load**: Fetch current config from server
- **Edit**: Modify config in the text editor
- **Save**: Save changes back to server
- Works only if `/download/config.yaml` exists

### Logs Tab
- **Real-time Logs**: Auto-updates every 2 seconds
- **Refresh**: Manually refresh logs
- **Clear**: Clear log display

### Rate Limit Alert
- **Red Alert Box**: Shows when Spotify rate limit is active
- **Countdown Timer**: Displays remaining wait time
- **Auto-updates**: Updates every second

## API Endpoints (for Advanced Users)

### Health Check
```bash
curl http://localhost:5000/api/health
```

### Get Config
```bash
curl http://localhost:5000/api/config
```

### Save Config
```bash
curl -X POST http://localhost:5000/api/config \
  -H "Content-Type: application/json" \
  -d '{"config": "version: \"1.2\"\n..."}'
```

### Generate Plan
```bash
curl -X POST http://localhost:5000/api/download/plan \
  -H "Content-Type: application/json" \
  -d '{"configPath": "/download/config.yaml"}'
```

### Run Download
```bash
curl -X POST http://localhost:5000/api/download/run \
  -H "Content-Type: application/json" \
  -d '{"configPath": "/download/config.yaml"}'
```

### Get Status
```bash
curl http://localhost:5000/api/download/status
```

### Get Rate Limit Status
```bash
curl http://localhost:5000/api/rate-limit-status
```

### Get Logs
```bash
curl http://localhost:5000/api/logs
```

## Building for Production

### Docker Image
```bash
docker build -f musicdl.Dockerfile -t my-registry/musicdl:latest .
docker push my-registry/musicdl:latest
```

### Standalone Binary
```bash
# Build Go binary
go build -o musicdl ./control

# Build frontend
cd webserver/frontend
npm install
npm run build

# Build backend
cd ../backend
npm install
npm run build
```

## Environment Variables

### For Docker
```bash
docker run -e MUSICDL_API_PORT=8000 \
           -e MUSICDL_CACHE_DIR=/download/.cache \
           -p 80:3000 \
           ghcr.io/sv4u/musicdl:latest
```

### For Local Development
```bash
# In webserver/backend/.env
PORT=3000
GO_API_HOST=localhost
GO_API_PORT=5000
NODE_ENV=development
```

## Troubleshooting

### Port Already in Use
```bash
# Change port for frontend
cd webserver/frontend
npm run dev -- --port 5174

# Change port for backend
PORT=3001 npm run dev  # in backend/

# Change port for API
./musicdl api --port 8080
```

### Go API Won't Start
```bash
# Check if binary exists
go build -o musicdl ./control

# Try with explicit port
./musicdl api --port 5000

# Check if port is free
lsof -i :5000
```

### Frontend Won't Load
```bash
# Ensure dependencies are installed
cd webserver/frontend
npm install

# Clear cache and rebuild
rm -rf node_modules dist
npm install
npm run dev
```

### Config Not Updating
- Ensure `/download/config.yaml` exists
- Check file permissions (should be readable)
- Verify Docker volume mount: `-v /path/to/config.yaml:/download/config.yaml:ro`

### Rate Limit Not Showing
- The alert only appears when Spotify API returns rate limit error
- Check logs in Logs tab for rate limit messages
- Manually test by running many plan/download operations

## Common Workflows

### Download from Web Interface
1. Open `http://localhost` (Docker) or `http://localhost:3000` (Dev)
2. Go to Download tab
3. Click "Generate Plan"
4. Wait for completion
5. Click "Run Download"
6. Monitor progress and logs

### Edit Config via Web
1. Go to Configuration tab
2. Click "Load"
3. Edit YAML
4. Click "Save"
5. Generate new plan and download

### Use CLI While Web Runs
```bash
# In another terminal, while web interface is running:
./musicdl plan config.yaml
./musicdl download config.yaml
```

Both CLI and web interface share the same `.cache/` directory!

## Next Steps

- Read [README.md](README.md) for detailed documentation
- Check [IMPLEMENTATION.md](IMPLEMENTATION.md) for technical details
- See [webserver/README.md](webserver/README.md) for web server specifics
- Visit [GitHub Wiki](https://github.com/sv4u/musicdl/wiki) for architecture details

## Support

Issues? Check the logs:
- **Docker logs**: `docker logs <container-id>`
- **Web logs**: Check browser console (F12)
- **API logs**: Check stdout where `musicdl api` is running
- **Backend logs**: Check stdout where `npm run dev` is running
