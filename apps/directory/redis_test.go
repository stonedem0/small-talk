package main

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: s.Addr()})
	return s
}

func TestTryClaim(t *testing.T) {
	setupRedis(t)

	claimed, err := TryClaim("gaming", "app-1")
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected app-1 to claim the room")
	}

	// second claim by a different node should fail
	claimed, err = TryClaim("gaming", "app-2")
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("expected app-2 to be rejected, room already owned")
	}
}

func TestRefreshLease_Owner(t *testing.T) {
	s := setupRedis(t)

	_, _ = TryClaim("gaming", "app-1")
	s.FastForward(30 * time.Second)

	if err := RefreshLease("gaming", "app-1"); err != nil {
		t.Fatalf("RefreshLease failed: %v", err)
	}

	// after refresh, TTL should be reset to leaseTTL
	ttl := s.TTL(key("gaming"))
	if ttl < 55*time.Second {
		t.Fatalf("expected TTL ~60s after refresh, got %v", ttl)
	}
}

func TestRefreshLease_NonOwner(t *testing.T) {
	s := setupRedis(t)

	_, _ = TryClaim("gaming", "app-1")
	ttlBefore := s.TTL(key("gaming"))

	// app-2 should not be able to refresh a lease it doesn't own
	if err := RefreshLease("gaming", "app-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ttlAfter := s.TTL(key("gaming"))
	if ttlAfter > ttlBefore {
		t.Fatal("non-owner RefreshLease should not extend the TTL")
	}
}

func TestRelease_Owner(t *testing.T) {
	setupRedis(t)

	_, _ = TryClaim("gaming", "app-1")

	if err := Release("gaming", "app-1"); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	owner, _ := Owner("gaming")
	if owner != "" {
		t.Fatalf("expected room to be released, got owner %q", owner)
	}
}

func TestRelease_NonOwner(t *testing.T) {
	setupRedis(t)

	_, _ = TryClaim("gaming", "app-1")

	// app-2 must not be able to release app-1's lease
	if err := Release("gaming", "app-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	owner, _ := Owner("gaming")
	if owner != "app-1" {
		t.Fatalf("expected app-1 to still own the room, got %q", owner)
	}
}
