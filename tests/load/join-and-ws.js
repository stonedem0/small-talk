import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '20s', target: 100 },  // ramp to 100 conns
    { duration: '2m',  target: 100 },  // hold
    { duration: '20s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
  },
};

// Env:
// DIR   = http://<directory-host>:8081
// ROOM  = gaming (or any)
// TOKEN = your valid JWT
const DIR   = __ENV.DIR   || 'http://localhost:8081';
const ROOM  = __ENV.ROOM  || 'gaming';
const TOKEN = __ENV.TOKEN || '';

export default function () {
  const res = http.get(`${DIR}/join?room=${encodeURIComponent(ROOM)}`, {
    headers: TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {},
    timeout: '5s',
  });

  check(res, { 'join 200': (r) => r.status === 200 });

  if (res.status !== 200) {
    sleep(1);
    return;
  }

  const data = res.json();
  const url = data.wss_url; // e.g. ws://host:8080/ws?room=gaming
  const params = { headers: { 'Sec-WebSocket-Protocol': `Bearer ${TOKEN}` } };

  const out = ws.connect(url, params, function (socket) {
    socket.on('open', function () {
      socket.send(JSON.stringify({ message: 'hello from k6' }));
      socket.setInterval(function () {
        socket.send(JSON.stringify({ message: 'ping' }));
      }, 10000 + Math.random() * 5000); // 10–15s
    });

    socket.on('message', function (_msg) {
    });

    socket.on('error', function (e) {
      console.error('ws error', e);
    });
    socket.setTimeout(function () {
      socket.close();
    }, 30000 + Math.random() * 15000);
  });

  check(out, { 'ws ok': (o) => !o || !o.error });
  sleep(1);
}