Small Talk - systemd deployment
================================

This directory contains unit files and environment templates to run both services on an Ubuntu/Debian EC2 instance via systemd.

Services
- redis.service — Redis cache/broker
- small-talk-app.service — main chat app HTTP/WebSocket server
- small-talk-directory.service — directory/placement service
- react-client.service — frontend (Vite preview / static server)

Install (as root)
1) Create dirs for app/dir/frontend:
   mkdir -p /home/ubuntu/small-talk/apps/{app/server,directory,app/client}
   mkdir -p /etc/small-talk

2) Build binaries/frontend (adjust as needed):
   # App server
   (cd /home/ubuntu/small-talk/apps/app/server && go build -o app)
   # Directory service
   (cd /home/ubuntu/small-talk/apps/directory && go build -o directory)
   # Frontend (Vite preview or static)
   (cd /home/ubuntu/small-talk/apps/app/client && npm install && npm run build)

3) Configure env files:
   cp app.env.example /home/ubuntu/small-talk/apps/app/server/app.env
   cp directory.env.example /home/ubuntu/small-talk/apps/directory/directory.env
   # Edit and set secrets, CORS, Redis, etc.  chmod 600 the env files.

4) Install unit files:
   cp redis.service /etc/systemd/system/redis.service
   cp small-talk-app.service /etc/systemd/system/app.service
   cp small-talk-directory.service /etc/systemd/system/directory.service
   cp react-client.service /etc/systemd/system/react-client.service
   systemctl daemon-reload

5) Enable and start:
   systemctl enable --now redis.service
   systemctl enable --now app.service
   systemctl enable --now directory.service
   systemctl enable --now react-client.service

6) Check status/logs:
   systemctl status app.service
   journalctl -u app.service -f

Notes
- Ensure JWT_SECRET (or DIRECTORY_JWT_SECRET) used by directory equals the app’s signing secret.
- Open security groups for PORT (default 8080) and DIRECTORY_PORT (default 8081) as needed.
- You can deploy via Terraform/user_data as well; render these files in cloud-init and use the same steps.

