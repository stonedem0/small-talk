Small Talk - systemd deployment
================================

This directory contains unit files and environment templates to run both services on an Ubuntu/Debian EC2 instance via systemd.

Services
- small-talk-app.service — main chat app HTTP/WebSocket server
- small-talk-directory.service — directory/placement service

Install (as root)
1) Create runtime user and dirs:
   useradd --system --home /var/lib/small-talk --shell /usr/sbin/nologin smalltalk || true
   mkdir -p /var/lib/small-talk /etc/small-talk
   chown -R smalltalk:smalltalk /var/lib/small-talk

2) Build and install binaries (example; adjust Go version and paths as needed):
   # App server
   (cd /opt/small-talk/apps/app/server && go build -o /usr/local/bin/small-talk-app)
   # Directory service
   (cd /opt/small-talk/apps/directory && go build -o /usr/local/bin/small-talk-directory)

3) Configure env files:
   cp app.env.example /etc/small-talk/app.env
   cp directory.env.example /etc/small-talk/directory.env
   # Edit and set secrets, CORS, Redis, and ports

4) Install unit files:
   cp small-talk-app.service /etc/systemd/system/
   cp small-talk-directory.service /etc/systemd/system/
   systemctl daemon-reload

5) Enable and start:
   systemctl enable --now small-talk-app.service
   systemctl enable --now small-talk-directory.service

6) Check status/logs:
   systemctl status small-talk-app.service
   journalctl -u small-talk-app.service -f

Notes
- Ensure JWT_SECRET (or DIRECTORY_JWT_SECRET) used by directory equals the app’s signing secret.
- Open security groups for PORT (default 8080) and DIRECTORY_PORT (default 8081) as needed.
- You can deploy via Terraform/user_data as well; render these files in cloud-init and use the same steps.

