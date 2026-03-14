package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
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
		{Username: "rei", Message: "first", Room: "gaming"},
		{Username: "shinji", Message: "second", Room: "gaming"},
		{Username: "asuka", Message: "third", Room: "gaming"},
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
	onlineUsers["gaming"] = map[string]bool{"rei": true, "shinji": true}
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
	onlineUsers["gaming"] = map[string]bool{"rei": true}
	onlineUsers["music"] = map[string]bool{"shinji": true, "asuka": true}
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

// --- isDMRoom / isDMParticipant ---

func TestIsDMRoom(t *testing.T) {
	if !isDMRoom("dm:rei:shinji") {
		t.Fatal("expected dm:rei:shinji to be a DM room")
	}
	if isDMRoom("gaming") {
		t.Fatal("expected gaming to not be a DM room")
	}
}

func TestIsDMParticipant(t *testing.T) {
	if !isDMParticipant("dm:rei:shinji", "rei") {
		t.Fatal("rei should be a participant")
	}
	if !isDMParticipant("dm:rei:shinji", "shinji") {
		t.Fatal("shinji should be a participant")
	}
	if isDMParticipant("dm:rei:shinji", "asuka") {
		t.Fatal("asuka should not be a participant")
	}
}

// --- StartDMHandler ---

func TestStartDMHandler_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"shinji"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dm/start", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().StartDMHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["room"] != "dm:rei:shinji" {
		t.Fatalf("expected room dm:rei:shinji, got %q", resp["room"])
	}

	// both DM sets should be registered
	reiDMs, _ := RDB.SMembers(ctx, "dms:rei").Result()
	shinjiDMs, _ := RDB.SMembers(ctx, "dms:shinji").Result()
	if len(reiDMs) != 1 || reiDMs[0] != "shinji" {
		t.Fatalf("expected dms:rei = [shinji], got %v", reiDMs)
	}
	if len(shinjiDMs) != 1 || shinjiDMs[0] != "rei" {
		t.Fatalf("expected dms:shinji = [rei], got %v", shinjiDMs)
	}
}

func TestStartDMHandler_RoomNameSorted(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "misato", "$2a$10$x")

	// zara DMs alice — sorted: misato > rei, sorted: rei < misato → dm:misato:rei
	tok := makeToken("misato", time.Now().Add(time.Hour))
	body := `{"target":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dm/start", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().StartDMHandler(w, r)

	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["room"] != "dm:misato:rei" {
		t.Fatalf("expected dm:misato:rei, got %q", resp["room"])
	}
}

func TestStartDMHandler_CannotDMSelf(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dm/start", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().StartDMHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStartDMHandler_TargetNotFound(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"kaworu"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dm/start", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().StartDMHandler(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- GetDMListHandler ---

func TestGetDMListHandler_ReturnsList(t *testing.T) {
	setupHandlerRedis(t)

	RDB.SAdd(ctx, "dms:rei", "shinji", "asuka")

	tok := makeToken("rei", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dms", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().GetDMListHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var partners []string
	_ = json.NewDecoder(w.Body).Decode(&partners)
	if len(partners) != 2 {
		t.Fatalf("expected 2 partners, got %v", partners)
	}
}

func TestGetDMListHandler_Empty(t *testing.T) {
	setupHandlerRedis(t)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/dms", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().GetDMListHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var partners []string
	_ = json.NewDecoder(w.Body).Decode(&partners)
	if len(partners) != 0 {
		t.Fatalf("expected empty list, got %v", partners)
	}
}

// --- GetChatHistoryHandler DM guard ---

func TestGetChatHistoryHandler_DMForbidsNonParticipant(t *testing.T) {
	setupHandlerRedis(t)

	tok := makeToken("asuka", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/history?room=dm:rei:shinji", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().GetChatHistoryHandler(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-participant, got %d", w.Code)
	}
}

func TestGetChatHistoryHandler_DMAllowsParticipant(t *testing.T) {
	setupHandlerRedis(t)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/history?room=dm:rei:shinji", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().GetChatHistoryHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for participant, got %d", w.Code)
	}
}

// --- Postgres helpers ---

func setupTestDB(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set, skipping postgres tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	DB = db
	migrateDB()
	// clean slate for each test
	db.Exec(`TRUNCATE users, friends, friend_requests RESTART IDENTITY CASCADE`)
	t.Cleanup(func() {
		db.Exec(`TRUNCATE users, friends, friend_requests RESTART IDENTITY CASCADE`)
	})
}

func insertUser(t *testing.T, username, passwordHash string) {
	t.Helper()
	if _, err := DB.Exec(
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)`, username, passwordHash,
	); err != nil {
		t.Fatalf("insertUser %s: %v", username, err)
	}
}

// --- Register / Login ---

func TestRegisterHandler_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)

	body := `{"username":"nerv_user","password":"password123"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	newHandler().RegisterHandler(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterHandler_UsernameTooLong(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)

	long := strings.Repeat("a", maxUsernameLen+1)
	body := `{"username":"` + long + `","password":"password123"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	newHandler().RegisterHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "characters or fewer") {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
}

