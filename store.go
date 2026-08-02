package main

import (
	"errors"
	"strconv"
	"sync"
	"time"
)

// Task is the core data model. Struct tags like `json:"id"` tell Go's
// encoding/json package what field name to use when this struct is
// converted to/from JSON — that's how Go structs map onto HTTP request
// and response bodies.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // "pending" | "in-progress" | "done"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("task not found")

// Store holds all tasks in memory. In Go, a struct with methods attached
// to it (see the `(s *Store)` receiver below) is the closest equivalent
// to a "class" in other languages — but there's no inheritance, just
// composition.
//
// sync.RWMutex is a read-write lock. Go's HTTP server handles each
// request in its own goroutine (a lightweight thread), so multiple
// requests can hit this store at the same time. Without the mutex, two
// goroutines writing to the map concurrently would corrupt it or crash
// the program — Go's race detector will flag this instantly if you
// forget it. RWMutex specifically allows many concurrent readers OR one
// writer, which is a small optimization over a plain Mutex (which only
// ever allows one goroutine in, even for reads).
type Store struct {
	mu     sync.RWMutex
	tasks  map[string]Task
	nextID int
}

func NewStore() *Store {
	return &Store{
		tasks: make(map[string]Task),
	}
}

// Create adds a new task and returns it with its assigned ID and
// timestamps filled in. The `*Store` receiver (a pointer) means this
// method operates on the actual Store, not a copy of it — required
// here since we're mutating s.tasks and s.nextID.
func (s *Store) Create(title, description string) Task {
	s.mu.Lock()
	defer s.mu.Unlock() // defer runs this when the function returns, guaranteeing the lock is released even if we return early

	s.nextID++
	id := strconv.Itoa(s.nextID)
	now := time.Now()

	task := Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.tasks[id] = task
	return task
}

// List returns all tasks as a slice (Go's dynamically-sized array type).
func (s *Store) List() []Task {
	s.mu.RLock() // read lock — multiple List/Get calls can run concurrently
	defer s.mu.RUnlock()

	result := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		result = append(result, t)
	}
	return result
}

// Get looks up a single task by ID.
//
// Go doesn't have exceptions. Instead, functions that can fail return an
// extra `error` value, and the caller is expected to check it. Returning
// (Task{}, ErrNotFound) here means "no task, and here's why" — this is
// the idiomatic Go error-handling pattern you'll see everywhere in this
// codebase.
func (s *Store) Get(id string) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return task, nil
}

// UpdateStatus changes a task's status field.
func (s *Store) UpdateStatus(id, status string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	s.tasks[id] = task
	return task, nil
}

// Delete removes a task by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}

// CountActive returns how many tasks are not yet "done" — backs the
// tasks_active gauge metric.
func (s *Store) CountActive() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, t := range s.tasks {
		if t.Status != "done" {
			count++
		}
	}
	return count
}
