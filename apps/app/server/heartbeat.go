package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	directoryURL   = getenv("DIRECTORY_URL", "http://localhost:8081")
	appID          = getenv("APP_ID", hostnameOrFallback())
	wsPublicURL    = getenv("WS_PUBLIC_URL", "ws://localhost:8080/ws")
	heartbeatEvery = envDuration("HEARTBEAT_INTERVAL", 5*time.Second)

	httpc = &http.Client{Timeout: 1 * time.Second}
)

type hbPayload struct {
	AppID      string         `json:"app_id"`
	WSURL      string         `json:"ws_url"`
	Rooms      map[string]int `json:"rooms,omitempty"`
	UsersTotal int            `json:"total,omitempty"`
	Draining   bool           `json:"draining,omitempty"`
}

type roomCounters struct {
	mu   sync.RWMutex
	data map[string]int
	tot  atomic.Int64
}

func newRoomCounters() *roomCounters { return &roomCounters{data: make(map[string]int)} }

func (rc *roomCounters) inc(room string) {
	rc.mu.Lock()
	rc.data[room]++
	rc.mu.Unlock()
	rc.tot.Add(1)
}

func (rc *roomCounters) dec(room string) {
	rc.mu.Lock()
	if v := rc.data[room]; v > 1 {
		rc.data[room] = v - 1
	} else {
		delete(rc.data, room)
	}
	rc.mu.Unlock()
	rc.tot.Add(-1)
}

func (rc *roomCounters) snapshotActive() (map[string]int, int) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make(map[string]int, len(rc.data))
	for k, v := range rc.data {
		if v > 0 {
			out[k] = v
		}
	}
	return out, int(rc.tot.Load())
}

var (
	drainingFlag atomic.Bool
	roomsCounter = newRoomCounters()
)

func startHeartbeat(ctx context.Context) {
	// little jitter so nodes don’t align
	jitter := time.Duration(rand.Int64N(int64(heartbeatEvery / 5)))
	t := time.NewTicker(heartbeatEvery + jitter)
	go func() {
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				sendHeartbeat()
			}
		}
	}()
}

func sendHeartbeat() {
	rooms, total := roomsCounter.snapshotActive()
	payload := hbPayload{
		AppID:      appID,
		WSURL:      wsPublicURL,
		Rooms:      rooms, // only >0
		UsersTotal: total,
		Draining:   drainingFlag.Load(),
	}
	b, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", directoryURL+"/heartbeat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		log.Printf("heartbeat: post error: %v", err)
		return
	}
	fmt.Println("sent heartbeat")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("heartbeat: non-200: %d", resp.StatusCode)
	}
}

func hostnameOrFallback() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "app-unknown"
}

func envDuration(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			return dur
		}
	}
	return d
}
