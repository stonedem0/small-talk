# Run Go server with air
server port="8080":
  cd apps/app/server && PORT=$(printf "%s" {{port}} | sed 's/^port=//') air

# Build Linux binary of the Go server (for packaging/deploy)
server-build-linux:
  cd apps/app/server && GOOS=linux GOARCH=amd64 go build -o app

# Run React client with Vite
client:
  cd apps/app/client && npm run dev

# Run both servers concurrently using shell (and ensure local Redis is up & ready)
# Uses REDIS_PASSWORD from env; falls back to 'changeme' for dev-only convenience.
dev port="8080":
  just server {{port}} & just client

# Build React client
build:
  cd apps/app/client && npm run build

# Run in production (Go server + built React app)
start:
  sh -c 'just server-prod & just client-prod'

# Run Go server in production
server-prod port="8080":
  cd apps/app/server && PORT=$(printf "%s" {{port}} | sed 's/^port=//') go run main.go

# Preview built React app in production
client-prod:
  cd apps/app/client && npm run preview

# Run Directory service (set DIRECTORY_PORT to override, default 8081)
directory port="8081":
  cd apps/directory && DIRECTORY_PORT=$(printf "%s" {{port}} | sed 's/^port=//') go run .

# --- Local Redis helpers (Docker) ---
# Start (or restart) Redis on localhost:6379 with a password.
# Password precedence: explicit param > $REDIS_PASSWORD > 'changeme'
redis-up password="":
  sh -c 'PASS="{{password}}"; PASS="${PASS:-${REDIS_PASSWORD:-changeme}}"; docker start redis >/dev/null 2>&1 || docker run --name redis -p 6379:6379 -d redis:7-alpine redis-server --appendonly yes --requirepass "$PASS"'

# Stop and remove the Redis container
redis-down:
  docker stop redis || true
  docker rm redis || true

# Wait until Redis responds to PING (avoid race where server starts before Redis is ready)
redis-wait password="":
  sh -c 'PASS="{{password}}"; PASS="${PASS:-${REDIS_PASSWORD:-changeme}}"; for i in $$(seq 1 60); do docker exec redis redis-cli -a "$$PASS" PING >/dev/null 2>&1 && exit 0; sleep 0.5; done; echo "Redis not ready after 30s" >&2; exit 1'

# Open a redis-cli shell against the local container (auths with the given password)
redis-cli password="":
  sh -c 'PASS="{{password}}"; PASS="${PASS:-${REDIS_PASSWORD:-changeme}}"; docker exec -it redis redis-cli -a "$PASS"'

# Deploy React client (dist folder) to EC2
deploy-client:
  just build
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'mkdir -p /home/ubuntu/small-talk/apps/app/client'
  scp -i $SSH_KEY -r apps/app/client/dist/ ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/client/
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'sudo systemctl restart react-client'

# Deploy Go server to EC2
deploy-server:
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'mkdir -p /home/ubuntu/small-talk/apps/app/server'
  scp -i $SSH_KEY -r apps/app/server/* ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/server/
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'cd /home/ubuntu/small-talk/apps/app/server && GO111MODULE=on $(command -v go) build -o app && sudo systemctl restart app'

# Deploy both frontend (dist) and backend simultaneously
deploy:
  just build
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'mkdir -p /home/ubuntu/small-talk/apps/app/client /home/ubuntu/small-talk/apps/app/server'
  scp -i $SSH_KEY -r apps/app/client/dist/ ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/client/
  scp -i $SSH_KEY -r apps/app/server/* ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/server/
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'cd /home/ubuntu/small-talk/apps/app/server && GO111MODULE=on $(command -v go) build -o app && sudo systemctl restart app && sudo systemctl restart react-client'

# Deploy Directory service to EC2
deploy-directory:
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'mkdir -p /home/ubuntu/small-talk/apps/directory'
  scp -i $SSH_KEY -r apps/directory/* ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/directory/
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'cd /home/ubuntu/small-talk/apps/directory && GO111MODULE=on $(command -v go) build -o directory-server && sudo systemctl restart directory-server'
