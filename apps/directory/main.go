package main

import (
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
	http.HandleFunc("/health", HealthHandler)
	http.HandleFunc("/heartbeat", st.HeartbeatHandler)
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
