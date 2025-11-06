package main

import (
	"runtime/metrics"
)

type NodeStats struct {
	Goroutines    int
	RSSMB         uint64
	LastGCPauseMS uint64
	NumGC         uint32
	CPUPercent    float64
}

func collectNodeStats() NodeStats {
	samples := []metrics.Sample{
		{Name: "/cpu/classes/gc/mark/assist:cpu-seconds"},
		{Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/memory/classes/heap/free:bytes"},
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/sched/goroutines:goroutines"},
		{Name: "/gc/heap/allocs:bytes"},
		{Name: "/gc/heap/frees:bytes"},
		{Name: "/gc/pauses:seconds"},
	}

	metrics.Read(samples)

	s := NodeStats{}
	for _, sm := range samples {
		switch sm.Name {
		case "/sched/goroutines:goroutines":
			s.Goroutines = int(sm.Value.Uint64())
		case "/memory/classes/heap/objects:bytes":
			s.RSSMB = sm.Value.Uint64() / 1024 / 1024
		case "/gc/pauses:seconds":
			s.LastGCPauseMS = uint64(sm.Value.Float64() * 1000)
		}
	}

	s.CPUPercent = estimateCPUPercent() // optional helper you can stub or leave 0
	s.NumGC = uint32(samples[1].Value.Uint64())
	return s
}
