# small talk ☁️

a small, real-time chat experiment built with **go** and **react**.  
nothing fancy — just websockets, redis, and curiosity.

---

## structure

```
client/   → react frontend  
server/   → go backend  
Justfile  → build & deploy tasks
```

---

## getting started

### prerequisites
- go 1.x  
- node.js + npm  
- [just](https://github.com/casey/just) (command runner)

### setup
```bash
git clone https://github.com/yourusername/small-talk.git
cd small-talk

# backend
cd server
go mod download

# frontend
cd ../client
npm install
```

---

## dev mode

the project uses **just** for tasks:

```bash
just server   # run go server with hot reload
just client   # run react client with vite
just dev      # run both servers together
just build    # build react app
just start    # run in production mode
```

---

## deployment

basic ec2 deployment via ssh:

```bash
just deploy-server   # deploy go backend
just deploy-client   # deploy react frontend
just deploy          # deploy both
```

required env vars:
```
SSH_KEY   → path to ssh key  
EC2_IP    → ec2 public ip
```

---

## ☁️ scaling plans

this project started as a sandbox — a way to learn how far a single go + redis setup can stretch.  
the long-term goal is to see if it can handle **tens of thousands** of concurrent websocket connections  
without turning into a distributed monster or breaking the bank.

### next steps
- run multiple gateway instances behind an **aws alb**  
- keep **redis** as shared pub/sub  
- tune linux limits (`ulimit`, tcp keepalive, etc.) for high connection counts  
- add small metrics for connections, memory, latency

### target capacity
- ~10–20k sockets per instance  
- scale to **~100k total** across 6–10 ec2 nodes  
- single redis node for now

### cost awareness 💸
this is just for fun and learning — not optimized for cost.  
rough aws estimate for a 100k-connection experiment:

| service | est. monthly |
|----------|--------------:|
| ec2 (8× c7g.xlarge) | ~$850 |
| alb | ~$200 |
| redis (elasticache r7g.large/xlarge) | ~$320–640 |
| **total (no egress)** | **≈ $1.4k–$1.7k** |
| data transfer (depends on chat volume) | +$500–$4k |

for now, a **single node + redis** is enough for local and dev use.  
i’ll scale it later when i want to benchmark real load.

---

## license

MIT
