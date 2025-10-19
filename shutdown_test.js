// graceful_test.js
// Usage:
//   node graceful_test.js --url ws://localhost:8080/ws?room=test&token=... --pid 12345
//
// Requirements:
// - server must accept ?token=... for dev WS auth (or change URL to use your subprotocol approach)
// - server should broadcast a Message with Type="system" and Message="server_shutdown" during shutdown
//
// The script will create many clients, pause one client's socket to simulate slowness,
// publish a burst of messages from a publisher client, then send SIGTERM to the server pid.

const WebSocket = require("ws");
const axios = require("axios");
const yargs = require("yargs/yargs");
const { execFileSync } = require("child_process");
const { hideBin } = require("yargs/helpers");

const argv = yargs(hideBin(process.argv))
  .option("url", {
    type: "string",
    demandOption: true,
    describe: "WebSocket URL",
  })
  .option("pid", {
    type: "number",
    demandOption: false,
    describe: "Server PID to SIGTERM (optional; will auto-resolve if omitted)",
  })
  .option("clients", {
    type: "number",
    default: 10,
    describe: "Total clients to open",
  })
  .option("messages", {
    type: "number",
    default: 200,
    describe: "Number of burst messages to send",
  })
  .option("room", {
    type: "string",
    default: "test",
    describe: "Room name (informational)",
  }).argv;

const WS_URL = argv.url;
let PID = argv.pid;
const NUM_CLIENTS = argv.clients;
const NUM_MESSAGES = argv.messages;
const ROOM = argv.room;

function wait(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function makeClient(id, slow = false) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(WS_URL);
    // Track per-client shutdown state for better close diagnostics
    ws._meta = { id, sawShutdown: false };

    ws.on("open", () => {
      console.log(`[client ${id}] open`);
      if (slow) {
        // pause the underlying socket to simulate a client that stops reading.
        // This will create backpressure on the server side.
        if (ws._socket && typeof ws._socket.pause === "function") {
          ws._socket.pause();
          console.log(`[client ${id}] paused underlying socket (slow reader)`);
        } else {
          console.warn(
            `[client ${id}] cannot pause socket, slow simulation may be weaker`
          );
        }
      }
      resolve(ws);
    });

    ws.on("error", (err) => {
      console.error(`[client ${id}] error`, err && err.message);
      reject(err);
    });

    ws.on("close", (code, reason) => {
      const reasonStr = reason && reason.length ? reason.toString() : "";
      const fromServerFrame = code !== 1006; // 1006 means no close frame was received
      const afterShutdown = !!(ws._meta && ws._meta.sawShutdown);
      console.log(
        `[client ${id}] closed: code=${code} reason=${reasonStr} fromServerFrame=${fromServerFrame} afterShutdown=${afterShutdown}`
      );
      // Record for summary
      if (globalThis.__closeEvents) {
        globalThis.__closeEvents.push({ id, code, reason: reasonStr, fromServerFrame, afterShutdown });
      }
    });

    ws.on("message", (buf) => {
      try {
        const msg = JSON.parse(buf.toString());
        if (msg && msg.type === "system" && msg.message === "server_shutdown") {
          console.log(`[client ${id}] got server_shutdown`);
          if (ws._meta) ws._meta.sawShutdown = true;
        }
      } catch {}
    });
  });
}

function resolvePidFromUrl(urlStr) {
  try {
    const u = new URL(urlStr);
    const port = u.port ? Number(u.port) : u.protocol === "wss:" ? 443 : 80;
    if (!port) return null;
    // macOS: lsof -nP -iTCP:PORT -sTCP:LISTEN -t
    const out = execFileSync("lsof", ["-nP", `-iTCP:${port}`, "-sTCP:LISTEN", "-t"], { encoding: "utf8" });
    const lines = out.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    return lines.length ? Number(lines[0]) : null;
  } catch (e) {
    return null;
  }
}

async function publishBurst(publisherWs, n) {
  console.log(`publishing burst of ${n} messages`);
  for (let i = 0; i < n; i++) {
    const m = {
      room: ROOM,
      username: "publisher",
      message: `burst-${i}`,
      type: "chat",
      timestamp: new Date().toISOString(),
    };
    publisherWs.send(JSON.stringify(m));
    // micro-wait so we don't saturate local buffers too hard
    if (i % 50 === 0) await wait(5);
  }
  console.log("publisher finished sending burst");
}

