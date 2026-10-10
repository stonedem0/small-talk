package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

// seedPresence writes presence the way another node would: straight into Redis,
// with no local state on this node.
func seedPresence(t *testing.T, room string, users ...string) {
	t.Helper()
	members := make([]interface{}, len(users))
	for i, u := range users {
		members[i] = u
	}
	if err := RDB.SAdd(ctx, presenceKey(room), members...).Err(); err != nil {
		t.Fatal(err)
	}
	RDB.Expire(ctx, presenceKey(room), presenceTTL)
	RDB.SAdd(ctx, presenceRoomsKey, room)
}

func setLocalUsers(t *testing.T, room string, users ...string) {
	t.Helper()
	onlineUsersLock.Lock()
	m := map[string]bool{}
	for _, u := range users {
		m[u] = true
	}
	onlineUsers[room] = m
	onlineUsersLock.Unlock()
	t.Cleanup(func() {
		onlineUsersLock.Lock()
		delete(onlineUsers, room)
		onlineUsersLock.Unlock()
	})
}

func TestSyncPresence_MirrorsLocalStateIntoRedis(t *testing.T) {
	mr := setupAppRedis(t)
	setLocalUsers(t, "anime", "doom", "diva")

	syncPresence("anime")

	got, _ := RDB.SMembers(ctx, presenceKey("anime")).Result()
	if len(got) != 2 {
		t.Fatalf("members %v, want doom and diva", got)
	}
	if ttl := mr.TTL(presenceKey("anime")); ttl < 25*time.Second || ttl > presenceTTL {
		t.Fatalf("ttl %v, want about %v", ttl, presenceTTL)
	}
	if ok, _ := RDB.SIsMember(ctx, presenceRoomsKey, "anime").Result(); !ok {
		t.Fatal("room missing from the presence index")
	}
}

func TestSyncPresence_RemovesRoomWhenEmpty(t *testing.T) {
	setupAppRedis(t)
	seedPresence(t, "anime", "doom")

	syncPresence("anime") // no local users

	if n, _ := RDB.Exists(ctx, presenceKey("anime")).Result(); n != 0 {
		t.Fatal("presence key should be deleted when the room is empty")
	}
	if ok, _ := RDB.SIsMember(ctx, presenceRoomsKey, "anime").Result(); ok {
		t.Fatal("empty room should leave the presence index")
	}
}

// The periodic refresh must never delete: a node that used to host a room and has
// no users left would otherwise wipe the new host's presence every tick.
func TestSyncAllPresence_NeverDeletesOtherNodesRooms(t *testing.T) {
	setupAppRedis(t)
	seedPresence(t, "anime", "doom", "diva") // hosted elsewhere
	setLocalUsers(t, "music", "rei")
	onlineUsersLock.Lock()
	onlineUsers["stale"] = map[string]bool{} // room this node once hosted
	onlineUsersLock.Unlock()
	t.Cleanup(func() { onlineUsersLock.Lock(); delete(onlineUsers, "stale"); onlineUsersLock.Unlock() })

	syncAllPresence()

	if n, _ := RDB.SCard(ctx, presenceKey("anime")).Result(); n != 2 {
		t.Fatalf("another node's room was modified, members=%d", n)
	}
	if n, _ := RDB.SCard(ctx, presenceKey("music")).Result(); n != 1 {
		t.Fatalf("local active room not written, members=%d", n)
	}
}

func TestSyncAllPresence_ExtendsExpiry(t *testing.T) {
	mr := setupAppRedis(t)
	setLocalUsers(t, "music", "rei")
	syncPresence("music")
	mr.FastForward(20 * time.Second)

	syncAllPresence()

	if ttl := mr.TTL(presenceKey("music")); ttl < 25*time.Second {
		t.Fatalf("expiry not refreshed, ttl %v", ttl)
	}
}

// If the hosting node dies, its presence expires on its own.
func TestPresence_ExpiresWhenHostingNodeDies(t *testing.T) {
	mr := setupAppRedis(t)
	seedPresence(t, "anime", "doom")

	mr.FastForward(presenceTTL + time.Second)
	got, err := readPresence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expired presence still visible: %v", got)
	}
	if ok, _ := RDB.SIsMember(ctx, presenceRoomsKey, "anime").Result(); ok {
		t.Fatal("expired room should be dropped from the index")
	}
}

