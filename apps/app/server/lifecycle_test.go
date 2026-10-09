package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testOrigin = "http://test.local"

// startWSServer serves handleConnections over a real HTTP server so tests can
// open genuine WebSocket connections.
func startWSServer(t *testing.T) *httptest.Server {
	t.Helper()
	setupAppRedis(t)
	cleanSubscriptions(t)
	prev := allowedOrigins
	allowedOrigins = []string{testOrigin}
	t.Cleanup(func() { allowedOrigins = prev })

	a := newApp()
	// Guard count keeps the WaitGroup above zero while handlers call Add, so the
	// race detector doesn't flag a first Add concurrent with the cleanup's Wait.
	a.wg.Add(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleConnections(a, w, r)
	}))
	// Cleanups run LIFO: client connections close first, then srv.Close, then this
	// waits for all pumps, subscriptions and join announcements so nothing touches
	// the global RDB after the next test swaps it.
	t.Cleanup(func() {
		a.wg.Done()
		done := make(chan struct{})
		go func() { a.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("connection goroutines did not drain")
		}
	})
	t.Cleanup(srv.Close)
	return srv
}

func dialRoom(t *testing.T, srv *httptest.Server, room, user string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=" + room
	h := http.Header{}
	h.Set("Origin", testOrigin)
	h.Set("Authorization", "Bearer "+makeToken(user, time.Now().Add(time.Hour)))
	c, _, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		t.Fatalf("dial %s as %s: %v", room, user, err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func isOnline(room, user string) bool {
	onlineUsersLock.Lock()
	defer onlineUsersLock.Unlock()
	return onlineUsers[room][user]
}

func isSubscribed(room string) bool {
	subLock.Lock()
	defer subLock.Unlock()
	return subscriptions[room]
}

// waitSubscribed waits until Redis actually has a live subscriber for the room.
// The subscriptions flag flips before the subscriber is registered, and pub/sub
// is at-most-once, so publishing earlier would silently drop the message.
func waitSubscribed(t *testing.T, room string) {
	t.Helper()
	eventually(t, "redis subscriber live for "+room, func() bool {
		m, err := RDB.PubSubNumSub(context.Background(), "room:"+room).Result()
		return err == nil && m["room:"+room] >= 1
	})
}

// readUntil reads messages until one matches, or fails after a timeout.
func readUntil(t *testing.T, c *websocket.Conn, match func(Message) bool) Message {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var m Message
		if json.Unmarshal(data, &m) == nil && match(m) {
			return m
		}
	}
}

// A client must not be able to drop or rename another user's presence by
// putting someone else's name in the username_update payload.
func TestUsernameUpdate_IgnoresSpoofedIdentity(t *testing.T) {
	srv := startWSServer(t)
	room := "spoof_room"

	alice := dialRoom(t, srv, room, "alice")
	_ = dialRoom(t, srv, room, "bob")
	eventually(t, "alice and bob online", func() bool { return isOnline(room, "alice") && isOnline(room, "bob") })

	// alice claims to be bob while renaming herself to carol
	payload, _ := json.Marshal(Message{Type: "username_update", Username: "bob", Message: "carol"})
	if err := alice.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatal(err)
	}

	eventually(t, "alice renamed to carol", func() bool { return isOnline(room, "carol") })
	if !isOnline(room, "bob") {
		t.Fatal("bob's presence was removed by a spoofed username_update")
	}
	if isOnline(room, "alice") {
		t.Fatal("alice's old name should have been replaced")
	}

	// broadcast announces the real sender's old name, not the spoofed one
	m := readUntil(t, alice, func(m Message) bool { return strings.Contains(m.Message, "changed username") })
	if m.Username != "alice" {
		t.Fatalf("system message attributed to %q, want alice", m.Username)
	}
}

