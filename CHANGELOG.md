# Changelog

All notable changes to this project will be documented here. Format follows
[Keep a Changelog](https://keepachangelog.com).

## [Unreleased]

### Changed
- **Module path renamed** from `github.com/chrishecht/kanban` to
  `github.com/hechtch/kanban` (matches the GitHub handle the
  module will eventually live under).


### Added
- **OpenAPI 3.1 spec + Swagger UI** — migrated the entire HTTP
  surface to [huma v2](https://github.com/danielgtaylor/huma).
  Each handler is now a typed function `func(ctx, *Input)
  (*Output, error)`; huma generates the OpenAPI spec from the
  signatures at startup, so the spec can't drift from the code.
  - `GET /api/openapi.json` and `GET /api/openapi.yaml` — the spec
  - `GET /api/docs` — Swagger UI (no external CDN deps)
  - PATCH / upsert handlers use a new `Optional[T]` generic that
    tracks `Present` + `Null` so `{"project_id": null}` still
    means "clear" while omitting the field still means "leave
    alone" — same wire semantics as before, now in the spec too.
  - Responses include a `$schema` URL field pointing at the
    response's JSON schema (huma convention; frontends can ignore it).
- **`git_branch` on tasks** — tracks the branch carrying the work.
  Accepted on both `PATCH /api/tasks/:id` and `PUT
  /api/agent/plans/:slug` (clear with `git_branch: null`). Rendered
  as a small monospace chip on the card, below the actor row.
- **`awaiting_merge` status** — new column between Doing and Done
  for work that's been pushed and is waiting for a PR / merge.
  Doesn't auto-transition; flip to `done` when the branch actually
  lands.

### Changed
- **Status `waiting` renamed to `blocked`** — existing rows
  migrated automatically on next start. Old name no longer
  accepted. Task table CHECK constraint dropped at the same time
  (the Go-side `validStatus` map is now the only validator), so
  future status changes don't need a schema rebuild.
- **Board has five columns now**: Todo · Doing · Blocked ·
  Awaiting Merge · Done (Backlog stays off-board).

### Added
- **Agent API for projects + slug-addressable project assignment**
  — `project.slug` (auto-derived from name, unique, partial index)
  lets agents reference projects by slug just like plans. New
  endpoints: `GET /api/agent/projects` (with `plan_count`),
  `GET/PUT /api/agent/projects/:slug`, `GET
  /api/agent/projects/:slug/plans`. Plan upsert now accepts
  `project_slug`; unknown slugs return `422 unknown project_slug
  "X"` rather than silently auto-creating. `GET /api/agent/plans`
  accepts `?project=<slug>` filter. Existing projects backfilled
  on startup via a Go-side `Slugify` of their names. `CLAUDE.md`
  documents the agent workflow for future sessions.
- **Agent API (slim slice)** — slug-addressable `/api/agent/plans/*`
  endpoints so a Claude agent (or any localhost automation) can
  claim a plan, move it across the board, and append activity notes
  without knowing internal task IDs. New `task.plan_slug` column
  (unique partial index on non-NULL slugs), new `activity` table
  (append-only `create`/`status`/`note` entries). Six endpoints:
  `GET /plans`, `GET/PUT /plans/:slug` (upsert with `body`),
  `PUT /plans/:slug/status` (idempotent — writes activity only when
  status changes or a `note` is given), `POST /plans/:slug/notes`,
  `GET /plans/:slug/activity`. Human tasks (no `plan_slug`) are
  invisible to the agent endpoints. Phase 1+2 of
  `plans/agent-api.md`; git-state chips + frontend surface remain.
- **Markdown task bodies** — tasks now carry a free-form markdown
  `body` field rendered on a dedicated drill-down route
  (`/task/:id`). Click a card, press `e`, or hit Enter on a focused
  card to open the big view; metadata (title, status, priority,
  due, project, tags) is inline-editable, and the body has its own
  read/edit toggle (`⌘↵` to save). Lazy-loaded; marked + DOMPurify
  add ~24 kB transfer to the task-view chunk only — initial bundle
  unchanged (315 kB raw / 85 kB transfer).
- **Dev proxy** — `frontend/proxy.conf.json` routes `/api/*` from
  `ng serve` (port 4200) to the Go backend (port 8000). Without
  it, frontend writes silently failed because requests landed on
  Vite and got `index.html` back.
- **Font swap** — headings, due-text, and other hand-display spots
  now use Architects Daughter (paper-ink template default) instead
  of Caveat.
- **Initial plan** — `plans/kanban.md` captures the v0.1 scope:
  4-column board, project-grouped list, three create flows, Angular
  v21 + Go + SQLite, dashboard-integrated under `/apps/kanban/`.
  Visual direction comes from the React wireframe at
  `plans/Kanban Wireframes v2 _standalone_.html`.
- **Project scaffold** — Go backend (`backend/`) with SQLite migrations
  and a `/api/health` route, Angular v21 frontend (`frontend/`) with
  base-href-aware `ApiService` and dashboard bundle budgets, top-level
  `Makefile` + multi-stage `Dockerfile` per the global standards.
- **Backend CRUD** — REST handlers for projects + tasks under
  `/api/*`. `GET /api/tasks` filters on `status`, `project_id`
  (including `null` for Inbox), and `q` substring. Tag set replaced
  atomically inside a transaction; `done` status auto-sets
  `completed_at`. `POST /api/tasks/parse` returns a draft task
  (no persistence) from natural-language input — `!`/`!!`/`!!!`
  priority, `@project`, `#tag`, trailing `by <due-text>` — and
  resolves a matched `@project` name to `project_id`.
- **Angular shell** — header (project name, `← Dashboard` link when
  proxied, view tabs, ⌘N capture button), sidebar (project list +
  per-project counts + Inbox bucket), router-outlet main; lazy
  routes `/board` (default) and `/list`. Inter (body) + Caveat
  (handwritten) loaded via `@fontsource`.
- **Board view** — Angular CDK drag-and-drop with fractional
  `sort_order` so a reorder is a single-row update. Cross-column
  drops also flip status. Cards (roomy variant) show priority
  badges (`!`/`!!`/`!!!`), project color dot + name, due text in
  Caveat, and tags. Right-click / `⋯` opens a quick menu (edit,
  status, priority, delete).
- **List view** — roomy density grouped by project (with an Inbox
  group for tasks without a project); click a row to inline-expand
  a full editor for that task.
- **Three create flows** — full modal dialog (a11y per
  `CLAUDE.angular.md`: `role=dialog`, `aria-modal`,
  `aria-labelledby`, focus trap, Escape closes, focus restored on
  close); inline `+ add task` input at the top of each board
  column; global `⌘N` / `Ctrl+N` natural-language capture with a
  live parsed-fields preview that flags unknown `@project` names.
- **Keyboard** — `↑↓←→` navigate cards, `space` cycle status,
  `1–4` set priority, `e` edit, `delete`/`backspace` remove,
  `⌘N` quick capture from anywhere.
- **Notebook-paper visual style** — cream paper (`#f6f2e8`), ink
  (`#1f2430`), terra-cotta accent (`#d4654a`); Inter for body,
  Caveat cursive for headings and the due-text affordance.
- **Single-binary container** — multi-stage Dockerfile (Angular
  build → Go build → distroless static runtime) embeds the SPA
  into the binary via `//go:embed all:web` behind a `-tags embed`
  build tag, so one process serves UI + API. `make container-run`
  mounts `~/.kanban/data` for SQLite persistence.
- **Dashboard integration** — service entries in
  `~/projects/dashboard/{docker-compose.yml, scripts/up.sh,
  apps.yaml}` for split frontend (`app-kanban`, built with
  `BASE_HREF=/apps/kanban/`) + backend (`app-kanban-backend`,
  built with the shared `go.Dockerfile`).
