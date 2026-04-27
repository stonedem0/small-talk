package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func setupDirectoryRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	s := miniredis.RunT(t)
	RDB = redis.NewClient(&redis.Options{Addr: s.Addr()})
	return s
}

func healthyApp(id, wsURL string) *App {
	return &App{
		AppID:    id,
		WSURL:    wsURL,
		Healthy:  true,
		Draining: false,
		LastSeen: time.Now(),
		Rooms:    map[string]int{},
	}
}

func joinRequest(room string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/join?room="+room, nil)
	return r
}

func TestJoinHandler_MissingRoom(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/join", nil)
	s.JoinHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestJoinHandler_NoHealthyApps(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()
	// no apps registered at all

	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestJoinHandler_RoutesToOwner(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	s.mu.Lock()
	s.apps["app-1"] = healthyApp("app-1", "ws://app-1:8080/ws")
	s.mu.Unlock()

	// pre-claim the room in Redis
	_, _ = TryClaim("gaming", "app-1")

	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["wss_url"] != "ws://app-1:8080/ws?room=gaming" {
		t.Fatalf("expected app-1 wss_url, got %q", resp["wss_url"])
	}
}

func TestJoinHandler_ClaimsNewRoom(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	s.mu.Lock()
	s.apps["app-1"] = healthyApp("app-1", "ws://app-1:8080/ws")
	s.mu.Unlock()

	// no existing owner in Redis
	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Redis should now have the claim
	owner, err := Owner("gaming")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "app-1" {
		t.Fatalf("expected app-1 to own the room, got %q", owner)
	}
}

func TestJoinHandler_SkipsDrainingApp(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	s.mu.Lock()
	s.apps["app-1"] = &App{
		AppID:    "app-1",
		WSURL:    "ws://app-1:8080/ws",
		Healthy:  true,
		Draining: true, // draining — should be skipped
		LastSeen: time.Now(),
		Rooms:    map[string]int{},
	}
	s.mu.Unlock()

	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when only app is draining, got %d", w.Code)
	}
}

func TestMarkStale_RecentNodeStaysHealthy(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = &App{
		AppID:    "app-1",
		Healthy:  true,
		Draining: false,
		LastSeen: time.Now(),
	}
	s.mu.Unlock()

	s.MarkStale()

	s.mu.RLock()
	healthy := s.apps["app-1"].Healthy
	s.mu.RUnlock()

	if !healthy {
		t.Fatal("expected recently-seen node to stay healthy")
	}
}

func TestMarkStale_OldNodeBecomesUnhealthy(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = &App{
		AppID:    "app-1",
		Healthy:  true,
		Draining: false,
		LastSeen: time.Now().Add(-11 * time.Second), // beyond 10s TTL
	}
	s.mu.Unlock()

	s.MarkStale()

	s.mu.RLock()
	healthy := s.apps["app-1"].Healthy
	s.mu.RUnlock()

	if healthy {
		t.Fatal("expected node with stale heartbeat to become unhealthy")
	}
}

func TestMarkStale_DrainingNodeBecomesUnhealthy(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = &App{
		AppID:    "app-1",
		Healthy:  true,
		Draining: true,
		LastSeen: time.Now(), // recent, but draining
	}
	s.mu.Unlock()

	s.MarkStale()

	s.mu.RLock()
	healthy := s.apps["app-1"].Healthy
	s.mu.RUnlock()

	if healthy {
		t.Fatal("expected draining node to be marked unhealthy")
	}
}

func TestMarkStale_MultipleNodes(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-fresh"] = &App{AppID: "app-fresh", Healthy: true, LastSeen: time.Now()}
	s.apps["app-stale"] = &App{AppID: "app-stale", Healthy: true, LastSeen: time.Now().Add(-20 * time.Second)}
	s.apps["app-drain"] = &App{AppID: "app-drain", Healthy: true, Draining: true, LastSeen: time.Now()}
	s.mu.Unlock()

	s.MarkStale()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.apps["app-fresh"].Healthy {
		t.Error("app-fresh should be healthy")
	}
	if s.apps["app-stale"].Healthy {
		t.Error("app-stale should be unhealthy")
	}
	if s.apps["app-drain"].Healthy {
		t.Error("app-drain should be unhealthy")
	}
}

func TestJoinHandler_RoomFull(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	s.mu.Lock()
	s.apps["app-1"] = &App{
		AppID:    "app-1",
		WSURL:    "ws://app-1:8080/ws",
		Healthy:  true,
		Draining: false,
		LastSeen: time.Now(),
		Rooms:    map[string]int{"gaming": ROOM_CAPACITY}, // at capacity
	}
	s.mu.Unlock()

	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 when room is full, got %d", w.Code)
	}
}

