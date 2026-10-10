package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func startSSEServer(t *testing.T) *httptest.Server {
	t.Helper()
	setupAppRedis(t)
	h := NewHandler(RDB, testSecret, testSecret)
	srv := httptest.NewServer(http.HandlerFunc(h.SSEHandler))
	t.Cleanup(srv.Close)
	return srv
}

// sseStream is an open /events connection; lines of "data: ..." arrive on events.
type sseStream struct {
	events chan string
	resp   *http.Response
}

func openSSE(t *testing.T, srv *httptest.Server, user string) *sseStream {
	t.Helper()
	tok := makeToken(user, time.Now().Add(time.Hour))
	resp, err := http.Get(srv.URL + "?token=" + tok)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sse status %d", resp.StatusCode)
	}
	s := &sseStream{events: make(chan string, 16), resp: resp}
	t.Cleanup(func() { resp.Body.Close() })
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data: ") {
				s.events <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return s
}

func (s *sseStream) expect(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-s.events:
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func (s *sseStream) expectNothing(t *testing.T) {
	t.Helper()
	select {
	case got := <-s.events:
		t.Fatalf("unexpected event %q", got)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSSE_ReceivesNotification(t *testing.T) {
	srv := startSSEServer(t)
	bob := openSSE(t, srv, "bob")

	pushNotification("bob", `{"type":"dm","from":"alice"}`)
	bob.expect(t, `{"type":"dm","from":"alice"}`)
}

// The notification may come from a different node than the one holding the SSE
// connection: here a separate Redis client plays the other node.
func TestSSE_ReceivesNotificationPublishedByAnotherNode(t *testing.T) {
	srv := startSSEServer(t)
	bob := openSSE(t, srv, "bob")

	otherNode := redis.NewClient(&redis.Options{Addr: RDB.Options().Addr})
	defer otherNode.Close()
	if err := otherNode.Publish(ctx, notifyChannel("bob"), `{"type":"friend_request"}`).Err(); err != nil {
		t.Fatal(err)
	}
	bob.expect(t, `{"type":"friend_request"}`)
}

func TestSSE_OnlyTheTargetedUserIsNotified(t *testing.T) {
	srv := startSSEServer(t)
	bob := openSSE(t, srv, "bob")
	carol := openSSE(t, srv, "carol")

	pushNotification("bob", "for-bob")
	bob.expect(t, "for-bob")
	carol.expectNothing(t)
}

func TestSSE_AllOfAUsersConnectionsAreNotified(t *testing.T) {
	srv := startSSEServer(t)
	tab1 := openSSE(t, srv, "bob")
	tab2 := openSSE(t, srv, "bob")

	pushNotification("bob", "hello")
	tab1.expect(t, "hello")
	tab2.expect(t, "hello")
}

// A notification published right after the stream opens must not be lost: the
// handler only opens the stream once Redis has confirmed the subscription.
func TestSSE_NoNotificationLostRightAfterConnect(t *testing.T) {
	srv := startSSEServer(t)
	for i := 0; i < 20; i++ {
		s := openSSE(t, srv, "bob")
		pushNotification("bob", "immediate")
		s.expect(t, "immediate")
	}
}

func TestSSE_RequiresToken(t *testing.T) {
	srv := startSSEServer(t)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSSE_StopsListeningWhenClientDisconnects(t *testing.T) {
	srv := startSSEServer(t)
	s := openSSE(t, srv, "bob")
	s.resp.Body.Close()

	eventually(t, "subscription released", func() bool {
		m, err := RDB.PubSubNumSub(ctx, notifyChannel("bob")).Result()
		return err == nil && m[notifyChannel("bob")] == 0
	})
}
