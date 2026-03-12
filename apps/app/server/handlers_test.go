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

// --- GetRoomsWithCategoriesHandler ---

func TestGetRoomsWithCategoriesHandler_Empty(t *testing.T) {
	setupHandlerRedis(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rooms-with-categories", nil)
	newHandler().GetRoomsWithCategoriesHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string][]string
	_ = json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestGetRoomsWithCategoriesHandler_Grouped(t *testing.T) {
	setupHandlerRedis(t)

	RDB.SAdd(ctx, "rooms", "gaming", "music", "anime")
	RDB.HSet(ctx, "room:categories", "gaming", "gaming", "music", "music", "anime", "anime & arts")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rooms-with-categories", nil)
	newHandler().GetRoomsWithCategoriesHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string][]string
	_ = json.NewDecoder(w.Body).Decode(&result)

	if len(result["gaming"]) != 1 || result["gaming"][0] != "gaming" {
		t.Fatalf("expected gaming category to have [gaming], got %v", result["gaming"])
	}
	if len(result["music"]) != 1 || result["music"][0] != "music" {
		t.Fatalf("expected music category to have [music], got %v", result["music"])
	}
	if len(result["anime & arts"]) != 1 || result["anime & arts"][0] != "anime" {
		t.Fatalf("expected anime & arts category to have [anime], got %v", result["anime & arts"])
	}
}

func TestGetRoomsWithCategoriesHandler_UncategorizedFallsToGeneral(t *testing.T) {
	setupHandlerRedis(t)

	RDB.SAdd(ctx, "rooms", "randomroom")
	// no category set for randomroom

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rooms-with-categories", nil)
	newHandler().GetRoomsWithCategoriesHandler(w, r)

	var result map[string][]string
	_ = json.NewDecoder(w.Body).Decode(&result)

	if len(result["general"]) != 1 || result["general"][0] != "randomroom" {
		t.Fatalf("expected uncategorized room in general, got %v", result)
	}
}

func TestGetRoomsWithCategoriesHandler_MultipleRoomsPerCategory(t *testing.T) {
	setupHandlerRedis(t)

	RDB.SAdd(ctx, "rooms", "gaming", "nerd_herd")
	RDB.HSet(ctx, "room:categories", "gaming", "gaming", "nerd_herd", "gaming")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/rooms-with-categories", nil)
	newHandler().GetRoomsWithCategoriesHandler(w, r)

	var result map[string][]string
	_ = json.NewDecoder(w.Body).Decode(&result)

	if len(result["gaming"]) != 2 {
		t.Fatalf("expected 2 rooms in gaming category, got %v", result["gaming"])
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
