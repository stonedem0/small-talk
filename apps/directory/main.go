package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	st := NewState()
	RedisInit()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	http.HandleFunc("/health", HealthHandler)
	http.HandleFunc("/heartbeat", st.HeartbeatHandler)
	http.HandleFunc("/join", st.JoinHandler)
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
