package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

var appHealth atomic.Value

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	log.Printf("heartbeat: %s", string(body))
	appHealth.Store(string(body))
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func joinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	room := strings.TrimSpace(r.URL.Query().Get("room"))
	if room == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Room parameter is required"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"room": room, "message": "joined"})
}
