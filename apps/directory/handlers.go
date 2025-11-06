package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stonedem0/small-talk/internal/shared"
)

const ROOM_CAPACITY = 300 // max users per room

type Heartbeat struct {
	AppID      string         `json:"app_id"`
	WSURL      string         `json:"ws_url"` // e.g. wss://app-7.example.com/ws
	Rooms      map[string]int `json:"rooms,omitempty"`
	UsersTotal int            `json:"total,omitempty"`
	Draining   bool           `json:"draining,omitempty"`
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
func withCORSAndAuth(requireAuth bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if o := allowOrigin(origin); o != "" {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if requireAuth {
			tok := extractBearer(r)
			if tok == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing token"})
				return
			}
			if _, err := requireJWT(tok); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}

var allowedOrigins = func() []string {
	s := os.Getenv("DIRECTORY_CORS_ORIGINS")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}()

func allowOrigin(origin string) string {
	if origin == "" {
		return ""
	}
	if len(allowedOrigins) == 0 {
		return origin // dev: reflect
	}
	for _, a := range allowedOrigins {
		if a == origin {
			return origin
		}
	}
	return ""
}

func dirJWTSecret() []byte {
	if v := os.Getenv("DIRECTORY_JWT_SECRET"); strings.TrimSpace(v) != "" {
		return []byte(v)
	}
	return []byte(os.Getenv("JWT_SECRET"))
}

func extractBearer(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if v := r.URL.Query().Get("token"); v != "" {
		return v
	}
	return ""
}

func requireJWT(tokenStr string) (jwt.MapClaims, error) {
	secret := dirJWTSecret()
	if len(secret) == 0 {
		return nil, fmt.Errorf("directory jwt secret not configured")
	}
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected jwt alg: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	cl, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return cl, nil
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
	w.Header().Set("Content-Type", "application/json")
	var hb Heartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		log.Printf("heartbeat decode error: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	if hb.AppID == "" || hb.WSURL == "" {
		log.Printf("heartbeat missing fields: app_id=%q ws_url=%q", hb.AppID, hb.WSURL)
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

	for room, count := range hb.Rooms {
		if count > 0 {
			if err := RefreshLease(room, hb.AppID); err != nil {
				log.Printf("refresh lease error room=%s app=%s: %v", room, hb.AppID, err)
			}
		} else {
			if err := Release(room, hb.AppID); err != nil {
				log.Printf("release lease error room=%s app=%s: %v", room, hb.AppID, err)
			}
		}
	}

	log.Printf("heartbeat ok app=%s total=%d draining=%v", hb.AppID, hb.UsersTotal, hb.Draining)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *State) JoinHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	room := r.URL.Query().Get("room")
	if room == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "room parameter is required"})
		log.Printf("join: missing room param")
		return
	}
	s.MarkStale()

	if owner, err := Owner(room); err != nil {
		log.Printf("join: owner lookup error room=%s: %v", room, err)
	} else if owner != "" {
		if a, ok := s.getApp(owner); ok && a.Healthy && !a.Draining && s.belowRoomLimit(owner, room) {
			log.Printf("join: using owner app=%s for room=%s", owner, room)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"wss_url": a.WSURL + "?room=" + room,
			})
			return
		}
		log.Printf("join: owner app not eligible app=%s room=%s", owner, room)
	}
	apps := s.getHealthyAppIDs()
	if len(apps) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no healthy apps"})
		log.Printf("join: no healthy apps for room=%s", room)
		return
	}

	ranked := shared.RankApps(room, apps)

	for _, appID := range ranked {
		if !s.belowRoomLimit(appID, room) {
			log.Printf("join: app over room limit app=%s room=%s", appID, room)
			continue
		}
		claimed, err := TryClaim(room, appID)
		if err != nil {
			log.Printf("join: try-claim error room=%s app=%s: %v", room, appID, err)
			continue
		}
		if !claimed {
			log.Printf("join: try-claim conflict room=%s app=%s", room, appID)
			continue
		}
		if a, ok := s.getApp(appID); ok && a.WSURL != "" {
			log.Printf("join: assigned app=%s for room=%s", appID, room)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"wss_url": a.WSURL + "?room=" + room,
			})
			return
		}
		err = Release(room, appID)
		if err != nil {
			log.Printf("join: release error room=%s app=%s: %v", room, appID, err)
			continue
		}

	}

	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "room_full"})
	log.Printf("join: room full room=%s", room)
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

func (s *State) belowRoomLimit(appID, room string) bool {
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

func (s *State) getApp(id string) (*App, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.apps[id]
	return a, ok
}

func (s *State) getHealthyAppIDs() []string {
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
