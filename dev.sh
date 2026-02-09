#!/bin/bash

# musicdl development startup script
# Starts both Go API server and Node.js web server for development

set -e

echo "🎵 musicdl Development Environment"
echo "===================================="
echo ""

# Check if Go binary exists
if [ ! -f "./musicdl" ]; then
  echo "📦 Building Go binary..."
  go build -o musicdl ./control
fi

# Check Node.js dependencies
if [ ! -d "./webserver/backend/node_modules" ]; then
  echo "📦 Installing backend dependencies..."
  cd webserver/backend
  npm install
  cd ../..
fi

if [ ! -d "./webserver/frontend/node_modules" ]; then
  echo "📦 Installing frontend dependencies..."
  cd webserver/frontend
  npm install
  cd ../..
fi

echo ""
echo "Starting services..."
echo ""

# Start Go API server in background
echo "🚀 Starting Go API server on port 5000..."
MUSICDL_API_PORT=5000 ./musicdl api &
GO_PID=$!

# Wait for API server to start
sleep 2

# Start Express backend
echo "🚀 Starting Express backend on port 3000..."
cd webserver/backend
PORT=3000 GO_API_HOST=localhost GO_API_PORT=5000 npm run dev &
EXPRESS_PID=$!

# Start Vue frontend dev server
echo "🚀 Starting Vue frontend dev server on port 5173..."
cd ../frontend
npm run dev &
VITE_PID=$!

cd ../..

echo ""
echo "✅ All services started!"
echo ""
echo "📱 Frontend:  http://localhost:5173"
echo "💻 Backend:   http://localhost:3000"
echo "🔌 API:       http://localhost:5000"
echo ""
echo "Press Ctrl+C to stop all services..."
echo ""

# Handle shutdown
trap "kill $GO_PID $EXPRESS_PID $VITE_PID 2>/dev/null; exit 0" SIGINT SIGTERM

# Wait for all processes
wait
