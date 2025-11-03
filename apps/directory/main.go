package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/join", joinHandler)
	port := os.Getenv("DIRECTORY_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("Directory service starting on %s", addr)
	http.ListenAndServe(addr, nil)
}
