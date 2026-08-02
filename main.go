// Package main: every standalone Go program's entry-point file lives in
// `package main` with a `func main()` — that combination is what makes
// this a runnable binary rather than an importable library.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	store := NewStore()
	s := &server{store: store}

	// chi.NewRouter gives us a router we register routes on. Think of it
	// as: "when a request matching this method+path arrives, call this
	// function."
	r := chi.NewRouter()

	// Middleware wraps every request before it reaches your handler —
	// used here for structured request logging and automatic recovery
	// if a handler panics (so one bad request can't take the whole
	// server down).
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/tasks", func(r chi.Router) {
		r.Post("/", s.createTask)
		r.Get("/", s.listTasks)
		r.Get("/{id}", s.getTask)
		r.Patch("/{id}", s.updateTaskStatus)
		r.Delete("/{id}", s.deleteTask)
	})

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	// promhttp.Handler serves every metric registered via promauto in
	// metrics.go, formatted the way Prometheus expects when it scrapes
	// this endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// The chaos endpoint only exists if CHAOS_ENABLED=true is set in the
	// environment — this keeps it out of any deployment where it wasn't
	// deliberately opted into (e.g. never accidentally live in a context
	// where you forgot it was there).
	if os.Getenv("CHAOS_ENABLED") == "true" {
		r.Post("/admin/chaos", s.chaos)
		log.Println("WARNING: chaos endpoint is ENABLED at /admin/chaos")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("taskflow listening on :%s", port)
	// http.ListenAndServe blocks forever, serving requests, until the
	// process is killed or it hits a fatal error (in which case
	// log.Fatal prints the error and exits).
	log.Fatal(http.ListenAndServe(":"+port, r))
}
