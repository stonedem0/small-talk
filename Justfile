# Run Go server with air
server:
  cd server && air

# Run React client with Vite
client:
  cd client && npm run dev

# Run both servers concurrently using shell
dev:
  sh -c 'just server & just client'

# Build React client
build:
  cd client && npm run build

# Run in production (Go server + built React app)
start:
  sh -c 'just server-prod & just client-prod'

# Run Go server in production
server-prod:
  cd server && go run main.go

# Preview built React app in production
client-prod:
  cd client && npm run preview
