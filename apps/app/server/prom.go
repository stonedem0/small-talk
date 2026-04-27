package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_http_requests_total",
		Help: "Total HTTP requests partitioned by method, path, and status code.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "smalltalk_http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "path"})

	wsConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smalltalk_ws_connections_active",
		Help: "Current number of active WebSocket connections.",
	})

	wsConnectionsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smalltalk_ws_connections_total",
		Help: "Total WebSocket connections established.",
	})

	roomUsersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "smalltalk_room_users_active",
		Help: "Active users per room.",
	}, []string{"room"})

	messagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_messages_total",
		Help: "Total messages published by type.",
	}, []string{"type"})

	heartbeatsSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smalltalk_heartbeats_sent_total",
		Help: "Total heartbeats successfully sent to directory.",
	})

	heartbeatErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "smalltalk_heartbeat_errors_total",
		Help: "Total heartbeat failures (network or non-200 response).",
	})
)

// metricsHandler returns the Prometheus scrape endpoint.
func metricsHandler() http.Handler { return promhttp.Handler() }

// instrumentedMux wraps next (the DefaultServeMux) so every request — except
// /metrics itself — is recorded by Prometheus.
func instrumentedMux(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rw, r)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rw.code)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying ResponseWriter so SSE (http.Flusher) works.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying ResponseWriter so WebSocket upgrades work.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}
