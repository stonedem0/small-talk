package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	st := NewState()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	http.HandleFunc("/health", HealthHandler)
	http.HandleFunc("/heartbeat", st.HeartbeatHandler)
	http.HandleFunc("/join", JoinHandler)
	port := os.Getenv("DIRECTORY_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("Directory service starting on %s", addr)
	http.ListenAndServe(addr, nil)
}
