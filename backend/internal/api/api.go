package api

import (
	"encoding/json"
	"net/http"

	"github.com/chrishecht/kanban/backend/internal/store"
)

func Register(mux *http.ServeMux, _ *store.Store) {
	mux.HandleFunc("GET /api/health", health)
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
