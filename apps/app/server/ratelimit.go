package main

import (
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter      *rate.Limiter
	lastSeenNano int64 // unix nano, accessed via atomic
}

var authLimiters sync.Map // map[string]*ipLimiter

func init() {
	go cleanupAuthLimiters()
}

// cleanupAuthLimiters evicts IPs not seen in the last 10 minutes.
func cleanupAuthLimiters() {
	for {
		time.Sleep(5 * time.Minute)
		cutoff := time.Now().Add(-10 * time.Minute).UnixNano()
		authLimiters.Range(func(k, v any) bool {
			if atomic.LoadInt64(&v.(*ipLimiter).lastSeenNano) < cutoff {
				authLimiters.Delete(k)
			}
			return true
		})
	}
}

func getAuthLimiter(ip string) *rate.Limiter {
	// Fast path: already exists.
	if v, ok := authLimiters.Load(ip); ok {
		il := v.(*ipLimiter)
		atomic.StoreInt64(&il.lastSeenNano, time.Now().UnixNano())
		return il.limiter
	}
	// Slow path: first request from this IP.
	// LoadOrStore guarantees only one limiter wins even under concurrent misses.
	// 10 requests burst, refills at 1 request/second.
	candidate := &ipLimiter{
		limiter:      rate.NewLimiter(rate.Every(time.Second), 10),
		lastSeenNano: time.Now().UnixNano(),
	}
	actual, _ := authLimiters.LoadOrStore(ip, candidate)
	il := actual.(*ipLimiter)
	atomic.StoreInt64(&il.lastSeenNano, time.Now().UnixNano())
	return il.limiter
}

// RateLimitAuth limits requests per IP. Intended for auth endpoints.
func RateLimitAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !getAuthLimiter(ip).Allow() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

const (
	// connBurst / connRefill bound how fast one websocket client may send.
	// Typing indicators count too, so the budget is generous for real use.
	connBurst  = 20
	connRefill = 10 // messages per second, sustained

	// connMaxDrops is how many over-limit messages we tolerate on one
	// connection before closing it.
	connMaxDrops = 100
)

// connLimiter is a per-connection token bucket for inbound websocket messages.
// It is owned by a single readPump goroutine, so it needs no locking.
type connLimiter struct {
	l     *rate.Limiter
	drops int
}

func newConnLimiter() *connLimiter {
	return &connLimiter{l: rate.NewLimiter(rate.Limit(connRefill), connBurst)}
}

// allow reports whether the message may be processed. kick is true once the
// connection has been over the limit too many times and should be closed.
func (c *connLimiter) allow() (ok, kick bool) {
	if c.l.Allow() {
		return true, false
	}
	c.drops++
	return false, c.drops >= connMaxDrops
}
