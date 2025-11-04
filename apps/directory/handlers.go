package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/stonedem0/small-talk/internal/shared"
)

const ROOM_CAPACITY = 300 // max users per room

type Heartbeat struct {
	AppID      string         `json:"app_id"`
	WSURL      string         `json:"ws_url"` // e.g. wss://app-7.example.com/ws
	Rooms      map[string]int `json:"rooms,omitempty"`
	UsersTotal int            `json:"total,omitempty"`
	Draining   bool
}

type App struct {
	AppID      string
	WSURL      string
	UsersTotal int
	Rooms      map[string]int
	Draining   bool
	LastSeen   time.Time
	Healthy    bool
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
	a.WSURL = hb.WSURL
	a.UsersTotal = hb.UsersTotal
	if hb.Rooms != nil {
		a.Rooms = hb.Rooms
	}
	a.Draining = hb.Draining
	a.LastSeen = time.Now()
	a.Healthy = true

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *State) JoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	room := r.URL.Query().Get("room")
	if room == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "room parameter is required"})
		return
	}

	// keep health fresh
	s.MarkStale()

	apps := s.GetHealthyAppIDs()
	if len(apps) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no healthy apps"})
		return
	}

	ranked := shared.RankApps(room, apps)

	for _, appID := range ranked {
		if !s.BelowRoomLimit(appID, room) {
			continue
		}
		ws, ok := s.WSURL(appID)
		if !ok || ws == "" {
			continue
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"wss_url": ws + "?room=" + room,
		})
		return
	}

	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "room_full"})
}

func (s *State) WSURL(appID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.apps[appID]
	if !ok {
		return "", false
	}
	return a.WSURL, a.WSURL != ""
}

func (s *State) BelowRoomLimit(appID, room string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.apps[appID]
	if !ok {
		return false
	}
	cur := 0
	if a.Rooms != nil {
		cur = a.Rooms[room]
	}
	return cur < ROOM_CAPACITY
}

func (s *State) GetHealthyAppIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	appIDs := make([]string, 0)
	for _, a := range s.apps {
		if a.Healthy && !a.Draining && a.WSURL != "" {
			appIDs = append(appIDs, a.AppID)
		}
	}
	return appIDs
}

func (s *State) MarkStale() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.apps {
		a.Healthy = !a.Draining && now.Sub(a.LastSeen) <= s.healthyTTL
	}
}
