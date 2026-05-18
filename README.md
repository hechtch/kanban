# Kanban

Personal task tracker — notebook-paper aesthetic, four fixed status
columns, project-grouped list view, and a `⌘N` natural-language quick
capture. Built as a single Angular SPA + Go API backed by SQLite. Runs
standalone or behind the dashboard reverse proxy at `/apps/kanban/`.

Visual direction lives in
[`plans/Kanban Wireframes v2 _standalone_.html`](plans/Kanban%20Wireframes%20v2%20_standalone_.html);
scope and design notes in [`plans/kanban.md`](plans/kanban.md).

## Quickstart

```sh
make install          # npm install + go mod tidy
make run              # backend on :8000, frontend on :4200 (parallel)
```

Then open <http://localhost:4200>. Tasks persist to SQLite at
`~/.kanban/data/kanban.db` (override with `KANBAN_DB_PATH`).

Other useful targets:

| Target | What |
|---|---|
| `make run-bg` | Backend in background (logs in `/tmp/kanban-backend.log`), frontend in foreground |
| `make build` | Production Angular bundle + `backend/bin/kanban` |
| `make test` | Go + Angular tests with visible coverage |
| `make container-build` | Single-binary image (`kanban:latest`) — embeds the SPA via `-tags embed` |
| `make container-run` | Run that image with `~/.kanban/data` mounted for persistence |

`CONTAINER_RUNTIME` defaults to `docker`; pass
`CONTAINER_RUNTIME=podman` to use podman.

## Using it

- **Board** (`/board`, default) — four columns Todo · Doing · Waiting ·
  Done. Drag cards within or across columns; cross-column drops also
  flip status. `↑↓←→` navigate, `space` cycles status, `1–4` set
  priority, `e` opens edit, `delete` removes, right-click / `⋯` opens
  a quick menu.
- **List** (`/list`) — rows grouped by project (with an Inbox bucket
  for tasks without a project). Click a row to inline-expand the full
  editor.
- **⌘N capture** (or `Ctrl+N`, or the header button) — type
  `email landlord by friday !! @admin #ping`; the preview shows the
  parsed title / priority / due / project / tags before you commit.

## Deployment models

Two complementary setups, both supported:

1. **Single binary** (`Dockerfile` at the repo root). Multi-stage
   build: Angular → Go → distroless static. The Go binary embeds the
   SPA at `web/frontend/browser` via `//go:embed all:web`, gated by
   `//go:build embed`, so one process serves UI + API on `:8000`.
   `go run .` in dev skips the embed (no `web/` required).
2. **Dashboard reverse proxy** at
   [`~/projects/dashboard`](../dashboard). Frontend and backend run
   as separate containers (`app-kanban` via the shared
   `frontend.Dockerfile` with `BASE_HREF=/apps/kanban/`,
   `app-kanban-backend` via the shared `go.Dockerfile`). Service
   entries live in that repo's `docker-compose.yml`, `scripts/up.sh`,
   and `apps.yaml`.

The Angular `ApiService` derives its base URL from `<base href>`, so
the same bundle works in both modes.

## API surface

```
GET    /api/health
GET    /api/projects                          POST   /api/projects
PATCH  /api/projects/:id                      DELETE /api/projects/:id
GET    /api/tasks  ?status=&project_id=&q=    POST   /api/tasks
POST   /api/tasks/parse                       PATCH  /api/tasks/:id
                                              DELETE /api/tasks/:id
```

`PATCH /api/tasks/:id` is the workhorse — drag-reorder, status flips,
priority changes, inline edits all hit it. `project_id: null` clears
the project; omitting the field leaves it alone. `POST /api/tasks/parse`
returns a draft task (no persistence) so the ⌘N flow can preview
before committing.

## Layout

```
backend/      Go API (modernc.org/sqlite, pure-Go, no CGO)
frontend/     Angular v21 SPA — board, list, modal, capture
plans/        Living plan docs + the v2 wireframe HTML
Dockerfile    Single-binary embed build
Makefile      install / run / build / test / container-*
```

See [`CHANGELOG.md`](CHANGELOG.md) for what's in each release.
