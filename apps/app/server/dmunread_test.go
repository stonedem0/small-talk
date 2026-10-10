package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func hashGet(t *testing.T, user, partner string) string {
	t.Helper()
	v, _ := RDB.HGet(ctx, dmUnreadKey(user), partner).Result()
	return v
}

func TestDMPartner(t *testing.T) {
	cases := []struct{ room, user, want string }{
		{"dm:alice:bob", "alice", "bob"},
		{"dm:alice:bob", "bob", "alice"},
		{"dm:alice:bob", "carol", ""}, // not a participant
		{"gaming", "alice", ""},       // not a DM room
		{"dm:broken", "alice", ""},
	}
	for _, c := range cases {
		if got := dmPartner(c.room, c.user); got != c.want {
			t.Errorf("dmPartner(%q, %q) = %q, want %q", c.room, c.user, got, c.want)
		}
	}
}

func TestRecordUnreadDM_CountsAndNotifiesWhenRecipientIsAway(t *testing.T) {
	srv := startSSEServer(t)
	bob := openSSE(t, srv, "bob")

	recordUnreadDM("dm:alice:bob", "alice")
	recordUnreadDM("dm:alice:bob", "alice")

	for want := 1; want <= 2; want++ {
		var got struct {
			Type   string `json:"type"`
			From   string `json:"from"`
			Room   string `json:"room"`
			Unread int    `json:"unread"`
		}
		select {
		case ev := <-bob.events:
			if err := json.Unmarshal([]byte(ev), &got); err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no notification %d", want)
		}
		if got.Type != "dm" || got.From != "alice" || got.Room != "dm:alice:bob" || got.Unread != want {
			t.Fatalf("notification %d = %+v", want, got)
		}
	}
	if v := hashGet(t, "bob", "alice"); v != "2" {
		t.Fatalf("stored count %q, want 2", v)
	}
	if v := hashGet(t, "alice", "bob"); v != "" {
		t.Fatalf("the sender must not get an unread count, got %q", v)
	}
}

// Someone looking at the conversation is reading it: no count, no notification.
func TestRecordUnreadDM_SkippedWhenRecipientIsInTheRoom(t *testing.T) {
	srv := startSSEServer(t)
	setLocalUsers(t, "dm:alice:bob", "alice", "bob")
	bob := openSSE(t, srv, "bob")

	recordUnreadDM("dm:alice:bob", "alice")

	bob.expectNothing(t)
	if v := hashGet(t, "bob", "alice"); v != "" {
		t.Fatalf("counted a message bob is looking at: %q", v)
	}
}

func TestRecordUnreadDM_IgnoresRoomsThatAreNotDMs(t *testing.T) {
	setupAppRedis(t)
	recordUnreadDM("gaming", "alice")
	if n, _ := RDB.Exists(ctx, dmUnreadKey("alice"), dmUnreadKey("gaming")).Result(); n != 0 {
		t.Fatal("a non-DM room created unread state")
	}
}

func TestMarkDMRead_ClearsTheCountAndTellsOtherSessions(t *testing.T) {
	srv := startSSEServer(t)
	RDB.HSet(ctx, dmUnreadKey("bob"), "alice", 3, "carol", 1)
	otherTab := openSSE(t, srv, "bob")

	markDMRead("bob", "alice")

	otherTab.expect(t, `{"from":"alice","type":"dm_read"}`)
	if v := hashGet(t, "bob", "alice"); v != "" {
		t.Fatalf("count not cleared: %q", v)
	}
	if v := hashGet(t, "bob", "carol"); v != "1" {
		t.Fatalf("another conversation was touched: %q", v)
	}
}

func TestMarkDMRead_SilentWhenThereWasNothingUnread(t *testing.T) {
	srv := startSSEServer(t)
	bob := openSSE(t, srv, "bob")

	markDMRead("bob", "alice")

	bob.expectNothing(t)
}

func TestGetDMUnreadHandler(t *testing.T) {
	setupAppRedis(t)
	RDB.HSet(ctx, dmUnreadKey("bob"), "alice", 2, "carol", 1, "dave", 0)
	h := newHandler()
	get := func(user string) (int, map[string]int) {
		r := httptest.NewRequest(http.MethodGet, "/dms/unread", nil)
		if user != "" {
			r.Header.Set("Authorization", "Bearer "+makeToken(user, time.Now().Add(time.Hour)))
		}
		w := httptest.NewRecorder()
		h.GetDMUnreadHandler(w, r)
		var m map[string]int
		_ = json.NewDecoder(w.Body).Decode(&m)
		return w.Code, m
	}

	code, got := get("bob")
	if code != http.StatusOK || len(got) != 2 || got["alice"] != 2 || got["carol"] != 1 {
		t.Fatalf("bob got %d %v, want alice=2 carol=1 (zero counts omitted)", code, got)
	}
	if code, got := get("alice"); code != http.StatusOK || len(got) != 0 {
		t.Fatalf("alice must only see her own counts, got %d %v", code, got)
	}
	if code, _ := get(""); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", code)
	}
}

// --- end to end over real connections ---

func openSSEOnSameRedis(t *testing.T, user string) *sseStream {
	t.Helper()
	h := NewHandler(RDB, testSecret, testSecret)
	srv := httptest.NewServer(http.HandlerFunc(h.SSEHandler))
	t.Cleanup(srv.Close)
	return openSSE(t, srv, user)
}

func sendChat(t *testing.T, c *websocket.Conn, text string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"username": "x", "message": text})
	if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatal(err)
	}
}

// Opening the conversation reads it, and the user's other sessions find out.
func TestOpeningADMClearsItsUnreadCount(t *testing.T) {
	srv := startWSServer(t)
	RDB.HSet(ctx, dmUnreadKey("bob"), "alice", 2)
	otherTab := openSSEOnSameRedis(t, "bob")

	dialRoom(t, srv, "dm:alice:bob", "bob")

	otherTab.expect(t, `{"from":"alice","type":"dm_read"}`)
	if v := hashGet(t, "bob", "alice"); v != "" {
		t.Fatalf("count not cleared on open: %q", v)
	}
}

// A message sent while the recipient is in the DM is read; one sent after they
// leave is unread.
func TestMessagesAreOnlyUnreadWhileTheRecipientIsAway(t *testing.T) {
	srv := startWSServer(t)
	room := "dm:alice:bob"
	alice := dialRoom(t, srv, room, "alice")
	bob := dialRoom(t, srv, room, "bob")
	eventually(t, "both present", func() bool { return isOnline(room, "alice") && isOnline(room, "bob") })
	waitSubscribed(t, room)
	bobSSE := openSSEOnSameRedis(t, "bob")

	sendChat(t, alice, "hello while you are here")
	readUntil(t, bob, func(m Message) bool { return m.Message == "hello while you are here" })
	bobSSE.expectNothing(t)
	if v := hashGet(t, "bob", "alice"); v != "" {
		t.Fatalf("message to a present recipient counted as unread: %q", v)
	}

	bob.Close()
	eventually(t, "bob left", func() bool { return !isOnline(room, "bob") })
	sendChat(t, alice, "hello after you left")

	eventually(t, "unread counted", func() bool { return hashGet(t, "bob", "alice") == "1" })
	got := <-bobSSE.events
	if !strings.Contains(got, `"type":"dm"`) || !strings.Contains(got, `"unread":1`) {
		t.Fatalf("unexpected notification %q", got)
	}
}
