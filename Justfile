# Run Go server with air
server port="8080":
  cd apps/app/server && PORT={{port}} air

# Run React client with Vite
client:
  cd apps/app/client && npm run dev

# Run both servers concurrently using shell
dev port="8080":
  sh -c 'just server {{port}} & just client'

# Build React client
build:
  cd apps/app/client && npm run build

# Run in production (Go server + built React app)
start:
  sh -c 'just server-prod & just client-prod'

# Run Go server in production
server-prod port="8080":
  cd apps/app/server && PORT={{port}} go run main.go

# Preview built React app in production
client-prod:
  cd apps/app/client && npm run preview

# Run Directory service (set DIRECTORY_PORT to override, default 8081)
directory port="8081":
  cd apps/directory && DIRECTORY_PORT={{port}} go run .

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
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'cd /home/ubuntu/small-talk/apps/app/server && GO111MODULE=on /usr/local/go/bin/go build -o chat-server && sudo systemctl restart chat-server'

# Deploy both frontend (dist) and backend simultaneously
deploy:
  just build
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'mkdir -p /home/ubuntu/small-talk/apps/app/client /home/ubuntu/small-talk/apps/app/server'
  scp -i $SSH_KEY -r apps/app/client/dist/ ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/client/
  scp -i $SSH_KEY -r apps/app/server/* ubuntu@$EC2_IP:/home/ubuntu/small-talk/apps/app/server/
  ssh -i $SSH_KEY ubuntu@$EC2_IP 'cd /home/ubuntu/small-talk/apps/app/server && GO111MODULE=on /usr/local/go/bin/go build -o chat-server && sudo systemctl restart chat-server && sudo systemctl restart react-client'

