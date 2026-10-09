package main

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// A room is hosted by exactly one node. The directory assigns it by writing a
// lease key in Redis; the node must agree before accepting a WebSocket for it, so
// a client with a stale URL or a direct connection cannot split a room across nodes.
//
// The key format and TTL must match apps/directory/redis.go (key, leaseTTL).
// They are duplicated because the app server deploys without internal/shared.
const roomLeaseTTL = 60 * time.Second

func roomLeaseKey(room string) string { return "directory:room:" + room }

// roomLeaseScript returns the current owner, claiming the room for ARGV[1] if it
// has none, in one round trip so two nodes cannot both think they won.
var roomLeaseScript = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if cur then return cur end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
return ARGV[1]
`)

// claimOrGetRoomOwner returns the node that owns the room's lease, atomically
// claiming it for this node when nobody holds it. The directory's heartbeat keeps
// the lease alive for rooms with connected clients.
func claimOrGetRoomOwner(room string) (string, error) {
	return roomLeaseScript.Run(ctx, RDB, []string{roomLeaseKey(room)}, appID, int(roomLeaseTTL.Seconds())).Text()
}
