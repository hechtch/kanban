# Kanban work tracker

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
- [ ] CRUD for projects + tasks; `GET /api/tasks` filtering
- [x] `golangci-lint` config + `go test ./...` with visible coverage

### Phase 2 — Angular shell
- [x] `ng new` v21, strict templates *(test runner: Angular v21's
      native `@angular/build:unit-test` (Vitest) rather than Jest —
      diverges from `~/.claude/CLAUDE.angular.md` which predates v20;
      revisit if Jest is strictly required)*
- [x] `<base href>`-derived API base in a single service
- [ ] App shell: header (project name, `← Dashboard` link when
      proxied), sidebar (project list + counts), main area
- [ ] Routing: `/board` (default), `/list`
- [x] Live region `<div role="status" aria-live="polite">` in shell
- [x] Bundle budgets configured per dashboard rule
- [~] Notebook-paper look: cream `#f6f2e8`, ink `#1f2430`, coral
      `#d4654a` set as CSS vars; Inter + Caveat font wiring still
      pending

### Phase 3 — Board view
- [ ] 4 columns rendered from a fixed enum; cards from `/api/tasks`
- [ ] Card component (roomy variant from wireframe)
- [ ] Drag-and-drop reorder + cross-column status change (Angular CDK)
- [ ] Quick-edit popover: right-click or `⋯` menu
- [ ] Keyboard: arrow navigation, `space`, `e`, `1–4`

### Phase 4 — Create flows
- [ ] Modal dialog (a11y per `CLAUDE.angular.md`: role/aria-modal/
      labelledby, focus trap, Escape closes)
- [ ] Inline quick-add at the top of each column
- [ ] `⌘N` natural-language capture → `POST /api/tasks/parse` →
      preview → confirm. Parser is intentionally simple at first
      (regex for `!`/`!!`/`!!!`, `@project`, `#tag`, trailing
      `by <due-text>`)

### Phase 5 — List view
- [ ] Roomy density grouped by project
- [ ] Click row → inline expand for editing (single pattern, not
      three)

### Phase 6 — Polish & ship
- [ ] Dockerfile + Makefile per global standards (`run`, `run-bg`,
      `build`, `test`, `lint`, `clean`, `container-*`)
- [ ] Append service to dashboard `docker-compose.yml` + `up.sh`
- [ ] `CHANGELOG.md` with `[Unreleased] → Added` block populated
- [ ] Smoke-test the `⌘N` flow end-to-end in a browser
- [ ] Tag `v0.1.0` and move this file to `plans/done/kanban.md`

## Open questions

- Drag library: Angular CDK `DragDropModule` is the obvious pick;
  worth a 30-min check that it handles column→column moves with
  the fractional-index sort cleanly.
- Should `Waiting` cards surface a "waiting on" string separate
  from tags? The wireframe shows a `blocked` tag — keeping it as a
  tag for v0.1 is simpler; revisit if it feels weak in practice.
- Is the Backlog 5th column worth shipping in v0.1, or defer
  until after the core loop feels right?
