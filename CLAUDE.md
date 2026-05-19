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

## Agent API — how Claude should use this kanban

The kanban is **API-first for Claude agents**: it's a place to keep
plan state alive across sessions. When you're working on a plan in
this fleet, you should be talking to the kanban as you go.

> **Keep `~/.claude/CLAUDE-kanban.md` in sync.** That file is the
> brief other Claude sessions read to learn this API. Any time you
> change endpoints, payload shape, the status enum, slug rules, or
> the recommended workflow here, update that file in the same
> commit — otherwise agents in other projects will be working from
> a stale spec.

### Base URL

- **Local dev / single-binary**: `http://localhost:8000/api/agent/*`.
- **Behind the dashboard** (the primary deployment): the proxy
  rewrites `/apps/kanban/api/*` → backend. Use the dashboard URL
  rather than localhost when one is available.

### Slug-addressable, not ID-addressable

Everything you touch from an agent is keyed by **slug**, not by
numeric ID. You should never see or store a `task.id` /
`project.id` in agent code.

- **Plan slug** = the plan filename minus `.md`, lowercased
  (`plans/agent-api.md` → `agent-api`).
- **Project slug** = auto-derived from the project name on create
  (`"Plan Tracking"` → `tracking`). The server picks the slug; you
  read it back from the response.
- Slug shape (enforced server-side): `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`.

### Canonical content lives in the DB

The task `body` field IS the plan. `plans/<slug>.md` is a **seed**
(used once when you first claim a plan) and `plans/done/<slug>.md`
is the **archival snapshot** written when the work ships. Between
those two moments, edits go to the DB, not the file. Don't sync
back to the file mid-flight.

### Workflow per session

1. **Claim** the plan by slug. `PUT /api/agent/plans/<slug>` with
   `{}` is enough — first call creates a `todo` task and writes a
   `create` activity entry; subsequent calls patch it.
2. If this is the first claim and `plans/<slug>.md` exists on disk,
   read it and include it as `body` in the upsert so the DB picks
   up the existing text.
3. **Set the project** in the same upsert: `{"project_slug":
   "tracking"}`. Unknown project slugs return `422` — create the
   project first with `PUT /api/agent/projects/<slug>` if needed.
4. **Move to doing** when you start: `PUT
   /api/agent/plans/<slug>/status` with `{"status":"doing","note":
   "starting phase N"}`. Status flips are idempotent — re-posting
   the same status without a note is a no-op.
5. **Leave notes** as you go: `POST /api/agent/plans/<slug>/notes`
   with `{"text":"blocked on X"}`. Cheaper than a status flip and
   shows up in the activity log.
6. **Flip to done** when shipping; if the plan is fully delivered,
   also write `plans/done/<slug>.md` from the current task body so
   the archival convention is preserved.

### Canonical spec

The API ships an **OpenAPI 3.1 spec** generated by huma from the
typed handler signatures — that's the source of truth, not the
endpoint list here. URLs (dashboard deployment):

- `http://localhost:8080/apps/kanban/api/openapi.json` — JSON spec
- `http://localhost:8080/apps/kanban/api/openapi.yaml` — YAML spec
- `http://localhost:8080/apps/kanban/api/docs` — Swagger UI

When changing endpoints, the typed input/output structs in
`backend/internal/api/*.go` ARE the contract — huma regenerates
the spec at startup, so there's no separate spec to keep in sync.
You DO still need to update `~/.claude/CLAUDE-kanban.md` whenever
the agent-facing surface changes (see top of this section).

### Endpoints in one screen

```
GET    /api/agent/plans                      list (?project=<slug> filter)
GET    /api/agent/plans/<slug>               { slug, task, activity[] }
PUT    /api/agent/plans/<slug>               upsert (title, body, priority,
                                             due_text, project_slug, tags)
PUT    /api/agent/plans/<slug>/status        { status, note? }
POST   /api/agent/plans/<slug>/notes         { text }
GET    /api/agent/plans/<slug>/activity      timeline (oldest first)

GET    /api/agent/projects                   list w/ plan_count
GET    /api/agent/projects/<slug>            single
PUT    /api/agent/projects/<slug>            upsert (name, color, sort_order)
GET    /api/agent/projects/<slug>/plans      same as /plans?project=<slug>
```

### Conventions

- Status enum is fixed: `todo`, `doing`, `blocked`,
  `awaiting_merge`, `done`, `backlog`. Don't invent new ones —
  the column is the column.
- `git_branch` is a free-form string on the task; agents set it
  to track which branch carries the work. Renders as a monospace
  chip on the card.
- `body` is markdown, rendered at `/task/<task_id>` in the UI. The
  rendered view has its own edit toggle, so a human can fix typos
  without going through the API.
- An agent task is identified by `task.plan_slug != null`. Human
  tasks created via the regular `/api/tasks` POST do not appear in
  `/api/agent/plans` — that filter is intentional.

### What still lives outside this API

- **Git state** chips (`branch`, `uncommitted`, `unpushed`,
  `merged_into`) are still on the `plans/agent-api.md` roadmap, not
  shipped. Don't depend on them yet.
- **Frontend chip** indicating a card is plan-owned isn't drawn
  yet. The route works; the visual cue doesn't.

## Out of scope for v0.1

Collaboration, accounts, external-tracker sync, mobile-optimized
layouts, attachments. Don't grow the surface here — add it to
`plans/kanban.md`'s "Open questions" instead so the conversation can
happen on its own.
