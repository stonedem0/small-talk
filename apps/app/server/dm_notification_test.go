package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startRoomSubscriber runs the room subscription the way an app node does.
func startRoomSubscriber(t *testing.T, room string) {
	t.Helper()
	cleanSubscriptions(t)
	subLock.Lock()
	subscriptions[room] = true
	subLock.Unlock()
	c, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { subscribeToRoom(c, room); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	waitSubscribed(t, room)
}

func publishRaw(t *testing.T, room string, msg Message) {
	t.Helper()
	b, _ := json.Marshal(msg)
	if err := RDB.Publish(ctx, "room:"+room, string(b)).Err(); err != nil {
		t.Fatal(err)
	}
}

// A normal DM (clients send chat messages with no type) notifies the recipient.
func TestDMNotification_SentForChatMessage(t *testing.T) {
	srv := startSSEServer(t)
	room := "dm:alice:bob"
	startRoomSubscriber(t, room)
	bob := openSSE(t, srv, "bob")

	publishRaw(t, room, Message{Username: "alice", Message: "hi bob"})

	select {
	case got := <-bob.events:
		if !strings.Contains(got, `"type":"dm"`) || !strings.Contains(got, `"from":"alice"`) {
			t.Fatalf("unexpected notification %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bob was not notified of a DM")
	}
}

// Things that are not messages must not look like a DM: changing your status
// while the DM window is open publishes a status_update into the DM room.
func TestDMNotification_NotSentForNonMessageEvents(t *testing.T) {
	for _, typ := range []string{"status_update", "typing", "stop_typing", "system"} {
		t.Run(typ, func(t *testing.T) {
			srv := startSSEServer(t)
			room := "dm:alice:bob"
			startRoomSubscriber(t, room)
			bob := openSSE(t, srv, "bob")

			publishRaw(t, room, Message{Type: typ, Username: "alice", Message: "brb"})

			bob.expectNothing(t)
		})
	}
}

// End to end through the real status broadcast: alice is in a DM room with bob
// and changes her status.
func TestDMNotification_StatusChangeInDMRoomDoesNotNotifyPartner(t *testing.T) {
	srv := startSSEServer(t)
	room := "dm:alice:bob"
	startRoomSubscriber(t, room)
	seedPresence(t, room, "alice") // alice has the DM window open
	bob := openSSE(t, srv, "bob")

	broadcastStatusUpdate(context.Background(), "alice", "gone fishing")

	bob.expectNothing(t)
}

// Live-only events must not be stored in history, where the client would replay
// them as chat lines.
func TestSubscriber_StoresOnlyChatAndSystemInHistory(t *testing.T) {
	setupAppRedis(t)
	room := "gaming"
	startRoomSubscriber(t, room)
	// a listener so we know when each message has been processed
	for _, m := range []Message{
		{Type: "typing", Username: "alice"},
		{Type: "status_update", Username: "alice", Message: "brb"},
		{Type: "stop_typing", Username: "alice"},
		{Username: "alice", Message: "hello"},
		{Type: "system", Username: "alice", Message: "left the room"},
	} {
		publishRaw(t, room, m)
	}
	eventually(t, "chat and system stored", func() bool {
		n, _ := RDB.LLen(ctx, "chat_history:"+room).Result()
		return n >= 2
	})
	time.Sleep(200 * time.Millisecond) // give any wrongly stored event time to land

	items, _ := RDB.LRange(ctx, "chat_history:"+room, 0, -1).Result()
	if len(items) != 2 {
		t.Fatalf("history has %d entries %v, want only the chat and system lines", len(items), items)
	}
}

// Status updates already sitting in history (stored before this fix) are not
// returned to clients.
func TestGetChatHistoryHandler_SkipsStoredStatusUpdates(t *testing.T) {
	setupHandlerRedis(t)
	for _, m := range []Message{
		{Username: "rei", Message: "first", Room: "gaming"},
		{Type: "status_update", Username: "rei", Message: "brb"},
		{Type: "system", Username: "rei", Message: "left the room", Room: "gaming"},
		{Type: "typing", Username: "rei"},
	} {
		b, _ := json.Marshal(m)
		RDB.LPush(ctx, "chat_history:gaming", b)
	}

	w := httptest.NewRecorder()
	newHandler().GetChatHistoryHandler(w, httptest.NewRequest(http.MethodGet, "/history?room=gaming", nil))

	var history []Message
	_ = json.NewDecoder(w.Body).Decode(&history)
	if len(history) != 2 || history[0].Message != "first" || history[1].Type != "system" {
		t.Fatalf("got %+v, want the chat line then the system line", history)
	}
}
