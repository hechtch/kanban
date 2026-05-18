# Changelog

All notable changes to this project will be documented here. Format follows
[Keep a Changelog](https://keepachangelog.com).

## [Unreleased]

### Added
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
