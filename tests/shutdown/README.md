Shutdown test

Usage:

1) Install deps

   npm install

2) Run test (from this directory):

   npm run test:shutdown -- --url ws://localhost:8080/ws?room=test\&token=YOUR_JWT

   # Optionally pass --pid; if omitted, the script will attempt to auto-resolve
   # the server PID from the WebSocket port using lsof on macOS.