func TestUsernameUpdate_RejectsEmptyName(t *testing.T) {
	srv := startWSServer(t)
	room := "empty_name_room"

	alice := dialRoom(t, srv, room, "alice")
	eventually(t, "alice online", func() bool { return isOnline(room, "alice") })

	payload, _ := json.Marshal(Message{Type: "username_update", Message: "   "})
	_ = alice.WriteMessage(websocket.TextMessage, payload)
	time.Sleep(150 * time.Millisecond)

	if !isOnline(room, "alice") || isOnline(room, "") {
		t.Fatal("empty username_update must be ignored")
	}
}

func TestSubscription_StoppedWhenLastClientLeaves(t *testing.T) {
	srv := startWSServer(t)
	room := "leak_room"

	c := dialRoom(t, srv, room, "alice")
	eventually(t, "subscription started", func() bool { return isSubscribed(room) })

	c.Close()
	eventually(t, "subscription stopped after last client left", func() bool { return !isSubscribed(room) })

	roomSubsMu.Lock()
	_, stillRegistered := roomSubs[room]
	roomSubsMu.Unlock()
	if stillRegistered {
		t.Fatal("roomSubs entry leaked after the room emptied")
	}
	roomsLock.RLock()
	_, stillInRooms := rooms[room]
	roomsLock.RUnlock()
	if stillInRooms {
		t.Fatal("empty room entry leaked in rooms map")
	}
}

func TestSubscription_KeptWhileOtherClientsRemain(t *testing.T) {
	srv := startWSServer(t)
	room := "shared_room"

	a := dialRoom(t, srv, room, "alice")
	b := dialRoom(t, srv, room, "bob")
	eventually(t, "both online", func() bool { return isOnline(room, "alice") && isOnline(room, "bob") })

	waitSubscribed(t, room)
	a.Close()
	eventually(t, "alice gone", func() bool { return !isOnline(room, "alice") })
	if !isSubscribed(room) {
		t.Fatal("subscription must stay while bob is still connected")
	}

	// bob can still send and receive
	payload, _ := json.Marshal(Message{Type: "chat", Message: "still here"})
	_ = b.WriteMessage(websocket.TextMessage, payload)
	readUntil(t, b, func(m Message) bool { return m.Message == "still here" })
}

// After a room empties and its subscription is stopped, a new join must start a
// fresh subscription and receive messages again.
func TestSubscription_ResumesAfterRoomEmptied(t *testing.T) {
	srv := startWSServer(t)
	room := "resume_room"

	first := dialRoom(t, srv, room, "alice")
	eventually(t, "subscribed", func() bool { return isSubscribed(room) })
	first.Close()
	eventually(t, "unsubscribed", func() bool { return !isSubscribed(room) })

	c := dialRoom(t, srv, room, "bob")
	eventually(t, "resubscribed", func() bool { return isSubscribed(room) })
	waitSubscribed(t, room)

	payload, _ := json.Marshal(Message{Type: "chat", Message: "hello again"})
	_ = c.WriteMessage(websocket.TextMessage, payload)
	m := readUntil(t, c, func(m Message) bool { return m.Message == "hello again" })
	if m.Username != "bob" {
		t.Fatalf("message attributed to %q, want bob", m.Username)
	}
}

func TestStopSubscriptionIfEmpty_NoopWhenClientPresent(t *testing.T) {
	setupAppRedis(t)
	cleanSubscriptions(t)
	room := "noop_room"

	ctx, cancel := context.WithCancel(context.Background())
	subLock.Lock()
	subscriptions[room] = true
	subLock.Unlock()
	done := make(chan struct{})
	go func() { subscribeToRoom(ctx, room); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	time.Sleep(50 * time.Millisecond)

	roomsLock.Lock()
	rooms[room] = map[*client]struct{}{{}: {}}
	roomsLock.Unlock()
	t.Cleanup(func() {
		roomsLock.Lock()
		delete(rooms, room)
		roomsLock.Unlock()
	})

	stopSubscriptionIfEmpty(room)

	select {
	case <-done:
		t.Fatal("subscription was stopped while a client was still present")
	case <-time.After(150 * time.Millisecond):
	}
	if !isSubscribed(room) {
		t.Fatal("subscription flag cleared while a client was still present")
	}
}