func TestRegisterHandler_DuplicateUsername(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "gendo", "$2a$10$placeholder")

	body := `{"username":"gendo","password":"password123"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	newHandler().RegisterHandler(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)

	body := `{"username":"kaworu","password":"whatever"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	newHandler().LoginHandler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Friends ---

func TestSendFriendRequest_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"shinji"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/request", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().SendFriendRequestHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var row int
	DB.QueryRow(`SELECT COUNT(*) FROM friend_requests WHERE from_username='rei' AND to_username='shinji'`).Scan(&row)
	if row != 1 {
		t.Fatal("expected friend request row in DB")
	}
}

func TestSendFriendRequest_CannotFriendSelf(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/request", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().SendFriendRequestHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSendFriendRequest_UserNotFound(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"kaworu"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/request", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().SendFriendRequestHandler(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAcceptFriendRequest_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")
	DB.Exec(`INSERT INTO friend_requests (from_username, to_username) VALUES ('rei', 'shinji')`)

	tok := makeToken("shinji", time.Now().Add(time.Hour))
	body := `{"from":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/accept", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().AcceptFriendRequestHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var reqCount, friendCount int
	DB.QueryRow(`SELECT COUNT(*) FROM friend_requests WHERE from_username='rei' AND to_username='shinji'`).Scan(&reqCount)
	DB.QueryRow(`SELECT COUNT(*) FROM friends WHERE (user_a='rei' AND user_b='shinji') OR (user_a='shinji' AND user_b='rei')`).Scan(&friendCount)
	if reqCount != 0 {
		t.Fatal("request should be deleted after accept")
	}
	if friendCount != 1 {
		t.Fatal("friendship row should exist after accept")
	}
}

func TestAcceptFriendRequest_NoPendingRequest(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")

	tok := makeToken("shinji", time.Now().Add(time.Hour))
	body := `{"from":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/accept", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().AcceptFriendRequestHandler(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetFriends_ReturnsList(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")
	insertUser(t, "asuka", "$2a$10$x")
	DB.Exec(`INSERT INTO friends (user_a, user_b) VALUES ('rei', 'shinji'), ('rei', 'asuka')`)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/friends", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().GetFriendsHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var friends []string
	json.NewDecoder(w.Body).Decode(&friends)
	if len(friends) != 2 {
		t.Fatalf("expected 2 friends, got %d: %v", len(friends), friends)
	}
}

func TestDeclineFriendRequest_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")
	DB.Exec(`INSERT INTO friend_requests (from_username, to_username) VALUES ('rei', 'shinji')`)

	tok := makeToken("shinji", time.Now().Add(time.Hour))
	body := `{"from":"rei"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/friends/decline", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().DeclineFriendRequestHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM friend_requests`).Scan(&count)
	if count != 0 {
		t.Fatal("request should be deleted after decline")
	}
}

func TestRemoveFriend_OK(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	insertUser(t, "rei", "$2a$10$x")
	insertUser(t, "shinji", "$2a$10$x")
	DB.Exec(`INSERT INTO friends (user_a, user_b) VALUES ('rei', 'shinji')`)

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := `{"target":"shinji"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/friends/remove", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().RemoveFriendHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM friends`).Scan(&count)
	if count != 0 {
		t.Fatal("friendship should be deleted")
	}
}

// --- CreateRoomHandler ---

func TestCreateRoomHandler_RoomNameTooLong(t *testing.T) {
	setupHandlerRedis(t)

	longName := strings.Repeat("a", maxRoomLen+1)
	body := strings.NewReader(`{"room":"` + longName + `"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/create-room", body)
	r = r.WithContext(context.WithValue(r.Context(), usernameKey, "rei"))
	newHandler().CreateRoomHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRoomHandler_DMPrefixBlocked(t *testing.T) {
	setupHandlerRedis(t)

	body := strings.NewReader(`{"room":"dm:rei:shinji"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/create-room", body)
	r = r.WithContext(context.WithValue(r.Context(), usernameKey, "rei"))
	newHandler().CreateRoomHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRoomHandler_OK(t *testing.T) {
	setupHandlerRedis(t)

	body := strings.NewReader(`{"room":"testroom","category":"chill"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/create-room", body)
	r = r.WithContext(context.WithValue(r.Context(), usernameKey, "rei"))
	newHandler().CreateRoomHandler(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

// --- UpdatePasswordHandler ---

func TestUpdatePasswordHandler_TooShort(t *testing.T) {
	setupHandlerRedis(t)
	setupTestDB(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("validpass"), bcrypt.MinCost)
	insertUser(t, "rei", string(hash))

	tok := makeToken("rei", time.Now().Add(time.Hour))
	body := strings.NewReader(`{"username":"rei","currentPassword":"validpass","newPassword":"short"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/update-password", body)
	r.Header.Set("Authorization", "Bearer "+tok)
	newHandler().UpdatePasswordHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// suppress unused import warning when POSTGRES_URL is not set
var _ = fmt.Sprintf
