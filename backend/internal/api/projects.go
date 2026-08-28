package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hechtch/kanban/backend/internal/store"
)

// ─── inputs / outputs ───────────────────────────────────────────────────

type projectIDInput struct {
	ID int64 `path:"id" doc:"Project ID" example:"1"`
}

type projectOutput struct {
	Body store.Project
}

type listProjectsOutput struct {
	Body []store.Project
}

// createProjectInput accepts only the writable fields. id and slug are
// assigned server-side; sort_order defaults to 0 if omitted.
type createProjectInput struct {
	Body struct {
		Name      string   `json:"name" doc:"Display name (required)"`
		Color     string   `json:"color,omitempty" doc:"Hex color, defaults to #1f2430"`
		Slug      string   `json:"slug,omitempty" doc:"Override the auto-derived slug"`
		SortOrder float64  `json:"sort_order,omitempty"`
		Archived  bool     `json:"archived,omitempty" doc:"Finished project: hidden from the sidebar, tasks off the board"`
		Tags      []string `json:"tags,omitempty" doc:"Tags every task in this project carries"`
	}
}

type patchProjectInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Name      *string   `json:"name,omitempty" doc:"Display name"`
		Color     *string   `json:"color,omitempty" doc:"Hex color (e.g. #d4654a)"`
		SortOrder *float64  `json:"sort_order,omitempty" doc:"Fractional sort order"`
		Archived  *bool     `json:"archived,omitempty" doc:"Finished project: hidden from the sidebar, tasks off the board"`
		Tags      *[]string `json:"tags,omitempty" doc:"Replaces the project's tag set; every task in the project carries these"`
	}
}

// ─── handlers ───────────────────────────────────────────────────────────

func registerProjects(api huma.API, st *store.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "list-projects",
		Method:      http.MethodGet,
		Path:        "/api/projects",
		Summary:     "List all projects",
		Tags:        []string{"Projects"},
	}, func(_ context.Context, _ *struct{}) (*listProjectsOutput, error) {
		ps, err := st.ListProjects()
		if err != nil {
			return nil, huma.Error500InternalServerError("list projects", err)
		}
		return &listProjectsOutput{Body: ps}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-project",
		Method:        http.MethodPost,
		Path:          "/api/projects",
		Summary:       "Create a project",
		Tags:          []string{"Projects"},
		DefaultStatus: http.StatusCreated,
	}, func(_ context.Context, in *createProjectInput) (*projectOutput, error) {
		out, err := st.CreateProject(store.Project{
			Name: in.Body.Name, Color: in.Body.Color, Slug: in.Body.Slug, SortOrder: in.Body.SortOrder,
			Archived: in.Body.Archived, Tags: in.Body.Tags,
		})
		if err != nil {
			return nil, storeErr(err)
		}
		return &projectOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "patch-project",
		Method:      http.MethodPatch,
		Path:        "/api/projects/{id}",
		Summary:     "Update a project",
		Tags:        []string{"Projects"},
	}, func(_ context.Context, in *patchProjectInput) (*projectOutput, error) {
		out, err := st.UpdateProject(in.ID, store.ProjectPatch{
			Name: in.Body.Name, Color: in.Body.Color, SortOrder: in.Body.SortOrder,
			Archived: in.Body.Archived, Tags: in.Body.Tags,
		})
		if err != nil {
			return nil, storeErr(err)
		}
		return &projectOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-project",
		Method:        http.MethodDelete,
		Path:          "/api/projects/{id}",
		Summary:       "Delete a project",
		Tags:          []string{"Projects"},
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *projectIDInput) (*struct{}, error) {
		if err := st.DeleteProject(in.ID); err != nil {
			return nil, storeErr(err)
		}
		return nil, nil
	})
}

// (kept here so other files in the package can use it for now; see tasks.go)
var _ = json.RawMessage(nil)
