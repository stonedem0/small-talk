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

# Seed test friends for a given user (default: asya). Password for all fake users: "password"
seed-db user="asya":
  psql postgres://smalltalk:smalltalk@localhost:5432/smalltalk -c "\
    INSERT INTO users (username, password_hash) VALUES \
      ('alice',   '\$2a\$10\$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'), \
      ('bob',     '\$2a\$10\$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'), \
      ('charlie', '\$2a\$10\$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'), \
      ('diana',   '\$2a\$10\$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy') \
    ON CONFLICT DO NOTHING;"
  psql postgres://smalltalk:smalltalk@localhost:5432/smalltalk -c "\
    INSERT INTO friends (user_a, user_b) VALUES \
      ('{{user}}', 'alice'),   \
      ('{{user}}', 'bob'),     \
      ('{{user}}', 'charlie'), \
      ('{{user}}', 'diana')    \
    ON CONFLICT DO NOTHING;"

# Create the smalltalk Postgres user and databases (idempotent)
db-setup:
  psql postgres -c "DO \$\$ BEGIN CREATE USER smalltalk WITH PASSWORD 'smalltalk'; EXCEPTION WHEN duplicate_object THEN NULL; END \$\$;"
  psql postgres -c "SELECT 'CREATE DATABASE smalltalk OWNER smalltalk' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='smalltalk')\gexec"
  psql postgres -c "SELECT 'CREATE DATABASE smalltalk_test OWNER smalltalk' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='smalltalk_test')\gexec"

# Run the full stack: Redis + Postgres + BE + Directory + FE (uses local services, no Docker)
dev-full port="8080":
  redis-server --daemonize yes
  sh -c 'for i in $(seq 1 20); do redis-cli ping >/dev/null 2>&1 && exit 0; sleep 0.5; done; echo "Redis not ready" >&2; exit 1'
  brew services start postgresql@18 || true
  sh -c 'for i in $(seq 1 20); do pg_isready -q && exit 0; sleep 0.5; done; echo "Postgres not ready" >&2; exit 1'
  just db-setup
  just server {{port}} & just directory & just client

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

# --- Tests ---

# Run all tests
test:
  cd apps/app/server && go test ./...
  cd apps/directory && go test ./...

# Run tests for the app server only
test-server:
  cd apps/app/server && go test ./... -v

# Run tests for the directory service only
test-directory:
  cd apps/directory && go test ./... -v

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
