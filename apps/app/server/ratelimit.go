package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	authLimiters   sync.Map // map[string]*ipLimiter
	authLimitersMu sync.Mutex
)

func init() {
	go cleanupAuthLimiters()
}

// cleanupAuthLimiters evicts IPs not seen in the last 10 minutes.
func cleanupAuthLimiters() {
	for {
		time.Sleep(5 * time.Minute)
		authLimiters.Range(func(k, v any) bool {
			if time.Since(v.(*ipLimiter).lastSeen) > 10*time.Minute {
				authLimiters.Delete(k)
			}
			return true
		})
	}
}

func getAuthLimiter(ip string) *rate.Limiter {
	v, ok := authLimiters.Load(ip)
	if !ok {
		// 10 requests burst, refills at 1 request/second
		l := &ipLimiter{
			limiter:  rate.NewLimiter(rate.Every(time.Second), 10),
			lastSeen: time.Now(),
		}
		authLimiters.Store(ip, l)
		return l.limiter
	}
	il := v.(*ipLimiter)
	il.lastSeen = time.Now()
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
