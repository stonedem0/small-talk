package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret")

func init() {
	jwtSecret = testSecret
}

func makeToken(username string, exp time.Time) string {
	claims := jwt.MapClaims{"username": username, "exp": jwt.NewNumericDate(exp)}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(testSecret)
	return signed
}

func newApp() *app {
	a := &app{}
	return a
}

func wsRequest(room, token string) *http.Request {
	url := "/ws"
	if room != "" {
		url += "?room=" + room
	}
	r := httptest.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestHandleConnections_ServerShuttingDown(t *testing.T) {
	a := newApp()
	a.shutting.Store(true)

	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("gaming", makeToken("alice", time.Now().Add(time.Hour))))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleConnections_MissingRoom(t *testing.T) {
	a := newApp()

	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("", makeToken("alice", time.Now().Add(time.Hour))))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleConnections_MissingToken(t *testing.T) {
	a := newApp()

	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("gaming", ""))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleConnections_InvalidToken(t *testing.T) {
	a := newApp()

	w := httptest.NewRecorder()
	r := wsRequest("gaming", "not.a.valid.token")
	handleConnections(a, w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleConnections_ExpiredToken(t *testing.T) {
	a := newApp()

	expired := makeToken("alice", time.Now().Add(-time.Hour))
	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("gaming", expired))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// reachedUpgrader returns true when auth passed and we hit the WS upgrader.
// Gorilla writes "Bad Request\n" when the request isn't a real WS connection,
// which is distinct from our own auth error messages.
func reachedUpgrader(w *httptest.ResponseRecorder) bool {
	return w.Code == http.StatusBadRequest && strings.TrimSpace(w.Body.String()) == "Bad Request"
}

func TestHandleConnections_TokenViaQueryParam(t *testing.T) {
	a := newApp()

	tok := makeToken("alice", time.Now().Add(time.Hour))
	r := httptest.NewRequest(http.MethodGet, "/ws?room=gaming&token="+tok, nil)
	w := httptest.NewRecorder()
	handleConnections(a, w, r)

	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected auth to pass via query param, got 401: %s", w.Body.String())
	}
	if !reachedUpgrader(w) {
		t.Fatalf("expected to reach WS upgrader, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleConnections_TokenViaWebSocketProtocol(t *testing.T) {
	a := newApp()

	tok := makeToken("alice", time.Now().Add(time.Hour))
	r := httptest.NewRequest(http.MethodGet, "/ws?room=gaming", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "Bearer "+tok)
	w := httptest.NewRecorder()
	handleConnections(a, w, r)

	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected auth to pass via Sec-WebSocket-Protocol, got 401: %s", w.Body.String())
	}
	if !reachedUpgrader(w) {
		t.Fatalf("expected to reach WS upgrader, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleConnections_DMForbidsNonParticipant(t *testing.T) {
	a := newApp()

	tok := makeToken("carol", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("dm:alice:bob", tok))

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-participant DM, got %d", w.Code)
	}
}

func TestHandleConnections_DMAllowsParticipant(t *testing.T) {
	a := newApp()

	tok := makeToken("alice", time.Now().Add(time.Hour))
	w := httptest.NewRecorder()
	handleConnections(a, w, wsRequest("dm:alice:bob", tok))

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected participant to be allowed into DM room, got 403")
	}
}
