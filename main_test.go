package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// --- Store tests ---
// These test the in-memory store directly, with no HTTP involved. Go's
// testing convention: any function named TestXxx(t *testing.T) in a
// _test.go file is automatically discovered and run by `go test`.

func TestStore_CreateAndGet(t *testing.T) {
	s := NewStore()

	created := s.Create("Buy milk", "2%, not whole")
	if created.ID == "" {
		t.Fatal("expected a non-empty ID")
	}
	if created.Status != "pending" {
		t.Errorf("expected new task status to be 'pending', got %q", created.Status)
	}

	fetched, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error fetching task: %v", err)
	}
	if fetched.Title != "Buy milk" {
		t.Errorf("expected title 'Buy milk', got %q", fetched.Title)
	}
}

func TestStore_GetMissing(t *testing.T) {
	s := NewStore()
	_, err := s.Get("does-not-exist")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_UpdateStatus(t *testing.T) {
	s := NewStore()
	task := s.Create("Test task", "")

	updated, err := s.UpdateStatus(task.ID, "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != "done" {
		t.Errorf("expected status 'done', got %q", updated.Status)
	}
}

func TestStore_Delete(t *testing.T) {
	s := NewStore()
	task := s.Create("To be deleted", "")

	if err := s.Delete(task.ID); err != nil {
		t.Fatalf("unexpected error deleting: %v", err)
	}
	if _, err := s.Get(task.ID); err != ErrNotFound {
		t.Errorf("expected task to be gone after delete, got err=%v", err)
	}
}

func TestStore_CountActive(t *testing.T) {
	s := NewStore()
	a := s.Create("A", "")
	s.Create("B", "")
	s.UpdateStatus(a.ID, "done")

	if got := s.CountActive(); got != 1 {
		t.Errorf("expected 1 active task, got %d", got)
	}
}

// --- HTTP handler tests ---
// These spin up the real router (same one main.go builds) and send
// requests through httptest, which simulates HTTP without opening an
// actual network port. This is the standard way to test Go HTTP
// handlers end-to-end.

func newTestRouter() http.Handler {
	s := &server{store: NewStore()}
	r := chi.NewRouter()
	r.Route("/tasks", func(r chi.Router) {
		r.Post("/", s.createTask)
		r.Get("/", s.listTasks)
		r.Get("/{id}", s.getTask)
		r.Patch("/{id}", s.updateTaskStatus)
		r.Delete("/{id}", s.deleteTask)
	})
	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)
	return r
}

func TestHandler_CreateTask(t *testing.T) {
	router := newTestRouter()

	body, _ := json.Marshal(createTaskRequest{Title: "Write tests", Description: "for taskflow"})
	req := httptest.NewRequest(http.MethodPost, "/tasks/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var got Task
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Title != "Write tests" {
		t.Errorf("expected title 'Write tests', got %q", got.Title)
	}
}

func TestHandler_CreateTask_MissingTitle(t *testing.T) {
	router := newTestRouter()

	body, _ := json.Marshal(createTaskRequest{Description: "no title given"})
	req := httptest.NewRequest(http.MethodPost, "/tasks/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing title, got %d", rec.Code)
	}
}

func TestHandler_GetTask_NotFound(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/tasks/nope", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandler_UpdateStatus_InvalidValue(t *testing.T) {
	router := newTestRouter()

	// create a task first
	createBody, _ := json.Marshal(createTaskRequest{Title: "Task"})
	createReq := httptest.NewRequest(http.MethodPost, "/tasks/", bytes.NewReader(createBody))
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created Task
	json.Unmarshal(createRec.Body.Bytes(), &created)

	// now try an invalid status transition
	updateBody, _ := json.Marshal(updateStatusRequest{Status: "not-a-real-status"})
	updateReq := httptest.NewRequest(http.MethodPatch, "/tasks/"+created.ID, bytes.NewReader(updateBody))
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid status value, got %d", updateRec.Code)
	}
}

func TestHandler_Healthz(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
