package shared

import (
	"hash/fnv"
	"sort"
)

func Score(room, appID string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(room))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(appID))
	return h.Sum64()
}

func RankApps(room string, appIDs []string) []string {
	type pair struct {
		id string
		s  uint64
	}
	ps := make([]pair, 0, len(appIDs))
	for _, id := range appIDs {
		ps = append(ps, pair{id, Score(room, id)})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].s > ps[j].s })
	out := make([]string, len(ps))
	for i := range ps {
		out[i] = ps[i].id
	}
	return out
}