// --- DM join ---

var dirTestSecret = []byte("dir-test-secret")

func init() {
	os.Setenv("JWT_SECRET", string(dirTestSecret))
}

func makeDirToken(username string) string {
	claims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(time.Now().Add(time.Hour))}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(dirTestSecret)
	return signed
}

func dmJoinRequest(with, token string) *http.Request {
	url := "/join?type=dm&with=" + with
	r := httptest.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestJoinHandler_DM_MissingWith(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/join?type=dm", nil)
	r.Header.Set("Authorization", "Bearer "+makeDirToken("alice"))
	s.JoinHandler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing with param, got %d", w.Code)
	}
}

func TestJoinHandler_DM_MissingToken(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	s.JoinHandler(w, dmJoinRequest("bob", ""))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", w.Code)
	}
}

func TestJoinHandler_DM_CannotDMSelf(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	w := httptest.NewRecorder()
	s.JoinHandler(w, dmJoinRequest("alice", makeDirToken("alice")))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for self-DM, got %d", w.Code)
	}
}

func TestJoinHandler_DM_DerivesRoomAndRoutes(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()

	s.mu.Lock()
	s.apps["app-1"] = healthyApp("app-1", "ws://app-1:8080/ws")
	s.mu.Unlock()

	tok := makeDirToken("zara")
	w := httptest.NewRecorder()
	s.JoinHandler(w, dmJoinRequest("alice", tok))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	// sorted: alice < zara
	if resp["room"] != "dm:alice:zara" {
		t.Fatalf("expected room dm:alice:zara, got %q", resp["room"])
	}
	if resp["wss_url"] == "" {
		t.Fatal("expected wss_url in response")
	}
}

// --- join outcome counters ---

func TestJoinMetric_NoHealthyApps(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState() // no apps registered

	before := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("no_healthy_apps"))
	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))
	after := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("no_healthy_apps"))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if after-before != 1 {
		t.Fatalf("expected no_healthy_apps counter delta 1, got %.0f", after-before)
	}
}

func TestJoinMetric_NewClaim(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = healthyApp("app-1", "ws://app-1:8080/ws")
	s.mu.Unlock()

	before := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("new_claim"))
	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))
	after := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("new_claim"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if after-before != 1 {
		t.Fatalf("expected new_claim counter delta 1, got %.0f", after-before)
	}
}

func TestJoinMetric_OwnerRouted(t *testing.T) {
	setupDirectoryRedis(t)
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = healthyApp("app-1", "ws://app-1:8080/ws")
	s.apps["app-1"].Rooms = map[string]int{"gaming": 5}
	s.mu.Unlock()
	// Pre-claim the room so owner lookup succeeds.
	if _, err := TryClaim("gaming", "app-1"); err != nil {
		t.Fatal(err)
	}

	before := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("owner_routed"))
	w := httptest.NewRecorder()
	s.JoinHandler(w, joinRequest("gaming"))
	after := testutil.ToFloat64(joinRequestsTotal.WithLabelValues("owner_routed"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if after-before != 1 {
		t.Fatalf("expected owner_routed counter delta 1, got %.0f", after-before)
	}
}
