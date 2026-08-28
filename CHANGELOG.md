# Changelog

All notable changes to this project will be documented here. Format follows
[Keep a Changelog](https://keepachangelog.com).

## [Unreleased]

### Changed
- **Ticket opens as a big modal over the board** — clicking a card
  (or a search result, or pressing `e`/Enter on a focused card) now
  opens the ticket in a large dialog over the current view instead
  of navigating away, so the board stays visible behind it. The
  header and project sidebar stay live: picking a project while a
  ticket is open closes it and filters the board underneath. Close
  with Esc, the ×, or a click on the dimmed area. The ticket is
  addressed by `?task=<id>` on the current view (`/board?task=5`),
  so it's still a deep link; the older `/task/<id>` URLs redirect.
- **Module path renamed** from `github.com/chrishecht/kanban` to
  `github.com/hechtch/kanban` (matches the GitHub handle the
  module will eventually live under).


### Fixed
- **Browsers no longer hold a stale bundle after a redeploy** — the
  SPA's `index.html` is now served with `Cache-Control: no-cache`
  (hashed chunks get a year-long `immutable`), in both the standalone
  binary and the dashboard's shared frontend image. Previously
  `index.html` had no cache header, so Firefox kept replaying an old
  copy — and the old chunks it named — for tens of minutes after each
  deploy.
- **Validation errors are 422, not 500** — a bad `status`, `priority`,
  `effort`, or over-long `model` on `PATCH /api/tasks/:id` (and the
  agent status endpoint) now returns `422` with the reason, instead
  of a generic `500`. The store tags these with `ErrValidation`.
- **Every delete asks first** — the board's card menu, the Delete /
  Backspace key on a focused card, and the list editor's Delete
  button now confirm before removing a task, matching the ticket
  view. Previously a stray Backspace on a focused card deleted it
  silently, with no undo.

### Added
- **Archive a finished project** — a project can be marked archived
  from the sidebar's new project editor (or by an agent, with
  `{"archived":true}` on the project upsert). It folds into an
  "archived" section at the bottom of the sidebar and its tasks drop
  off the Board and List, without deleting anything: expand the
  section and click the project and its tasks come back.
- **Project default tags** — a project can carry tags that every
  ticket in it inherits, so all *2026 Taxes* tickets read as `#tax`
  without tagging each one. The server merges them on read, so
  dropping a tag from the project drops it from every ticket at once;
  in the ticket they show as locked chips beside the editable tags.
- **Project editor** — projects were API-only; the sidebar now has a
  `+` to create one and a pencil on each row to rename it, change its
  colour, set its default tags, archive it, or delete it (its tasks
  move to Inbox).
- **Clickable checkboxes in notes** — Markdown task-list items
  (`- [ ]` / `- [x]`) in the ticket's Notes render as live checkboxes.
  Clicking one (or focusing it and pressing Space) flips the matching
  line in the body and saves, so a plan's checklist can be worked off
  from the ticket without opening the editor. Checked items dim;
  task syntax inside code blocks stays inert.
- **Version in the header** — the app name now reads `Kanban v0.1.0`,
  with the version baked in from `frontend/package.json` at build
  time so the header names what's actually deployed. First real
  semver for the project (was `0.0.0`).
- **Suggested model & effort per task** — new optional `model` and
  `effort` fields (e.g. `fable` / `xhigh` for a security audit,
  `sonnet` / `high` for mundane work) as a hint for whichever agent
  picks the task up. Set them from two selects in the ticket modal;
  they show as a `fable / xhigh` chip on the card. Exposed on
  `POST`/`PATCH /api/tasks` and the agent upsert
  (`PUT /api/agent/plans/<slug>`); `effort` is validated against
  `low / medium / high / xhigh / max`, `model` is free-form.
- **Multi-select projects in the sidebar** — Ctrl/⌘-click adds or
  removes a project from the filter, Shift-click selects a range
  (Inbox counts as the last row). Each selected project is its own
  removable pill above the board. With more than one project
  selected, new tasks aren't defaulted into any of them.
- **Project filter from the sidebar** — click a project (or *Inbox*)
  in the left panel and the Board and List show only that project's
  tasks; click it again or *All* to clear. A "showing ● Project ×
  show all" chip sits above the columns so the filter is visible
  even with the sidebar collapsed. The selection persists across
  reloads, and tasks created while filtered (quick-add, new-task
  modal, ⌘N capture without an `@project`) land in the selected
  project instead of vanishing into Inbox. Picking any sidebar row
  while reading a ticket closes the ticket and lands on the Board
  with that filter applied; from Search it jumps to the Board.
- **Tag filter across projects** — the sidebar now lists every tag in
  use with a count (`#finance 5`), most-used first. Click one and the
  Board and List show only tasks carrying it, whichever project they
  belong to; combine it with a project to narrow further. Each active
  filter is its own removable pill above the views. Tasks created
  while a tag is selected get that tag automatically.
- **Search across title + body** — `?q=foo` on `GET /api/tasks`
  and `GET /api/agent/plans` now hits a SQLite FTS5 index covering
  both `task.title` and `task.body`. Multi-word queries are AND'd
  (`?q=phase%203` finds tasks mentioning both terms anywhere). The
  old title-only `LIKE` behavior is gone — bodies are usually
  where the interesting words live now. The FTS5 virtual table is
  kept in sync via triggers on `task` INSERT/UPDATE/DELETE and is
  backfilled from existing rows on first migration.
- **`/search` tab in the UI** — new lazy-loaded route between
  *List* and the quick-capture button. Big text input (autofocus,
  200ms debounce), project filter chips, status filter chips,
  results as a flat scannable list. State lives in the URL
  (`?q=`, `?project=`, `?status=`), so a search is shareable and
  survives reload. `⌘F` or `/` from anywhere navigates to
  `/search` and focuses the input.
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
