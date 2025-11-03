package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// var appHealth atomic.Value

type Heartbeat struct {
	AppID    string         `json:"app_id"`
	WSURL    string         `json:"ws_url"` // e.g. wss://app-7.example.com/ws
	Rooms    map[string]int `json:"rooms,omitempty"`
	Total    int            `json:"total,omitempty"`
	Draining bool
}

type App struct {
	AppID    string
	WSURL    string
	Total    int
	Rooms    map[string]int
	Draining bool
	LastSeen time.Time
	Healthy  bool
}

type State struct {
	mu         sync.RWMutex
	apps       map[string]*App
	healthyTTL time.Duration
}

func NewState() *State {
	return &State{
		apps:       make(map[string]*App),
		healthyTTL: 10 * time.Second,
	}
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
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

func (s *State) HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request method"})
		return
	}
	var hb Heartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	if hb.AppID == "" || hb.WSURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "AppID and WSURL are required"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[hb.AppID]
	if !ok {
		a = &App{AppID: hb.AppID}
		s.apps[hb.AppID] = a
	}

	// body, _ := io.ReadAll(r.Body)
	// _ = r.Body.Close()
	log.Printf("heartbeat: %s", hb)
	// appHealth.Store(string(body))
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func JoinHandler(w http.ResponseWriter, r *http.Request) {
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

func (s *State) MarkStale() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.apps {
		a.Healthy = !a.Draining && now.Sub(a.LastSeen) <= s.healthyTTL
	}
}
