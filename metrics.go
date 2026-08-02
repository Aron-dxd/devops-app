package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// These are the app's custom business metrics — the whole reason
// Project 2's dashboards will show something meaningful instead of
// generic "HTTP requests per second" numbers every demo app has.
//
// promauto.New... both creates the metric AND registers it with
// Prometheus's default registry in one call — that registry is what
// gets exposed at /metrics for Prometheus to scrape.
var (
	// Counter: a number that only ever goes up (or resets to zero on
	// restart). Right tool for "how many times has X happened, total."
	tasksCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tasks_created_total",
		Help: "Total number of tasks created since the app started.",
	})

	tasksCompletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tasks_completed_total",
		Help: "Total number of tasks marked as done.",
	})

	// Gauge: a number that can go up or down. Right tool for "how many
	// X exist right now" — unlike a counter, this can decrease.
	tasksActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_active",
		Help: "Current number of tasks not yet marked done.",
	})

	// Histogram: buckets observations (here, how long an operation took)
	// into predefined ranges, so Prometheus/Grafana can later compute
	// percentiles (p50, p95, p99) — not just an average, which hides
	// outliers. The "labels" argument (`operation`) lets you slice this
	// one histogram by which handler it came from (create/list/update),
	// instead of needing a separate histogram per handler.
	taskOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "task_operation_duration_seconds",
		Help:    "Time taken to complete a task store operation, in seconds.",
		Buckets: prometheus.DefBuckets, // sensible default bucket boundaries
	}, []string{"operation"})
)