async function tryNewConnect(id, timeoutMs = 3000) {
  return new Promise((resolve) => {
    const ws = new WebSocket(WS_URL);
    let done = false;
    const t = setTimeout(() => {
      if (!done) {
        done = true;
        console.log(`[newconn ${id}] timed out after ${timeoutMs}ms`);
        try {
          ws.terminate();
        } catch (e) {}
        resolve(false);
      }
    }, timeoutMs);

    ws.on("open", () => {
      if (!done) {
        done = true;
        clearTimeout(t);
        console.log(
          `[newconn ${id}] succeeded (unexpected if server stopped accepting)`
        );
        ws.close();
        resolve(true);
      }
    });
    ws.on("error", (err) => {
      if (!done) {
        done = true;
        clearTimeout(t);
        console.log(
          `[newconn ${id}] error (expected after shutdown):`,
          err.message
        );
        resolve(false);
      }
    });
    ws.on("close", () => {
      if (!done) {
        done = true;
        clearTimeout(t);
        console.log(`[newconn ${id}] closed`);
        resolve(false);
      }
    });
  });
}

(async function main() {
  // Collect close events for a summary at the end
  globalThis.__closeEvents = [];
  console.log("Test starting");
  console.log(
    "WS URL:",
    WS_URL,
    "PID:",
    PID,
    "clients:",
    NUM_CLIENTS,
    "messages:",
    NUM_MESSAGES
  );

  // 1) open clients
  const clients = [];
  for (let i = 0; i < NUM_CLIENTS; i++) {
    const slow = i === 0; // make client 0 the slow one
    try {
      const ws = await makeClient(i, slow);
      clients.push({ id: i, ws, slow });
      // slightly stagger connects to avoid thundering herd
      await wait(10);
    } catch (e) {
      console.error(`[client ${i}] failed to connect:`, e && e.message);
    }
  }

  // small wait for all subscriptions to settle
  console.log("All clients connected. Waiting 250ms to settle...");
  await wait(250);

  // 2) pick a publisher client (not the slow one)
  const pub = clients.find((c) => !c.slow);
  if (!pub) {
    console.error("No publisher client available (all were slow). Exiting.");
    process.exit(1);
  }

  // 3) publish burst
  await publishBurst(pub.ws, NUM_MESSAGES);

  // 4) wait a short moment to let server fanout
  await wait(500);

  // 5) send SIGTERM to server PID (auto-resolve if missing/invalid)
  if (!PID) {
    PID = resolvePidFromUrl(WS_URL);
    if (PID) {
      console.log(`resolved PID from port: ${PID}`);
    } else {
      console.warn("Could not resolve PID from URL. Pass --pid explicitly.");
    }
  }
  if (PID) {
    console.log(`sending SIGTERM to pid ${PID}`);
    try {
      process.kill(PID, "SIGTERM");
      console.log("SIGTERM sent");
    } catch (err) {
      console.error("Failed to send SIGTERM:", err && err.message);
      // If ESRCH (no such process), try to resolve again and retry once
      const retryPid = resolvePidFromUrl(WS_URL);
      if (retryPid && retryPid !== PID) {
        PID = retryPid;
        console.log(`retrying SIGTERM to resolved pid ${PID}`);
        try {
          process.kill(PID, "SIGTERM");
          console.log("SIGTERM sent on retry");
        } catch (e2) {
          console.error("Retry SIGTERM failed:", e2 && e2.message);
        }
      }
    }
  } else {
    console.warn("No PID available; skipping SIGTERM step.");
  }

  // 6) attempt new connections (they should fail fast / be refused)
  console.log(
    "Attempting a few new connections to verify server refuses accepts..."
  );
  const newConnResults = await Promise.all([
    tryNewConnect("A", 2000),
    tryNewConnect("B", 2000),
  ]);
  console.log("New connection attempts results:", newConnResults);

  // 7) wait for server_shutdown message and disconnections
  console.log("Waiting up to 6s for clients to receive shutdown and close...");
  const shutdownDeadline = Date.now() + 6000;
  while (Date.now() < shutdownDeadline) {
    // check if process still exists
    try {
      process.kill(PID, 0); // check existence (no signal)
      // still alive
    } catch (e) {
      console.log("Server process no longer exists (exited).");
      break;
    }
    await wait(200);
  }

  // 8) cleanup local clients
  console.log("Cleaning up local clients...");
  for (const c of clients) {
    try {
      c.ws.terminate();
    } catch (e) {}
  }

  // Short summary of close codes observed
  const counts = new Map();
  for (const e of globalThis.__closeEvents) {
    const key = String(e.code);
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  const summary = [...counts.entries()].map(([code, count]) => `${code}: ${count}`).join(", ");
  console.log(`Close code summary: { ${summary} }`);
  const afterShutdownStats = globalThis.__closeEvents.reduce(
    (acc, e) => {
      if (e.afterShutdown) acc.after++;
      else acc.before++;
      if (e.fromServerFrame) acc.withFrame++;
      else acc.withoutFrame++;
      return acc;
    },
    { after: 0, before: 0, withFrame: 0, withoutFrame: 0 }
  );
  console.log(
    `Close timing/frame summary: afterShutdown=${afterShutdownStats.after}, beforeShutdown=${afterShutdownStats.before}, withFrame=${afterShutdownStats.withFrame}, withoutFrame=${afterShutdownStats.withoutFrame}`
  );

  console.log("Test finished.");
  process.exit(0);
})();
