# Kanban work tracker

> Delivered in v0.2.0 — 2026-08-28. See CHANGELOG.md. (0.1.0 was the
> versioning-adoption bump and was never tagged; the whole v0.1 scope
> plus the sidebar / ticket-modal follow-ups shipped together as 0.2.0.)

Personal Kanban app for tracking work across small projects. Visual
direction is locked by `plans/Kanban Wireframes v2 _standalone_.html` —
notebook-paper aesthetic, four fixed columns, three create modes,
project-grouped list view.

## Goals

- Track tasks across **4 fixed status columns**: Todo · Doing ·
  Waiting · Done. Optional 5th column (Backlog) is a per-board
  toggle, not a separate variant.
- Each task carries: title, priority (0–3, with `!`/`!!`/`!!!`
  affordance), tags, due date (free-form short string like `today`,
  `fri`, `next wk`), and a project assignment.
- Two primary views: **Board** (kanban columns) and **List** (rows
  grouped by project).
- Three ways to create a task: modal dialog, inline quick-add in a
  column header, and natural-language `⌘N` capture (e.g. *"email
  landlord about the leak by friday !! @admin #ping"*).
- Keyboard-first: `↑↓←→` navigate, `space` cycle status, `e` edit,
  `1–4` priority, `⌘N` new.

Non-goals for v0.1: collaboration, accounts, sync to external
trackers, mobile-optimized layout, attachments.

## Stack

- **Frontend**: Angular v21 (per `~/.claude/CLAUDE.angular.md`).
  Mounted under `/apps/kanban/` behind the dashboard reverse proxy.
- **Backend**: Go, listening on `:8000`, all routes under `/api/*`
  (per `~/.claude/CLAUDE.go.md` and the dashboard contract).
- **Storage**: SQLite at `~/.kanban/data/kanban.db`, override via
  `KANBAN_DB_PATH`. Never in the repo tree.
- **Container**: Multi-stage Dockerfile (Angular build → Go build →
  minimal runtime image). Bind-mount `~:/host:ro,Z` is not needed
  for v0.1 (no filesystem browsing).

## Dashboard integration

- API base derives from `<base href>`:
  `(document.querySelector('base')?.getAttribute('href') ?? '/').replace(/\/+$/, '') + '/api'`.
- Header renders a `← Dashboard` link when `<base href>` starts
  with `/apps/`.
- Append a service to `~/projects/dashboard/docker-compose.yml`
  *and* a `run_container` line to `~/projects/dashboard/scripts/up.sh`.
- Bundle budgets: 500 kB warn / 1 MB error initial; 4 kB / 8 kB
  per-component style. Reach for SVG before any chart library.

## Data model (initial sketch)

```
project    id, name, color, sort_order
task       id, title, status (todo|doing|waiting|done|backlog),
           priority (0|1|2|3), due_text, project_id, sort_order,
           created_at, updated_at, completed_at
tag        id, name
task_tag   task_id, tag_id     (many-to-many)
```

Sort order within a column/project is a float-or-fractional-index
so drag-reorder is a single-row update. Tags are free-form: typing
a new tag creates it; no separate tag-admin UI.

`due_text` stays a short opaque string in v0.1 (matches the
wireframe's *today / fri / next wk / —*). Real date parsing is a
follow-up; it's the natural extension of the `⌘N` natural-language
parser.

## API surface (v0.1)

```
GET    /api/projects
POST   /api/projects
PATCH  /api/projects/:id
DELETE /api/projects/:id

GET    /api/tasks                 ?status=&project_id=&q=
POST   /api/tasks
PATCH  /api/tasks/:id             partial update (status, priority,
                                  due_text, project_id, title, tags,
                                  sort_order)
DELETE /api/tasks/:id
POST   /api/tasks/parse           body: {text: string} → parsed
                                  draft task (no persistence)
```

`PATCH` is the workhorse — drag-reorder, status flip from `space`,
priority from `1–4`, and inline edits all hit it.

## Phased checklist

### Phase 1 — Backend skeleton
- [x] `go.mod` + `main.go` listening on `:8000`, routes under `/api/*`
- [x] SQLite via `modernc.org/sqlite` (pure-Go, no CGO)
- [x] Migrations baked in: run on startup, idempotent
- [x] `internal/store` package owns DB access; handlers are thin
- [x] CRUD for projects + tasks; `GET /api/tasks` filtering
- [x] `golangci-lint` config + `go test ./...` with visible coverage

### Phase 2 — Angular shell
- [x] `ng new` v21, strict templates *(test runner: Angular v21's
      native `@angular/build:unit-test` (Vitest) rather than Jest —
      diverges from `~/.claude/CLAUDE.angular.md` which predates v20;
      revisit if Jest is strictly required)*
- [x] `<base href>`-derived API base in a single service
- [x] App shell: header (project name, `← Dashboard` link when
      proxied), sidebar (project list + counts), main area
- [x] Routing: `/board` (default), `/list`
- [x] Live region `<div role="status" aria-live="polite">` in shell
- [x] Bundle budgets configured per dashboard rule
- [x] Notebook-paper look: cream `#f6f2e8`, ink `#1f2430`, coral
      `#d4654a` set as CSS vars; Inter + Caveat fonts loaded via
      `@fontsource/{inter,caveat}`

### Phase 3 — Board view
- [x] 4 columns rendered from a fixed enum; cards from `/api/tasks`
- [x] Card component (roomy variant from wireframe)
- [x] Drag-and-drop reorder + cross-column status change (Angular CDK)
      — fractional `sort_order` midpoint so a drop is one PATCH
- [x] Quick-edit popover: `⋯` menu (status / priority / delete);
      full edit dialog deferred to Phase 4
- [x] Keyboard: arrow nav, `space` cycle status, `1–4` priority,
      `delete`/`backspace` delete. `e` to open edit modal lands in
      Phase 4 with the modal itself.

### Phase 4 — Create flows
- [x] Modal dialog (a11y per `CLAUDE.angular.md`: role/aria-modal/
      labelledby, focus trap, Escape closes). Shared by new + edit
      flows; opened on `e` from a focused card.
- [x] Inline quick-add at the top of each column
- [x] `⌘N` natural-language capture → `POST /api/tasks/parse` →
      preview → confirm. Parser regex: `!`/`!!`/`!!!`, `@project`,
      `#tag`, trailing `by <due-text>`. Project name resolves to
      `project_id` server-side when it matches an existing project.

### Phase 5 — List view
- [x] Roomy density grouped by project
- [x] Click row → inline expand for editing (single pattern, not
      three)

### Phase 6 — Polish & ship
- [x] Dockerfile + Makefile per global standards (`run`, `run-bg`,
      `build`, `test`, `lint`, `clean`, `container-*`). Backend now
      uses `//go:embed all:web` behind a `-tags embed` build tag so
      the container ships UI + API in one binary while `go run .`
      in dev stays cheap (no `web/` required).
- [x] Append service to dashboard `docker-compose.yml`, `up.sh`,
      and `apps.yaml`. Dashboard uses the split-container pattern
      (`app-kanban` via shared `frontend.Dockerfile` with
      `BASE_HREF=/apps/kanban/`; `app-kanban-backend` via shared
      `go.Dockerfile`). The kanban-tree Dockerfile remains for
      standalone single-binary deploys.
- [x] `CHANGELOG.md` with `[Unreleased] → Added` block populated
- [ ] Smoke-test the `⌘N` flow end-to-end in a browser
      *(user action — never recorded as done; left unchecked when
      this plan was archived on 2026-09-04)*
- [x] Tag the release and move this file to `plans/done/kanban.md`
      *(tagged `v0.2.0` on 2026-09-04 — 0.1.0 was never released)*
- [x] Sidebar project filter: clicking a project / Inbox narrows Board
      and List to it; persisted in localStorage; filter chip above the
      views; new tasks default into the selected project
- [x] Sidebar tag filter: tags in use listed with counts; a tag spans
      projects and ANDs with the project filter; new tasks inherit the
      selected tag
- [x] Ticket view is a big modal over the main pane (`?task=<id>`);
      `/task/<id>` redirects; sidebar/header stay usable behind it
- [x] Sidebar multi-select (Ctrl/⌘-click toggle, Shift-click range);
      sidebar click on Search jumps to the Board
- [x] Per-task `model` / `effort` hints for agents (card chip, ticket
      selects, `/api/tasks` + agent upsert, `kanban-plans` skill updated)
- [x] Markdown checkboxes in the ticket view are clickable — toggling
      one rewrites the matching `[ ]`/`[x]` in the body and saves
- [x] Hide (archive) a project once its work is done so it drops out
      of the sidebar and filters without deleting its tickets
- [x] Project-level default tags: tags attached to a project are
      applied to every ticket in it (e.g. all *2026 Taxes* tickets
      carry `#tax`)
- [x] Project editor in the sidebar (name, colour, default tags,
      archive, delete) — projects used to be API-only
- [x] Assignee filter (you / Claude) in the sidebar, derived from
      plan ownership rather than a stored field

## Open questions (resolved)

- **Drag library** — Angular CDK `DragDropModule` shipped and handles
  column→column moves cleanly with the fractional `sort_order` (one
  PATCH per drop; see the invariants in `CLAUDE.md`).
- **"Waiting on" for Waiting cards** — `waiting` became the `blocked`
  status (renamed in a migration); the reason lives in a note or tag,
  no separate field.
- **Backlog column** — shipped as an off-board sixth status; the board
  itself shows five columns (Todo · Doing · Blocked · Awaiting merge ·
  Done).
