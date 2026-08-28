package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hechtch/kanban/backend/internal/store"
)

// planRecord is the GET-by-slug payload. Wraps the underlying task with a
// promoted slug and the activity log so an agent gets everything in one hit.
type planRecord struct {
	Slug     string           `json:"slug"`
	Task     store.Task       `json:"task"`
	Activity []store.Activity `json:"activity"`
}

// ─── plan inputs ────────────────────────────────────────────────────────

type slugInput struct {
	Slug string `path:"slug" doc:"Plan slug (e.g. agent-api). Lowercase kebab-case." example:"agent-api"`
}

type listPlansInput struct {
	Project string `query:"project" doc:"Filter by project slug" required:"false"`
	Q       string `query:"q" doc:"Full-text search across plan title and body (FTS5; multi-word = AND)" required:"false"`
}

type planSummary struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ProjectID *int64 `json:"project_id"`
	UpdatedAt string `json:"updated_at"`
}

type listPlansOutput struct {
	Body []planSummary
}

type planRecordOutput struct {
	Body planRecord
}

type upsertPlanInput struct {
	Slug string `path:"slug"`
	Body struct {
		Title       *string          `json:"title,omitempty"`
		Body        *string          `json:"body,omitempty" doc:"Markdown plan content"`
		Priority    *int             `json:"priority,omitempty"`
		DueText     *string          `json:"due_text,omitempty"`
		ProjectID   Optional[int64]  `json:"project_id,omitempty"`
		ProjectSlug Optional[string] `json:"project_slug,omitempty" doc:"Project slug; null clears the project. Unknown slug → 422."`
		Tags        *[]string        `json:"tags,omitempty"`
		GitBranch   Optional[string] `json:"git_branch,omitempty"`
		Model       Optional[string] `json:"model,omitempty" doc:"Suggested Claude model for whoever picks this up (fable / opus / sonnet / haiku); null clears"`
		Effort      Optional[string] `json:"effort,omitempty" doc:"Suggested reasoning effort (low / medium / high / xhigh / max); null clears"`
	}
}

type statusInput struct {
	Slug string `path:"slug"`
	Body struct {
		Status string `json:"status" doc:"todo / doing / blocked / awaiting_merge / done / backlog"`
		Note   string `json:"note,omitempty" doc:"When non-empty, writes an activity entry even if status is unchanged"`
	}
}

type noteInput struct {
	Slug string `path:"slug"`
	Body struct {
		Text string `json:"text" doc:"Free-form note appended to the activity log"`
	}
}

type activityOutput struct {
	Body []store.Activity
}

type singleActivityOutput struct {
	Body store.Activity
}

// ─── plan handlers ──────────────────────────────────────────────────────

