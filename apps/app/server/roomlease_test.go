package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClaimOrGetRoomOwner_ClaimsUnownedRoom(t *testing.T) {
	mr := setupAppRedis(t)

	owner, err := claimOrGetRoomOwner("gaming")
	if err != nil {
		t.Fatal(err)
	}
	if owner != appID {
		t.Fatalf("owner %q, want this node %q", owner, appID)
	}
	if got, _ := mr.Get(roomLeaseKey("gaming")); got != appID {
		t.Fatalf("lease value %q, want %q", got, appID)
	}
	if ttl := mr.TTL(roomLeaseKey("gaming")); ttl < 55*time.Second || ttl > roomLeaseTTL {
		t.Fatalf("lease ttl %v, want about %v", ttl, roomLeaseTTL)
	}
}

func TestClaimOrGetRoomOwner_ReturnsExistingOwnerWithoutStealing(t *testing.T) {
	mr := setupAppRedis(t)
	_ = mr.Set(roomLeaseKey("gaming"), "other-node")
	mr.SetTTL(roomLeaseKey("gaming"), 30*time.Second)

	owner, err := claimOrGetRoomOwner("gaming")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "other-node" {
		t.Fatalf("owner %q, want other-node", owner)
	}
	if got, _ := mr.Get(roomLeaseKey("gaming")); got != "other-node" {
		t.Fatalf("lease was overwritten with %q", got)
	}
	if ttl := mr.TTL(roomLeaseKey("gaming")); ttl > 30*time.Second {
		t.Fatalf("lease ttl was extended to %v", ttl)
	}
}

func TestHandleConnections_RefusesRoomOwnedByAnotherNode(t *testing.T) {
	mr := setupAppRedis(t)
	_ = mr.Set(roomLeaseKey("gaming"), "other-node")

	w := httptest.NewRecorder()
	handleConnections(newApp(), w, wsRequest("gaming", makeToken("alice", time.Now().Add(time.Hour))))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	roomsLock.RLock()
	n := len(rooms["gaming"])
	roomsLock.RUnlock()
	if n != 0 {
		t.Fatalf("refused connection still registered %d clients", n)
	}
}

func TestHandleConnections_AcceptsRoomOwnedByThisNode(t *testing.T) {
	mr := setupAppRedis(t)
	_ = mr.Set(roomLeaseKey("gaming"), appID)

	w := httptest.NewRecorder()
	handleConnections(newApp(), w, wsRequest("gaming", makeToken("alice", time.Now().Add(time.Hour))))

	if !reachedUpgrader(w) {
		t.Fatalf("expected to reach the upgrader, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleConnections_RedisDownIs503(t *testing.T) {
	mr := setupAppRedis(t)
	mr.Close()

	w := httptest.NewRecorder()
	handleConnections(newApp(), w, wsRequest("gaming", makeToken("alice", time.Now().Add(time.Hour))))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// End to end over a real WebSocket: a node claims an unowned room on first
// connect and a second node's lease blocks connections to that room.
func TestWebSocket_ClaimsUnownedRoomAndRefusesForeignRoom(t *testing.T) {
	srv := startWSServer(t)

	c := dialRoom(t, srv, "free_room", "alice")
	eventually(t, "room claimed by this node", func() bool {
		v, err := RDB.Get(ctx, roomLeaseKey("free_room")).Result()
		return err == nil && v == appID
	})
	_ = c

	if err := RDB.Set(ctx, roomLeaseKey("foreign_room"), "other-node", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=foreign_room"
	h := http.Header{}
	h.Set("Origin", testOrigin)
	h.Set("Authorization", "Bearer "+makeToken("alice", time.Now().Add(time.Hour)))
	_, resp, err := websocket.DefaultDialer.Dial(url, h)
	if err == nil {
		t.Fatal("expected the foreign room to be refused")
	}
	if resp == nil || resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %v (err=%v)", resp, err)
	}
}
