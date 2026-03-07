package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupAppRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: s.Addr()})
	return s
}

// cleanSubscriptions resets the global subscriptions map between tests.
func cleanSubscriptions(t *testing.T) {
	t.Helper()
	subLock.Lock()
	subscriptions = make(map[string]bool)
	subLock.Unlock()
}

// TestSubscriptionsCleanedUpOnContextCancel verifies that when the context is
// canceled, subscribeToRoom removes the room from the subscriptions map so a
// future user joining can re-trigger the subscription.
func TestSubscriptionsCleanedUpOnContextCancel(t *testing.T) {
	setupAppRedis(t)
	cleanSubscriptions(t)

	ctx, cancel := context.WithCancel(context.Background())

	subLock.Lock()
	subscriptions["gaming"] = true
	subLock.Unlock()

	done := make(chan struct{})
	go func() {
		subscribeToRoom(ctx, "gaming")
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscribeToRoom did not exit after context cancel")
	}

	subLock.Lock()
	_, exists := subscriptions["gaming"]
	subLock.Unlock()

	if exists {
		t.Fatal("expected subscriptions[gaming] to be deleted after goroutine exit")
	}
}

// TestSubscriptionsCleanedUpOnPubSubClose verifies that if the pubsub
// subscription is closed externally (e.g. during graceful shutdown), the room
// is removed from the subscriptions map so a new subscription can be started.
func TestSubscriptionsCleanedUpOnPubSubClose(t *testing.T) {
	setupAppRedis(t)
	cleanSubscriptions(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subLock.Lock()
	subscriptions["gaming"] = true
	subLock.Unlock()

	done := make(chan struct{})
	go func() {
		subscribeToRoom(ctx, "gaming")
		close(done)
	}()

	// give the goroutine time to register in roomSubs
	time.Sleep(50 * time.Millisecond)

	// close the pubsub directly, as gracefulShutdown does
	roomSubsMu.Lock()
	if ps, ok := roomSubs["gaming"]; ok {
		ps.Close()
	}
	roomSubsMu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("subscribeToRoom did not exit after pubsub closed")
	}

	subLock.Lock()
	_, exists := subscriptions["gaming"]
	subLock.Unlock()

	if exists {
		t.Fatal("expected subscriptions[gaming] to be deleted after pubsub close")
	}
}

// TestResubscribeAfterCleanup verifies that once the subscriptions map is
// cleaned up, the next join correctly starts a new subscription goroutine.
func TestResubscribeAfterCleanup(t *testing.T) {
	setupAppRedis(t)
	cleanSubscriptions(t)

	ctx, cancel := context.WithCancel(context.Background())

	subLock.Lock()
	subscriptions["gaming"] = true
	subLock.Unlock()

	done := make(chan struct{})
	go func() {
		subscribeToRoom(ctx, "gaming")
		close(done)
	}()

	cancel()
	<-done

	// subscriptions["gaming"] is now gone — a new join should re-subscribe
	subLock.Lock()
	alreadySubscribed := subscriptions["gaming"]
	if !alreadySubscribed {
		subscriptions["gaming"] = true
	}
	subLock.Unlock()

	if alreadySubscribed {
		t.Fatal("expected subscriptions[gaming] to be cleared, allowing re-subscription")
	}
}