func registerAgentPlans(api huma.API, st *store.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "agent-list-plans",
		Method:      http.MethodGet,
		Path:        "/api/agent/plans",
		Summary:     "List plans (slim summary)",
		Tags:        []string{"Agent / Plans"},
	}, func(_ context.Context, in *listPlansInput) (*listPlansOutput, error) {
		if in.Project != "" {
			if err := store.ValidateSlug(in.Project); err != nil {
				return nil, huma.Error400BadRequest(err.Error())
			}
		}
		ts, err := st.ListPlanTasks(store.PlanFilter{
			ProjectSlug: in.Project, Query: in.Q,
		})
		if err != nil {
			return nil, storeErr(err)
		}
		out := make([]planSummary, 0, len(ts))
		for _, t := range ts {
			if t.PlanSlug == nil {
				continue
			}
			out = append(out, planSummary{
				Slug: *t.PlanSlug, Title: t.Title, Status: t.Status,
				ProjectID: t.ProjectID, UpdatedAt: t.UpdatedAt,
			})
		}
		return &listPlansOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-get-plan",
		Method:      http.MethodGet,
		Path:        "/api/agent/plans/{slug}",
		Summary:     "Get plan (with activity)",
		Tags:        []string{"Agent / Plans"},
	}, func(_ context.Context, in *slugInput) (*planRecordOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		t, err := st.GetTaskByPlanSlug(in.Slug)
		if err != nil {
			return nil, storeErr(err)
		}
		acts, err := st.ListPlanActivity(in.Slug)
		if err != nil {
			return nil, huma.Error500InternalServerError("list activity", err)
		}
		return &planRecordOutput{Body: planRecord{Slug: in.Slug, Task: t, Activity: acts}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-upsert-plan",
		Method:      http.MethodPut,
		Path:        "/api/agent/plans/{slug}",
		Summary:     "Upsert a plan",
		Description: "First call creates with status=todo and writes a `create` activity. Later calls patch. `project_slug` takes precedence over `project_id`; an unknown project slug returns 422.",
		Tags:        []string{"Agent / Plans"},
	}, func(_ context.Context, in *upsertPlanInput) (*planRecordOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		patch := store.PlanUpsert{
			Title:    in.Body.Title,
			Body:     in.Body.Body,
			Priority: in.Body.Priority,
			DueText:  in.Body.DueText,
			Tags:     in.Body.Tags,
		}
		if in.Body.ProjectID.Present {
			if in.Body.ProjectID.Null {
				patch.ClearProjectID = true
			} else {
				v := in.Body.ProjectID.Value
				patch.ProjectID = &v
			}
		}
		if in.Body.ProjectSlug.Present {
			if in.Body.ProjectSlug.Null {
				patch.ClearProjectID = true
			} else {
				v := in.Body.ProjectSlug.Value
				patch.ProjectSlug = &v
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
		t, _, err := st.UpsertPlan(in.Slug, patch)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		acts, err := st.ListPlanActivity(in.Slug)
		if err != nil {
			return nil, huma.Error500InternalServerError("list activity", err)
		}
		return &planRecordOutput{Body: planRecord{Slug: in.Slug, Task: t, Activity: acts}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-set-plan-status",
		Method:      http.MethodPut,
		Path:        "/api/agent/plans/{slug}/status",
		Summary:     "Move a plan to a different status",
		Description: "Idempotent: same status without a note is a no-op. With a note, writes an activity entry.",
		Tags:        []string{"Agent / Plans"},
	}, func(_ context.Context, in *statusInput) (*planRecordOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		t, err := st.SetPlanStatus(in.Slug, in.Body.Status, in.Body.Note)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, huma.Error404NotFound(err.Error())
			}
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		acts, err := st.ListPlanActivity(in.Slug)
		if err != nil {
			return nil, huma.Error500InternalServerError("list activity", err)
		}
		return &planRecordOutput{Body: planRecord{Slug: in.Slug, Task: t, Activity: acts}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "agent-append-note",
		Method:        http.MethodPost,
		Path:          "/api/agent/plans/{slug}/notes",
		Summary:       "Append a free-form activity note",
		Tags:          []string{"Agent / Plans"},
		DefaultStatus: http.StatusCreated,
	}, func(_ context.Context, in *noteInput) (*singleActivityOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		if in.Body.Text == "" {
			return nil, huma.Error422UnprocessableEntity("text required")
		}
		act, err := st.AppendPlanNote(in.Slug, in.Body.Text)
		if err != nil {
			return nil, storeErr(err)
		}
		return &singleActivityOutput{Body: act}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-list-plan-activity",
		Method:      http.MethodGet,
		Path:        "/api/agent/plans/{slug}/activity",
		Summary:     "List activity entries for a plan",
		Tags:        []string{"Agent / Plans"},
	}, func(_ context.Context, in *slugInput) (*activityOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		acts, err := st.ListPlanActivity(in.Slug)
		if err != nil {
			return nil, storeErr(err)
		}
		return &activityOutput{Body: acts}, nil
	})
}

// ─── project handlers ───────────────────────────────────────────────────

type projectRecord struct {
	store.Project
	PlanCount int `json:"plan_count"`
}

type projectRecordOutput struct {
	Body projectRecord
}

type listProjectRecordsOutput struct {
	Body []projectRecord
}

type upsertProjectInput struct {
	Slug string `path:"slug"`
	Body struct {
		Name      *string   `json:"name,omitempty"`
		Color     *string   `json:"color,omitempty"`
		SortOrder *float64  `json:"sort_order,omitempty"`
		Archived  *bool     `json:"archived,omitempty" doc:"Finished project: hidden from the sidebar, tasks off the board"`
		Tags      *[]string `json:"tags,omitempty" doc:"Replaces the project's tag set; every task in the project carries these"`
	}
}

type listProjectPlansOutput struct {
	Body []store.Task
}

func registerAgentProjects(api huma.API, st *store.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "agent-list-projects",
		Method:      http.MethodGet,
		Path:        "/api/agent/projects",
		Summary:     "List projects (with plan counts)",
		Tags:        []string{"Agent / Projects"},
	}, func(_ context.Context, _ *struct{}) (*listProjectRecordsOutput, error) {
		projects, err := st.ListProjects()
		if err != nil {
			return nil, huma.Error500InternalServerError("list projects", err)
		}
		plans, err := st.ListPlanTasks()
		if err != nil {
			return nil, huma.Error500InternalServerError("list plans", err)
		}
		counts := map[int64]int{}
		for _, t := range plans {
			if t.ProjectID != nil {
				counts[*t.ProjectID]++
			}
		}
		out := make([]projectRecord, 0, len(projects))
		for _, p := range projects {
			out = append(out, projectRecord{Project: p, PlanCount: counts[p.ID]})
		}
		return &listProjectRecordsOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-get-project",
		Method:      http.MethodGet,
		Path:        "/api/agent/projects/{slug}",
		Summary:     "Get a project",
		Tags:        []string{"Agent / Projects"},
	}, func(_ context.Context, in *slugInput) (*projectRecordOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		p, err := st.GetProjectBySlug(in.Slug)
		if err != nil {
			return nil, storeErr(err)
		}
		plans, err := st.ListPlanTasks(in.Slug)
		if err != nil {
			return nil, huma.Error500InternalServerError("list plans", err)
		}
		return &projectRecordOutput{Body: projectRecord{Project: p, PlanCount: len(plans)}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-upsert-project",
		Method:      http.MethodPut,
		Path:        "/api/agent/projects/{slug}",
		Summary:     "Upsert a project",
		Description: "First call creates with the given name + color. Later calls patch.",
		Tags:        []string{"Agent / Projects"},
	}, func(_ context.Context, in *upsertProjectInput) (*projectOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		p, created, err := st.UpsertProjectBySlug(in.Slug, store.ProjectUpsert{
			Name: in.Body.Name, Color: in.Body.Color, SortOrder: in.Body.SortOrder,
			Archived: in.Body.Archived, Tags: in.Body.Tags,
		})
		if err != nil {
			return nil, storeErr(err)
		}
		// Huma doesn't have an easy way to flip the response code from a
		// single handler; we set the operation's DefaultStatus to 200 above
		// and live with that distinction being lost in the agent path. The
		// `created` boolean is implicit in whether activity has a `create`
		// entry from a recent timestamp.
		_ = created
		return &projectOutput{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-list-project-plans",
		Method:      http.MethodGet,
		Path:        "/api/agent/projects/{slug}/plans",
		Summary:     "List plans under a project",
		Tags:        []string{"Agent / Projects"},
	}, func(_ context.Context, in *slugInput) (*listProjectPlansOutput, error) {
		if err := store.ValidateSlug(in.Slug); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		plans, err := st.ListPlanTasks(in.Slug)
		if err != nil {
			return nil, storeErr(err)
		}
		return &listProjectPlansOutput{Body: plans}, nil
	})
}
