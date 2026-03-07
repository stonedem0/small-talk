package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupHandlerRedis(t *testing.T) {
	t.Helper()
	s := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: s.Addr()})
}

func newHandler() *Handler {
	return NewHandler(RDB, testSecret, testSecret)
}

// --- GetChatHistoryHandler ---

func TestGetChatHistoryHandler_MissingRoom(t *testing.T) {
	setupHandlerRedis(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/history", nil)
	newHandler().GetChatHistoryHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetChatHistoryHandler_EmptyRoom(t *testing.T) {
	setupHandlerRedis(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/history?room=gaming", nil)
	newHandler().GetChatHistoryHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var history []Message
	_ = json.NewDecoder(w.Body).Decode(&history)
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %d messages", len(history))
	}
}

func TestGetChatHistoryHandler_ReturnsOldestFirst(t *testing.T) {
	setupHandlerRedis(t)

	// LPUSH stores newest-first (index 0 = newest), so push in order: msg1, msg2, msg3
	// After LPush: [msg3, msg2, msg1] — handler should reverse to [msg1, msg2, msg3]
	msgs := []Message{
		{Username: "alice", Message: "first", Room: "gaming"},
		{Username: "bob", Message: "second", Room: "gaming"},
		{Username: "carol", Message: "third", Room: "gaming"},
	}
	for _, m := range msgs {
		b, _ := json.Marshal(m)
		RDB.LPush(ctx, "chat_history:gaming", b)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/history?room=gaming", nil)
	newHandler().GetChatHistoryHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var history []Message
	_ = json.NewDecoder(w.Body).Decode(&history)

	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	// oldest first
	if history[0].Message != "first" || history[2].Message != "third" {
		t.Fatalf("expected oldest-first order, got: %v %v %v",
			history[0].Message, history[1].Message, history[2].Message)
	}
}

// --- GetOnlineUsersHandler ---

func TestGetOnlineUsersHandler_SpecificRoom(t *testing.T) {
	setupHandlerRedis(t)

	onlineUsersLock.Lock()
	onlineUsers["gaming"] = map[string]bool{"alice": true, "bob": true}
	onlineUsersLock.Unlock()
	t.Cleanup(func() {
		onlineUsersLock.Lock()
		delete(onlineUsers, "gaming")
		onlineUsersLock.Unlock()
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/online-users?room=gaming", nil)
	newHandler().GetOnlineUsersHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]int
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"] != 2 {
		t.Fatalf("expected count=2, got %d", resp["count"])
	}
}

func TestGetOnlineUsersHandler_EmptyRoom(t *testing.T) {
	setupHandlerRedis(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/online-users?room=gaming", nil)
	newHandler().GetOnlineUsersHandler(w, r)

	var resp map[string]int
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"] != 0 {
		t.Fatalf("expected count=0 for empty room, got %d", resp["count"])
	}
}

func TestGetOnlineUsersHandler_AllRooms(t *testing.T) {
	setupHandlerRedis(t)

	onlineUsersLock.Lock()
	onlineUsers["gaming"] = map[string]bool{"alice": true}
	onlineUsers["music"] = map[string]bool{"bob": true, "carol": true}
	onlineUsersLock.Unlock()
	t.Cleanup(func() {
		onlineUsersLock.Lock()
		delete(onlineUsers, "gaming")
		delete(onlineUsers, "music")
		onlineUsersLock.Unlock()
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/online-users", nil)
	newHandler().GetOnlineUsersHandler(w, r)

	var resp map[string]int
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["gaming"] != 1 {
		t.Fatalf("expected gaming=1, got %d", resp["gaming"])
	}
	if resp["music"] != 2 {
		t.Fatalf("expected music=2, got %d", resp["music"])
	}
}
