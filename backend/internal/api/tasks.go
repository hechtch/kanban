package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hechtch/kanban/backend/internal/store"
)

// ─── inputs / outputs ───────────────────────────────────────────────────

type taskIDInput struct {
	ID int64 `path:"id" doc:"Task ID"`
}

type taskOutput struct {
	Body store.Task
}

type listTasksInput struct {
	Status    string `query:"status" doc:"Filter by status (todo/doing/blocked/awaiting_merge/done/backlog)" required:"false"`
	ProjectID string `query:"project_id" doc:"Filter by project id; pass 'null' for inbox-only tasks" required:"false"`
	Q         string `query:"q" doc:"Full-text search across task title and body (FTS5; multi-word = AND)" required:"false"`
}

type listTasksOutput struct {
	Body []store.Task
}

// createTaskInput accepts only the writable fields. id / sort_order /
// created_at / updated_at / completed_at / plan_slug are server-managed.
type createTaskInput struct {
	Body struct {
		Title     string   `json:"title" doc:"Task title (required)"`
		Body      string   `json:"body,omitempty" doc:"Markdown body"`
		Status    string   `json:"status,omitempty" doc:"todo / doing / blocked / awaiting_merge / done / backlog (default: todo)"`
		Priority  int      `json:"priority,omitempty" doc:"0–3"`
		DueText   string   `json:"due_text,omitempty"`
		ProjectID *int64   `json:"project_id,omitempty"`
		SortOrder float64  `json:"sort_order,omitempty"`
		Tags      []string `json:"tags,omitempty"`
		GitBranch *string  `json:"git_branch,omitempty"`
		Model     *string  `json:"model,omitempty" doc:"Suggested Claude model for an agent picking this up, e.g. fable / opus / sonnet / haiku"`
		Effort    *string  `json:"effort,omitempty" doc:"Suggested reasoning effort: low / medium / high / xhigh / max"`
	}
}

// patchTaskInput models the wire shape directly, using Optional[T] on fields
// where absent-vs-null matters (project_id today; others could later).
type patchTaskInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Title     *string          `json:"title,omitempty"`
		Body      *string          `json:"body,omitempty"`
		Status    *string          `json:"status,omitempty" doc:"todo / doing / blocked / awaiting_merge / done / backlog"`
		Priority  *int             `json:"priority,omitempty" doc:"0–3"`
		DueText   *string          `json:"due_text,omitempty"`
		ProjectID Optional[int64]  `json:"project_id,omitempty" doc:"Project id, or null to clear, or omit to leave alone"`
		SortOrder *float64         `json:"sort_order,omitempty"`
		Tags      *[]string        `json:"tags,omitempty"`
		GitBranch Optional[string] `json:"git_branch,omitempty" doc:"Branch carrying the work, or null to clear"`
		Model     Optional[string] `json:"model,omitempty" doc:"Suggested Claude model (fable / opus / sonnet / haiku), or null to clear"`
		Effort    Optional[string] `json:"effort,omitempty" doc:"Suggested reasoning effort (low / medium / high / xhigh / max), or null to clear"`
	}
}

// ─── handlers ───────────────────────────────────────────────────────────

func registerTasks(api huma.API, st *store.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Method:      http.MethodGet,
		Path:        "/api/tasks",
		Summary:     "List tasks (filtered)",
		Tags:        []string{"Tasks"},
	}, func(_ context.Context, in *listTasksInput) (*listTasksOutput, error) {
		f := store.TaskFilter{Status: in.Status, Query: in.Q}
		if in.ProjectID != "" {
			f.HasProj = true
			if in.ProjectID == "null" || in.ProjectID == "0" {
				f.ProjectID = nil
			} else {
				id, err := strconv.ParseInt(in.ProjectID, 10, 64)
				if err != nil {
					return nil, huma.Error400BadRequest("invalid project_id")
				}
				f.ProjectID = &id
			}
		}
		ts, err := st.ListTasks(f)
		if err != nil {
			return nil, huma.Error500InternalServerError("list tasks", err)
		}
		return &listTasksOutput{Body: ts}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Method:        http.MethodPost,
		Path:          "/api/tasks",
		Summary:       "Create a task",
		Tags:          []string{"Tasks"},
		DefaultStatus: http.StatusCreated,
	}, func(_ context.Context, in *createTaskInput) (*taskOutput, error) {
		t := store.Task{
			Title:     in.Body.Title,
			Body:      in.Body.Body,
			Status:    in.Body.Status,
			Priority:  in.Body.Priority,
			DueText:   in.Body.DueText,
			ProjectID: in.Body.ProjectID,
			SortOrder: in.Body.SortOrder,
			Tags:      in.Body.Tags,
			GitBranch: in.Body.GitBranch,
			Model:     in.Body.Model,
			Effort:    in.Body.Effort,
		}
		out, err := st.CreateTask(t)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return &taskOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-task",
		Method:      http.MethodPatch,
		Path:        "/api/tasks/{id}",
		Summary:     "Update a task (partial)",
		Description: "Distinguishes absent fields (left alone) from explicit null on `project_id` / `git_branch` / `model` / `effort` (cleared).",
		Tags:        []string{"Tasks"},
	}, func(_ context.Context, in *patchTaskInput) (*taskOutput, error) {
		patch := store.TaskPatch{
			Title:     in.Body.Title,
			Body:      in.Body.Body,
			Status:    in.Body.Status,
			Priority:  in.Body.Priority,
			DueText:   in.Body.DueText,
			SortOrder: in.Body.SortOrder,
			Tags:      in.Body.Tags,
		}
		if in.Body.ProjectID.Present {
			if in.Body.ProjectID.Null {
				patch.ClearProjectID = true
			} else {
				v := in.Body.ProjectID.Value
				patch.ProjectID = &v
			}
		}
		if in.Body.GitBranch.Present {
			if in.Body.GitBranch.Null {
				patch.ClearGitBranch = true
			} else {
				v := in.Body.GitBranch.Value
				patch.GitBranch = &v
			}
		}
		if in.Body.Model.Present {
			if in.Body.Model.Null {
				patch.ClearModel = true
			} else {
				v := in.Body.Model.Value
				patch.Model = &v
			}
		}
		if in.Body.Effort.Present {
			if in.Body.Effort.Null {
				patch.ClearEffort = true
			} else {
				v := in.Body.Effort.Value
				patch.Effort = &v
			}
		}
		out, err := st.UpdateTask(in.ID, patch)
		if err != nil {
			return nil, storeErr(err)
		}
		return &taskOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-task",
		Method:        http.MethodDelete,
		Path:          "/api/tasks/{id}",
		Summary:       "Delete a task",
		Tags:          []string{"Tasks"},
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *taskIDInput) (*struct{}, error) {
		if err := st.DeleteTask(in.ID); err != nil {
			return nil, storeErr(err)
		}
		return nil, nil
	})

	registerParse(api, st)
}
