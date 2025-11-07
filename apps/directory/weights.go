package main

import (
	"math"
	"sort"

	"github.com/stonedem0/small-talk/internal/shared"
)

const (
	alphaCPU   = 0.7
	alphaMem   = 0.7
	alphaUsers = 0.4

	MemBudgetMB        = 2048
	TargetUsersPerNode = 800
)

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func nodeWeight(a *App) float64 {
	// hard filters
	if a == nil || !a.Healthy || a.Draining || a.WSURL == "" {
		return 0
	}
	cpu := clamp01((func() float64 {
		if a.Stats == nil {
			return 0
		}
		return a.Stats.CPUPercent / 100.0
	})())
	mem := clamp01((func() float64 {
		if a.Stats == nil || MemBudgetMB <= 0 {
			return 0
		}
		return float64(a.Stats.RSSMB) / float64(MemBudgetMB)
	})())
	users := clamp01((func() float64 {
		if TargetUsersPerNode <= 0 {
			return 0
		}
		return float64(a.UsersTotal) / float64(TargetUsersPerNode)
	})())

	penalty := 1 + alphaCPU*cpu + alphaMem*mem + alphaUsers*users
	w := 1.0 / penalty
	// tiny floor to keep ties stable but let true 0 (draining/unhealthy) be zero
	return math.Max(w, 0.0001)
}

// WeightedRankApps: HRW score multiplied by weight
func (s *State) WeightedRankApps(room string, appIDs []string) []string {
	type pair struct {
		id  string
		sc  uint64
		w   float64
		val float64
	}
	ps := make([]pair, 0, len(appIDs))
	for _, id := range appIDs {
		a := s.apps[id]
		w := nodeWeight(a)
		if w == 0 {
			continue
		}
		hrw := shared.Score(room, id)
		val := float64(hrw) * w
		ps = append(ps, pair{id: id, sc: hrw, w: w, val: val})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].val > ps[j].val })
	out := make([]string, len(ps))
	for i := range ps {
		out[i] = ps[i].id
	}
	return out
}
