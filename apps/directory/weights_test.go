package main

import (
	"testing"
	"time"
)

func idleApp(id string) *App {
	return &App{
		AppID:    id,
		WSURL:    "ws://" + id + ":8080/ws",
		Healthy:  true,
		Draining: false,
		LastSeen: time.Now(),
		Stats:    &NodeStats{CPUPercent: 0, RSSMB: 0},
	}
}

func loadedApp(id string) *App {
	return &App{
		AppID:      id,
		WSURL:      "ws://" + id + ":8080/ws",
		Healthy:    true,
		Draining:   false,
		LastSeen:   time.Now(),
		UsersTotal: int(TargetUsersPerNode),
		Stats:      &NodeStats{CPUPercent: 90, RSSMB: MemBudgetMB},
	}
}

// --- nodeWeight ---

func TestNodeWeight_NilApp(t *testing.T) {
	if w := nodeWeight(nil); w != 0 {
		t.Fatalf("expected 0 for nil app, got %f", w)
	}
}

func TestNodeWeight_Unhealthy(t *testing.T) {
	a := idleApp("app-1")
	a.Healthy = false
	if w := nodeWeight(a); w != 0 {
		t.Fatalf("expected 0 for unhealthy app, got %f", w)
	}
}

func TestNodeWeight_Draining(t *testing.T) {
	a := idleApp("app-1")
	a.Draining = true
	if w := nodeWeight(a); w != 0 {
		t.Fatalf("expected 0 for draining app, got %f", w)
	}
}

func TestNodeWeight_NoWSURL(t *testing.T) {
	a := idleApp("app-1")
	a.WSURL = ""
	if w := nodeWeight(a); w != 0 {
		t.Fatalf("expected 0 for app with no WSURL, got %f", w)
	}
}

func TestNodeWeight_IdleNodeIsMaxWeight(t *testing.T) {
	a := idleApp("app-1")
	w := nodeWeight(a)
	// idle node: penalty = 1 + 0 + 0 + 0 = 1, weight = 1.0
	if w < 0.99 || w > 1.0 {
		t.Fatalf("expected weight ~1.0 for idle node, got %f", w)
	}
}

func TestNodeWeight_HighCPUReducesWeight(t *testing.T) {
	idle := idleApp("app-idle")
	busy := idleApp("app-busy")
	busy.Stats.CPUPercent = 90

	wIdle := nodeWeight(idle)
	wBusy := nodeWeight(busy)

	if wBusy >= wIdle {
		t.Fatalf("expected high-CPU node to have lower weight: idle=%.3f busy=%.3f", wIdle, wBusy)
	}
}

func TestNodeWeight_HighMemoryReducesWeight(t *testing.T) {
	idle := idleApp("app-idle")
	heavy := idleApp("app-heavy")
	heavy.Stats.RSSMB = MemBudgetMB

	wIdle := nodeWeight(idle)
	wHeavy := nodeWeight(heavy)

	if wHeavy >= wIdle {
		t.Fatalf("expected high-memory node to have lower weight: idle=%.3f heavy=%.3f", wIdle, wHeavy)
	}
}

func TestNodeWeight_HighUserCountReducesWeight(t *testing.T) {
	idle := idleApp("app-idle")
	crowded := idleApp("app-crowded")
	crowded.UsersTotal = TargetUsersPerNode

	wIdle := nodeWeight(idle)
	wCrowded := nodeWeight(crowded)

	if wCrowded >= wIdle {
		t.Fatalf("expected high-user node to have lower weight: idle=%.3f crowded=%.3f", wIdle, wCrowded)
	}
}

func TestNodeWeight_MaxLoadHasLowestWeight(t *testing.T) {
	idle := idleApp("app-idle")
	maxed := loadedApp("app-maxed")

	wIdle := nodeWeight(idle)
	wMaxed := nodeWeight(maxed)

	if wMaxed >= wIdle {
		t.Fatalf("expected max-load node to have lower weight: idle=%.3f maxed=%.3f", wIdle, wMaxed)
	}
}

// --- WeightedRankApps ---

func TestWeightedRankApps_ExcludesUnhealthy(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-good"] = idleApp("app-good")
	s.apps["app-sick"] = &App{AppID: "app-sick", WSURL: "ws://x", Healthy: false}
	s.apps["app-drain"] = func() *App { a := idleApp("app-drain"); a.Draining = true; return a }()
	s.mu.Unlock()

	ranked := s.WeightedRankApps("gaming", []string{"app-good", "app-sick", "app-drain"})

	for _, id := range ranked {
		if id == "app-sick" || id == "app-drain" {
			t.Fatalf("expected unhealthy/draining app to be excluded, but got %s in results", id)
		}
	}
	if len(ranked) != 1 || ranked[0] != "app-good" {
		t.Fatalf("expected only app-good in results, got %v", ranked)
	}
}

func TestWeightedRankApps_Deterministic(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = idleApp("app-1")
	s.apps["app-2"] = idleApp("app-2")
	s.apps["app-3"] = idleApp("app-3")
	s.mu.Unlock()

	appIDs := []string{"app-1", "app-2", "app-3"}
	first := s.WeightedRankApps("gaming", appIDs)
	second := s.WeightedRankApps("gaming", appIDs)

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("ranking not deterministic: %v vs %v", first, second)
		}
	}
}

func TestWeightedRankApps_EmptyWhenNoHealthyNodes(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.apps["app-1"] = &App{AppID: "app-1", Healthy: false}
	s.mu.Unlock()

	ranked := s.WeightedRankApps("gaming", []string{"app-1"})
	if len(ranked) != 0 {
		t.Fatalf("expected empty ranking, got %v", ranked)
	}
}
