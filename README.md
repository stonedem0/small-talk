# small talk ☁️

a real-time chat app built with **go** and **react**. started as a websocket experiment, grew into a small distributed system.

the backend is a go server handling websocket connections, jwt auth, and postgres persistence. rooms are distributed across multiple app nodes via a separate **directory service** that uses consistent hashing (HRW) and redis leases to decide which node owns each room — so users in the same room always land on the same server. a redis pubsub layer bridges nodes so messages flow correctly regardless of where a client connects.

the frontend is a react + vite SPA with a retro-inspired UI. users can join public chat rooms, send direct messages, manage a friends list, set custom statuses, and pin favorite rooms to their sidebar.

---

## features

- real-time chat rooms with categories
- direct messages (DMs) between users
- friends system (send / accept / decline / remove)
- custom user statuses (set from the topbar, broadcasts live)
- favorite rooms (pinned in the sidebar, persisted per user)
- online presence — see who's in a room and which friends are online
- typing indicators
- chat history (last 100 messages per room via redis)
- jwt auth with refresh tokens
- username updates broadcast live to all room members

---

## structure

```
apps/
  app/
    server/     → go websocket backend
    client/     → react frontend (vite)
  directory/    → go directory service (room placement, hrw hashing, leases)
internal/
  shared/       → hrw hashing, shared structs, helpers
Justfile        → build, run, deploy tasks
```

---

## prerequisites

- go 1.25+
- node.js + npm
- redis 6+
- postgresql 14+
- just task runner

---

## environment variables

### app server (`apps/app/server`)
```
PORT=8080
JWT_SECRET=your-secret
REFRESH_JWT_SECRET=your-refresh-secret
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=devpass123
POSTGRES_URL=postgres://user:pass@localhost:5432/smalltalk?sslmode=disable
DIRECTORY_URL=http://localhost:8081
CORS_ORIGINS=http://localhost:5173
```

### directory service (`apps/directory`)
```
DIRECTORY_PORT=8081
JWT_SECRET=your-secret
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=devpass123
CORS_ORIGINS=http://localhost:5173
```

### react client (`apps/app/client`)
```
VITE_API_URL=http://localhost:8080
VITE_DIRECTORY_URL=http://localhost:8081
```

### tests
```
POSTGRES_TEST_URL=postgres://user:pass@localhost:5432/smalltalk_test?sslmode=disable
```

---

## installing

```bash
git clone https://github.com/yourusername/small-talk.git
cd small-talk

# backend
cd apps/app/server
go mod download

# directory
cd ../../directory
go mod download

# frontend
cd ../app/client
npm install
```

---

## development workflow

the entire workflow is powered by `just`.

### run go server with hot reload
```bash
just server port=8080
```

### run react client
```bash
just client
```

### run both
```bash
just dev port=8080
```

### build react
```bash
just build
```

### "prod-ish" local run
```bash
just start
```

### run tests
```bash
cd apps/app/server
go test ./...
```

---

## directory service

the directory is a lightweight control-plane responsible for:

- receiving heartbeats from app nodes
- tracking number of users per room/app
- storing **room → app** ownership in redis using leases
- preventing race conditions between nodes
- choosing which node should serve a room via **HRW hashing**
- weighted HRW (cpu/mem/users aware)

### run it
```bash
just directory port=8081
```

---

## deployment to ec2

requires:
```
SSH_KEY=~/.ssh/your-key.pem
EC2_IP=x.x.x.x
```

### deploy server
```bash
just deploy-server
```

### deploy client
```bash
just deploy-client
```

### deploy both
```bash
just deploy
```

deployment uploads files, builds the go binary on ec2, and restarts
systemd units (`app`, `react-client`).

---

## scaling plans 🧪

- multi-node setup
- directory as control-plane
- HRW hashing
- redis leases
- heartbeats with metrics
- weighted placement
- draining mode

---

## license

MIT
