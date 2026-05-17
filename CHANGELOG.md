# Changelog

## [Unreleased]

### Added
- **Initial plan** — `plans/kanban.md` captures the v0.1 scope:
  4-column board, project-grouped list, three create flows, Angular
  v21 + Go + SQLite, dashboard-integrated under `/apps/kanban/`.
  Visual direction comes from the React wireframe at
  `plans/Kanban Wireframes v2 _standalone_.html`.
- **Project scaffold** — Go backend (`backend/`) with SQLite migrations
  and a `/api/health` route, Angular v21 frontend (`frontend/`) with
  base-href-aware `ApiService` and dashboard bundle budgets, top-level
  `Makefile` + multi-stage `Dockerfile` per the global standards.
