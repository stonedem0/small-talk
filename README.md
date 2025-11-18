# small talk ☁️

a small, real-time chat experiment built with **go** and **react**.
nothing fancy — just websockets, redis, and curiosity… that accidentally grew into a tiny distributed system.

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

- go 1.22+
- node.js + npm
- redis 6+
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

