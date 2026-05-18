package api

import (
	"encoding/json"
	"net/http"

	"github.com/chrishecht/kanban/backend/internal/store"
)

func listProjects(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ps, err := st.ListProjects()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ps)
	}
}

func createProject(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p store.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := st.CreateProject(p)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func patchProject(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			Name      *string  `json:"name"`
			Color     *string  `json:"color"`
			SortOrder *float64 `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := st.UpdateProject(id, store.ProjectPatch{
			Name: body.Name, Color: body.Color, SortOrder: body.SortOrder,
		})
		if err != nil {
			writeErr(w, errStatus(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func deleteProject(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := st.DeleteProject(id); err != nil {
			writeErr(w, errStatus(err), err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
