# Small Talk

A real-time chat application built with Go and React.

## Project Structure

- `client/` - React frontend application
- `server/` - Go backend server
- `Justfile` - Build and deployment commands

## Prerequisites

- Go 1.x
- Node.js and npm
- Just (command runner)

## Setup

1. Clone the repository:
```bash
git clone https://github.com/yourusername/small-talk.git
cd small-talk
```

2. Install dependencies:
```bash
# Install backend dependencies
cd server
go mod download

# Install frontend dependencies
cd ../client
npm install
```

## Development

The project uses Just for running various commands. Here are the main development commands:

- `just server` - Run Go server with hot reload
- `just client` - Run React client with Vite
- `just dev` - Run both servers concurrently
- `just build` - Build React client
- `just start` - Run in production mode

## Deployment

The project includes deployment scripts for EC2:

- `just deploy-client` - Deploy React client to EC2
- `just deploy-server` - Deploy Go server to EC2
- `just deploy` - Deploy both frontend and backend

Note: Deployment requires setting up the following environment variables:
- `SSH_KEY` - Path to your SSH key
- `EC2_IP` - IP address of your EC2 instance

## License

MIT 