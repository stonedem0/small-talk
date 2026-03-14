package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// deleteAuthLimiter removes an IP entry so tests don't bleed into each other.
func deleteAuthLimiter(ip string) { authLimiters.Delete(ip) }

func TestRateLimitAuth_ExhaustedBurst(t *testing.T) {
	ip := "203.0.113.1" // TEST-NET, won't collide with real traffic
	defer deleteAuthLimiter(ip)

	handler := RateLimitAuth(okHandler)

	// First 10 requests should pass (burst = 10).
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.RemoteAddr = ip + ":12345"
		handler(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 11th request must be rate-limited.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = ip + ":12345"
	handler(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", w.Code)
	}
}

func TestRateLimitAuth_IndependentBuckets(t *testing.T) {
	ipA := "203.0.113.2"
	ipB := "203.0.113.3"
	defer deleteAuthLimiter(ipA)
	defer deleteAuthLimiter(ipB)

	handler := RateLimitAuth(okHandler)

	// Exhaust ipA's burst.
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/login", nil)
		r.RemoteAddr = ipA + ":1111"
		handler(w, r)
	}
	wA := httptest.NewRecorder()
	rA := httptest.NewRequest(http.MethodPost, "/login", nil)
	rA.RemoteAddr = ipA + ":1111"
	handler(wA, rA)
	if wA.Code != http.StatusTooManyRequests {
		t.Fatalf("ipA: expected 429 after burst, got %d", wA.Code)
	}

	// ipB should still have its own full bucket.
	wB := httptest.NewRecorder()
	rB := httptest.NewRequest(http.MethodPost, "/login", nil)
	rB.RemoteAddr = ipB + ":2222"
	handler(wB, rB)
	if wB.Code != http.StatusOK {
		t.Fatalf("ipB: expected 200 (independent bucket), got %d", wB.Code)
	}
}