func TestReadPresence_CombinesRoomsFromDifferentNodes(t *testing.T) {
	setupAppRedis(t)
	seedPresence(t, "anime", "diva", "doom") // node B
	seedPresence(t, "music", "rei")          // node C

	got, err := readPresence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"anime": {"diva", "doom"}, "music": {"rei"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Concurrent joins and leaves must leave Redis equal to the final local state.
func TestSyncPresence_ConcurrentUpdatesConverge(t *testing.T) {
	setupAppRedis(t)
	setLocalUsers(t, "busy")

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u := string(rune('a' + i%26))
			onlineUsersLock.Lock()
			if i%3 == 0 {
				delete(onlineUsers["busy"], u)
			} else {
				onlineUsers["busy"][u] = true
			}
			onlineUsersLock.Unlock()
			syncPresence("busy")
		}(i)
	}
	wg.Wait()
	syncPresence("busy")

	onlineUsersLock.Lock()
	want := len(onlineUsers["busy"])
	onlineUsersLock.Unlock()
	if n, _ := RDB.SCard(ctx, presenceKey("busy")).Result(); int(n) != want {
		t.Fatalf("redis has %d members, local state has %d", n, want)
	}
}

// --- handlers read what any node wrote ---

func TestRoomUsernamesHandler_ShowsRoomsHostedOnOtherNodes(t *testing.T) {
	setupAppRedis(t)
	seedPresence(t, "anime", "diva", "doom")

	w := httptest.NewRecorder()
	newHandler().GetRoomUsernamesHandler(w, httptest.NewRequest(http.MethodGet, "/room-usernames", nil))

	var got map[string][]string
	_ = json.NewDecoder(w.Body).Decode(&got)
	if !reflect.DeepEqual(got["anime"], []string{"diva", "doom"}) {
		t.Fatalf("got %v", got)
	}
}

func TestPresenceHandlers_503WhenRedisDown(t *testing.T) {
	mr := setupAppRedis(t)
	mr.Close()
	for _, path := range []string{"/room-usernames", "/online-users", "/online-users?room=anime"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/room-usernames" {
			newHandler().GetRoomUsernamesHandler(w, r)
		} else {
			newHandler().GetOnlineUsersHandler(w, r)
		}
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503, got %d", path, w.Code)
		}
	}
}

// --- status broadcast reaches rooms on other nodes ---

func TestBroadcastStatusUpdate_ReachesAllRoomsTheUserIsIn(t *testing.T) {
	setupAppRedis(t)
	seedPresence(t, "anime", "diva", "doom") // hosted on another node
	seedPresence(t, "music", "doom")         // another node again
	seedPresence(t, "gaming", "rei")         // doom is not here

	sub := RDB.Subscribe(ctx, "room:anime", "room:music", "room:gaming")
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	ch := sub.Channel()

	broadcastStatusUpdate(context.Background(), "doom", "brb")

	got := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case m := <-ch:
			var msg Message
			_ = json.Unmarshal([]byte(m.Payload), &msg)
			if msg.Type != "status_update" || msg.Username != "doom" || msg.Message != "brb" {
				t.Fatalf("unexpected message %+v", msg)
			}
			got[m.Channel] = true
		case <-deadline:
			t.Fatalf("only reached %v", got)
		}
	}
	if !got["room:anime"] || !got["room:music"] || got["room:gaming"] {
		t.Fatalf("reached %v, want anime and music only", got)
	}
	select {
	case m := <-ch:
		t.Fatalf("unexpected extra message on %s", m.Channel)
	case <-time.After(200 * time.Millisecond):
	}
}

// --- end to end over real WebSockets ---

func TestPresence_FollowsJoinRenameAndLeave(t *testing.T) {
	srv := startWSServer(t)
	room := "presence_room"

	alice := dialRoom(t, srv, room, "alice")
	bob := dialRoom(t, srv, room, "bob")
	members := func() []string { u, _ := RDB.SMembers(ctx, presenceKey(room)).Result(); return u }
	eventually(t, "both in redis", func() bool { return len(members()) == 2 })

	payload, _ := json.Marshal(Message{Type: "username_update", Message: "carol"})
	_ = alice.WriteMessage(1, payload)
	eventually(t, "rename mirrored", func() bool {
		ok, _ := RDB.SIsMember(ctx, presenceKey(room), "carol").Result()
		gone, _ := RDB.SIsMember(ctx, presenceKey(room), "alice").Result()
		return ok && !gone
	})

	bob.Close()
	eventually(t, "leave mirrored", func() bool { return len(members()) == 1 })

	alice.Close()
	eventually(t, "room removed once empty", func() bool {
		n, _ := RDB.Exists(ctx, presenceKey(room)).Result()
		ok, _ := RDB.SIsMember(ctx, presenceRoomsKey, room).Result()
		return n == 0 && !ok
	})
}
