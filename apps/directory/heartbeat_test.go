package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupHeartbeatRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: s.Addr()})
	return s
}

func heartbeatRequest(t *testing.T, payload any) *http.Request {
	t.Helper()
	b, _ := json.Marshal(payload)
	r := httptest.NewRequest(http.MethodPost, "/heartbeat", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestHeartbeatHandler_WrongMethod(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	s.HeartbeatHandler(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHeartbeatHandler_InvalidBody(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/heartbeat", bytes.NewBufferString("not json"))
	s.HeartbeatHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHeartbeatHandler_MissingAppID(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	s.HeartbeatHandler(w, heartbeatRequest(t, Heartbeat{WSURL: "ws://app-1:8080/ws"}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHeartbeatHandler_MissingWSURL(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	s.HeartbeatHandler(w, heartbeatRequest(t, Heartbeat{AppID: "app-1"}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHeartbeatHandler_RegistersNewApp(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	s.HeartbeatHandler(w, heartbeatRequest(t, Heartbeat{
		AppID: "app-1",
		WSURL: "ws://app-1:8080/ws",
	}))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	s.mu.RLock()
	a, ok := s.apps["app-1"]
	s.mu.RUnlock()

	if !ok {
		t.Fatal("expected app-1 to be registered in state")
	}
	if a.WSURL != "ws://app-1:8080/ws" {
		t.Fatalf("unexpected WSURL: %s", a.WSURL)
	}
	if !a.Healthy {
		t.Fatal("expected app to be marked healthy after heartbeat")
	}
}

func TestHeartbeatHandler_UpdatesExistingApp(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	// first heartbeat
	s.HeartbeatHandler(httptest.NewRecorder(), heartbeatRequest(t, Heartbeat{
		AppID:      "app-1",
		WSURL:      "ws://app-1:8080/ws",
		UsersTotal: 10,
	}))

	// second heartbeat with updated stats
	w := httptest.NewRecorder()
	s.HeartbeatHandler(w, heartbeatRequest(t, Heartbeat{
		AppID:      "app-1",
		WSURL:      "ws://app-1:8080/ws",
		UsersTotal: 50,
	}))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	s.mu.RLock()
	total := s.apps["app-1"].UsersTotal
	s.mu.RUnlock()

	if total != 50 {
		t.Fatalf("expected UsersTotal=50 after update, got %d", total)
	}
}

func TestHeartbeatHandler_DrainingSetsFlag(t *testing.T) {
	setupHeartbeatRedis(t)
	s := NewState()

	s.HeartbeatHandler(httptest.NewRecorder(), heartbeatRequest(t, Heartbeat{
		AppID:    "app-1",
		WSURL:    "ws://app-1:8080/ws",
		Draining: true,
	}))

	s.mu.RLock()
	draining := s.apps["app-1"].Draining
	s.mu.RUnlock()

	if !draining {
		t.Fatal("expected app-1 to be marked as draining")
	}
}

func TestHeartbeatHandler_RefreshesLeaseForActiveRooms(t *testing.T) {
	s2 := setupHeartbeatRedis(t)
	s := NewState()

	// pre-claim the room
	_, _ = TryClaim("gaming", "app-1")

	s.HeartbeatHandler(httptest.NewRecorder(), heartbeatRequest(t, Heartbeat{
		AppID: "app-1",
		WSURL: "ws://app-1:8080/ws",
		Rooms: map[string]int{"gaming": 5},
	}))

	// TTL should be refreshed to leaseTTL
	ttl := s2.TTL(key("gaming"))
	if ttl < 55*leaseTTL/60 {
		t.Fatalf("expected lease TTL to be refreshed, got %v", ttl)
	}
}
