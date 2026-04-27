package main

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// flusherWriter is an httptest.ResponseRecorder that also implements http.Flusher.
type flusherWriter struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flusherWriter) Flush() { f.flushed = true }

// hijackerWriter is an httptest.ResponseRecorder that also implements http.Hijacker.
type hijackerWriter struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

// --- statusRecorder ---

func TestStatusRecorder_CapturesCode(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder(), code: http.StatusOK}
	rec.WriteHeader(http.StatusTeapot)
	if rec.code != http.StatusTeapot {
		t.Fatalf("want %d, got %d", http.StatusTeapot, rec.code)
	}
}

func TestStatusRecorder_DefaultIs200(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder(), code: http.StatusOK}
	if rec.code != http.StatusOK {
		t.Fatalf("default should be 200, got %d", rec.code)
	}
}

func TestStatusRecorder_ForwardsFlusher(t *testing.T) {
	base := &flusherWriter{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: base}
	rec.Flush()
	if !base.flushed {
		t.Fatal("Flush was not forwarded — SSE would break")
	}
}

func TestStatusRecorder_FlushNoopWhenNotFlusher(t *testing.T) {
	// plain httptest.ResponseRecorder does not implement Flusher; must not panic
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	rec.Flush()
}

func TestStatusRecorder_ForwardsHijacker(t *testing.T) {
	base := &hijackerWriter{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: base}
	_, _, err := rec.Hijack()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !base.hijacked {
		t.Fatal("Hijack was not forwarded — WebSocket upgrade would break")
	}
}

func TestStatusRecorder_HijackErrorWhenNotHijacker(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	_, _, err := rec.Hijack()
	if err == nil {
		t.Fatal("expected error when underlying writer does not implement Hijacker")
	}
}

// --- instrumentedMux ---

func TestInstrumentedMux_SkipsMetricsPath(t *testing.T) {
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/metrics", "200"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := instrumentedMux(handler)
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/metrics", "200"))
	if after != before {
		t.Fatal("instrumentedMux must not record metrics for the /metrics path itself")
	}
}

func TestInstrumentedMux_RecordsRegularRequest(t *testing.T) {
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/ping", "200"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := instrumentedMux(handler)
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/ping", "200"))
	if after-before != 1 {
		t.Fatalf("expected counter delta 1, got %.0f", after-before)
	}
}

func TestInstrumentedMux_RecordsErrorStatus(t *testing.T) {
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/login", "401"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	wrapped := instrumentedMux(handler)
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/login", nil))

	after := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/login", "401"))
	if after-before != 1 {
		t.Fatalf("expected counter delta 1, got %.0f", after-before)
	}
}
