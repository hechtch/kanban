package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/chrishecht/kanban/backend/internal/store"
)

func Register(mux *http.ServeMux, st *store.Store) {
	mux.HandleFunc("GET /api/health", health)

	mux.HandleFunc("GET /api/projects", listProjects(st))
	mux.HandleFunc("POST /api/projects", createProject(st))
	mux.HandleFunc("PATCH /api/projects/{id}", patchProject(st))
	mux.HandleFunc("DELETE /api/projects/{id}", deleteProject(st))

	mux.HandleFunc("GET /api/tasks", listTasks(st))
	mux.HandleFunc("POST /api/tasks", createTask(st))
	mux.HandleFunc("POST /api/tasks/parse", parseTask(st))
	mux.HandleFunc("PATCH /api/tasks/{id}", patchTask(st))
	mux.HandleFunc("DELETE /api/tasks/{id}", deleteTask(st))
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func errStatus(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}
