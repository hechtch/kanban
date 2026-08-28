package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/hechtch/kanban/backend/internal/store"
)

// Register wires every HTTP route the kanban exposes onto the given mux,
// using huma for typed handlers + OpenAPI schema generation. The spec is
// served at /api/openapi.json (and .yaml), the Swagger UI at /api/docs.
func Register(mux *http.ServeMux, st *store.Store) {
	config := huma.DefaultConfig("Kanban API", "0.1.0")
	config.Info.Description = "API for the kanban task tracker. " +
		"Routes under `/api/*`. The `/api/agent/*` subtree is the " +
		"slug-addressable API for Claude agents and other automation " +
		"to track plan progress without knowing internal IDs."
	config.DocsPath = "/api/docs"
	config.OpenAPIPath = "/api/openapi"
	config.SchemasPath = "/api/schemas"

	api := humago.New(mux, config)

	registerSystem(api)
	registerProjects(api, st)
	registerTasks(api, st)
	registerAgentPlans(api, st)
	registerAgentProjects(api, st)
}

// ─── system / health ────────────────────────────────────────────────────

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func registerSystem(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/api/health",
		Summary:     "Liveness check",
		Tags:        []string{"System"},
	}, func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

// ─── error helpers ──────────────────────────────────────────────────────

// storeErr translates store package errors into huma error types so the
// generated OpenAPI spec advertises the right status codes for each
// operation.
func storeErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return huma.Error404NotFound(err.Error())
	}
	if errors.Is(err, store.ErrValidation) {
		return huma.Error422UnprocessableEntity(err.Error())
	}
	return huma.Error500InternalServerError("internal error", err)
}

// rawJSON is a marker type for endpoints whose body is decoded manually
// (used by the PATCH/upsert handlers that need absent-vs-null distinction).
// huma documents these as accepting an arbitrary JSON object.
type rawJSON map[string]json.RawMessage
