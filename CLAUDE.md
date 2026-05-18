# Project notes for Claude

Full-stack project; both `~/.claude/CLAUDE.go.md` and
`~/.claude/CLAUDE.angular.md` apply. This file only records the
project-specific bits a fresh session won't infer from the code.

## Source of truth

- **Plan**: `plans/kanban.md` — current checklist + open questions.
  Update in place; check items off as they ship.
- **Visual direction**: `plans/Kanban Wireframes v2 _standalone_.html`.
  Self-bundled artifact; don't try to read raw markup, work from the
  design tokens already in `frontend/src/styles.css`.
- **Aesthetic**: notebook paper. Cream `--paper`, ink `--ink`,
  terra-cotta `--accent`. Inter for body, Architects Daughter
  reserved for headings, the due-text affordance, and similar
  handwritten touches. Don't drift toward generic SaaS chrome.

## Two deployment models — both supported

1. **Standalone single-binary** (`Dockerfile` at repo root). Multi-stage
   build, distroless runtime. The backend embeds the SPA via
   `//go:embed all:web` in `backend/web_embed.go`, gated by
   `//go:build embed`. `backend/web_stub.go` (the `!embed` twin) is
   what dev builds use, so `go run .` never needs a `web/` directory.
   The Dockerfile passes `-tags embed` and copies the Angular dist
   into `backend/web` before `go build`.
2. **Dashboard split-container** in `~/projects/dashboard`. Frontend
   (`app-kanban`) built with the shared `frontend.Dockerfile` and
   `BASE_HREF=/apps/kanban/`; backend (`app-kanban-backend`) built
   with the shared `go.Dockerfile`. Entries in that repo's
   `docker-compose.yml`, `scripts/up.sh`, `apps.yaml`.

The Angular `ApiService` derives its base from `<base href>`, so the
same bundle works in both modes. **Never hardcode `/api` or
`http://localhost:8000/api`** — it'll break under the proxy.

## Key invariants

- **`sort_order` is fractional.** Drag-reorder picks the midpoint
  between neighbors and PATCHes one row. Don't add a "renumber the
  whole column" step — that defeats the point.
- **`PATCH /api/tasks/:id` distinguishes absent vs. null.** The
  handler decodes into `map[string]json.RawMessage` first so
  `project_id: null` (clear) is distinct from omitting the key
  (leave alone). `ClearProjectID bool` on `store.TaskPatch` carries
  that signal through.
- **`status = "done"` auto-sets `completed_at`**; flipping back
  clears it. Lives in `store/tasks.go:UpdateTask`. Don't duplicate
  this in the handler.
- **Tags are replaced, not diffed.** `POST` / `PATCH` with a `tags`
  array replaces the whole set inside a transaction.
- **All routes under `/api/*`.** The proxy depends on this.
- **Data lives outside the tree**: `~/.kanban/data/` (override with
  `KANBAN_DB_PATH`). Never commit `*.db*` files.

## State management

`TaskStore` (signal service) is the single source of truth for tasks
and projects on the frontend. Board, List, and the sidebar all read
from it. Mutations go through `store.patch / create / remove / move`,
which apply optimistic updates and roll back on error. Don't reach
for `HttpClient` directly from a component.

## Adding routes / components

Use Angular's control-flow syntax (`@if`, `@for`, `@empty`) and
standalone components — there are no NgModules here. Lazy-load
top-level views via `loadComponent` in `app.routes.ts` so the board
chunk (CDK is heavy) stays out of the initial bundle.

## Bundle budgets

The dashboard contract is 500 kB warn / 1 MB error initial, 4 kB / 8 kB
per-component style. Configured in `frontend/angular.json`. After any
meaningful UI change run `make build-frontend` and call out chunk-size
deltas in your end-of-task summary. Prefer SVG / hand-rolled visuals
over a chart library.

## Parser

`backend/internal/api/parse.go` strips sigils (`#tag`, `@project`,
`!`/`!!`/`!!!`) BEFORE the trailing `by …` clause, on purpose — the
due clause is greedy to end-of-string and would otherwise swallow a
trailing `#release`. The test cases in `parse_test.go` pin the order;
don't reorder without updating them.

## Testing

- `make test` runs Go + Angular with visible coverage. There's no
  hard threshold, but the number is always shown.
- Backend test helper `newTestStore(t)` uses `t.TempDir()` for an
  isolated SQLite file — use it, don't share a process-wide DB.
- Frontend tests hit `HttpTestingController`; remember to drain the
  initial `/api/projects` and `/api/tasks` calls the `TaskStore`
  constructor fires.

## Out of scope for v0.1

Collaboration, accounts, external-tracker sync, mobile-optimized
layouts, attachments. Don't grow the surface here — add it to
`plans/kanban.md`'s "Open questions" instead so the conversation can
happen on its own.
