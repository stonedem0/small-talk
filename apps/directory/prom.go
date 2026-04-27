package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// join outcomes: owner_routed | new_claim | no_healthy_apps | room_full
	joinRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_directory_join_requests_total",
		Help: "Total /join requests partitioned by placement outcome.",
	}, []string{"outcome"})

	joinDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smalltalk_directory_join_duration_seconds",
		Help:    "Latency of /join placement decisions.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5},
	})

	// lease operations: TryClaim / RefreshLease / Release, outcome: success | conflict | error
	leaseClaimsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_directory_lease_claims_total",
		Help: "TryClaim attempts partitioned by outcome (success|conflict|error).",
	}, []string{"outcome"})

	leaseRefreshesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_directory_lease_refreshes_total",
		Help: "RefreshLease calls partitioned by outcome (ok|error).",
	}, []string{"outcome"})

	leaseReleasesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smalltalk_directory_lease_releases_total",
		Help: "Release calls partitioned by outcome (ok|error).",
	}, []string{"outcome"})

	// cluster state (updated by MarkStale and HeartbeatHandler)
	healthyAppsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smalltalk_directory_healthy_apps",
		Help: "Number of currently healthy (non-draining, recently seen) app nodes.",
	})

	totalAppsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smalltalk_directory_total_apps",
		Help: "Total app nodes known to the directory.",
	})

	appUsersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "smalltalk_directory_app_users",
		Help: "Connected users reported per app node.",
	}, []string{"app_id"})

	appRoomsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "smalltalk_directory_app_rooms",
		Help: "Active rooms reported per app node.",
	}, []string{"app_id"})
)

// dirMetricsHandler returns the Prometheus scrape endpoint.
func dirMetricsHandler() http.Handler { return promhttp.Handler() }
