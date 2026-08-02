package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

// server holds shared dependencies that handlers need access to. Bundling
// them in a struct and attaching handlers as methods (see below) is the
// idiomatic Go alternative to global variables or dependency-injection
// frameworks you might expect from other languages.
type server struct {
	store *Store
}

// --- request/response payload shapes ---
// Small structs used only for decoding incoming JSON. Keeping them
// separate from the Task model means the API's input shape can evolve
// independently of the internal data model.

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON is a small shared helper so every handler doesn't repeat the
// same three lines of header-setting and encoding.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// createTask handles POST /tasks.
//
// Every handler in Go's net/http world has this exact signature:
// func(w http.ResponseWriter, r *http.Request). w is how you write the
// response back; r is the incoming request. This is the same shape for
// every single handler below — once it clicks once, it clicks everywhere.
func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskOperationDuration.WithLabelValues("create").Observe(time.Since(start).Seconds())
	}()

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "title is required"})
		return
	}

	task := s.store.Create(req.Title, req.Description)

	tasksCreatedTotal.Inc()       // counter += 1
	tasksActive.Set(float64(s.store.CountActive()))

	writeJSON(w, http.StatusCreated, task)
}

// listTasks handles GET /tasks.
func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskOperationDuration.WithLabelValues("list").Observe(time.Since(start).Seconds())
	}()

	tasks := s.store.List()
	writeJSON(w, http.StatusOK, tasks)
}

// getTask handles GET /tasks/{id}.
func (s *server) getTask(w http.ResponseWriter, r *http.Request) {
	// chi.URLParam pulls the {id} segment out of the URL path — this only
	// works because of how the route is registered in main.go.
	id := chi.URLParam(r, "id")

	task, err := s.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// updateTaskStatus handles PATCH /tasks/{id}.
func (s *server) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskOperationDuration.WithLabelValues("update").Observe(time.Since(start).Seconds())
	}()

	id := chi.URLParam(r, "id")

	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	valid := map[string]bool{"pending": true, "in-progress": true, "done": true}
	if !valid[req.Status] {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "status must be pending, in-progress, or done"})
		return
	}

	task, err := s.store.UpdateStatus(id, req.Status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "task not found"})
		return
	}

	if req.Status == "done" {
		tasksCompletedTotal.Inc()
	}
	tasksActive.Set(float64(s.store.CountActive()))

	writeJSON(w, http.StatusOK, task)
}

// deleteTask handles DELETE /tasks/{id}.
func (s *server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.store.Delete(id); err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "task not found"})
		return
	}

	tasksActive.Set(float64(s.store.CountActive()))
	w.WriteHeader(http.StatusNoContent) // 204: success, nothing to return
}

// healthz is a liveness probe: "is the process alive at all." Kubernetes
// restarts the pod if this stops responding.
func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz is a readiness probe: "is this instance ready to serve traffic
// right now." Kubernetes stops routing traffic to a pod that fails this,
// without necessarily restarting it — useful for e.g. "still warming up"
// or "database connection lost" states. This app is simple enough that
// ready == alive today, but keeping the endpoint separate from healthz
// is what lets that logic diverge later without an API-breaking change.
func (s *server) readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// chaos is deliberately dangerous and is only wired up in main.go when
// the CHAOS_ENABLED environment variable is set — see main.go. It exists
// purely so Project 2's chaos-drill days have an on-demand, repeatable
// way to break the app, instead of relying on manually timing
// `kubectl delete pod` during a live demo.
//
// Body: {"mode": "latency"} or {"mode": "crash"}
func (s *server) chaos(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	switch req.Mode {
	case "latency":
		delay := time.Duration(2000+rand.Intn(3000)) * time.Millisecond
		time.Sleep(delay)
		writeJSON(w, http.StatusOK, map[string]string{"induced": "latency", "delay": delay.String()})
	case "crash":
		writeJSON(w, http.StatusOK, map[string]string{"induced": "crash", "note": "process exiting now"})
		os.Exit(1) // kills the process immediately — this is the point
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "mode must be latency or crash"})
	}
}
