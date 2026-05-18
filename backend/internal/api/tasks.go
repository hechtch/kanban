package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chrishecht/kanban/backend/internal/store"
)

func listTasks(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := store.TaskFilter{
			Status: r.URL.Query().Get("status"),
			Query:  r.URL.Query().Get("q"),
		}
		if raw := r.URL.Query().Get("project_id"); raw != "" {
			f.HasProj = true
			if raw == "null" || raw == "0" {
				f.ProjectID = nil
			} else {
				id, err := strconv.ParseInt(raw, 10, 64)
				if err != nil {
					writeErr(w, http.StatusBadRequest, "invalid project_id")
					return
				}
				f.ProjectID = &id
			}
		}
		ts, err := st.ListTasks(f)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ts)
	}
}

func createTask(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var t store.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := st.CreateTask(t)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// Decode into a raw map first so we can tell "project_id absent" from
// "project_id: null" — the former leaves the value alone, the latter clears it.
func patchTask(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		raw := map[string]json.RawMessage{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		patch, err := buildTaskPatch(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := st.UpdateTask(id, patch)
		if err != nil {
			writeErr(w, errStatus(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func deleteTask(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := st.DeleteTask(id); err != nil {
			writeErr(w, errStatus(err), err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func buildTaskPatch(raw map[string]json.RawMessage) (store.TaskPatch, error) {
	var p store.TaskPatch
	if v, ok := raw["title"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return p, err
		}
		p.Title = &s
	}
	if v, ok := raw["body"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return p, err
		}
		p.Body = &s
	}
	if v, ok := raw["status"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return p, err
		}
		p.Status = &s
	}
	if v, ok := raw["priority"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			return p, err
		}
		p.Priority = &n
	}
	if v, ok := raw["due_text"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return p, err
		}
		p.DueText = &s
	}
	if v, ok := raw["project_id"]; ok {
		if string(v) == "null" {
			p.ClearProjectID = true
		} else {
			var n int64
			if err := json.Unmarshal(v, &n); err != nil {
				return p, err
			}
			p.ProjectID = &n
		}
	}
	if v, ok := raw["sort_order"]; ok {
		var f float64
		if err := json.Unmarshal(v, &f); err != nil {
			return p, err
		}
		p.SortOrder = &f
	}
	if v, ok := raw["tags"]; ok {
		var ts []string
		if err := json.Unmarshal(v, &ts); err != nil {
			return p, err
		}
		p.Tags = &ts
	}
	return p, nil
}
