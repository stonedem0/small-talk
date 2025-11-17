package main

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	st := NewState()
	RedisInit()
	key := os.Getenv("INTERNAL_API_KEY")
	http.HandleFunc("/health", withInternalKey(key, HealthHandler))
	http.HandleFunc("/heartbeat", withInternalKey(key, st.HeartbeatHandler))
	http.HandleFunc("/join", withCORSAndAuth(true, st.JoinHandler))
	port := os.Getenv("DIRECTORY_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("Directory service starting on %s", addr)
	// background: mark apps/owners stale over time
	go func() {
		t := time.NewTicker(2 * time.Second)
		for range t.C {
			st.MarkStale()
		}
	}()
	http.ListenAndServe(addr, nil)
}

// withInternalKey authorizes requests using a shared key.
// Accepts either:
// - Header: X-Internal-Key: <key>
// - Authorization: Bearer <key>
// If INTERNAL_API_KEY is empty, auth is bypassed (dev).
func withInternalKey(key string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if key == "" {
			next(w, r)
			return
		}
		provided := r.Header.Get("X-Internal-Key")
		if provided == "" {
			const prefix = "Bearer "
			if auth := r.Header.Get("Authorization"); len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
				provided = auth[len(prefix):]
			}
		}
		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		next(w, r)
	}
}
