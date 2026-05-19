# Agent-facing API

A thin layer on top of the v0.1 kanban API ([[kanban.md]]) that lets
Claude agents — and any other automation — track plan progress by
moving tickets along the board without ceremony.

## Motivation

Plans live in `plans/<slug>.md`. Each session, a Claude agent picks
up a plan and grinds on it. Today the board has no idea any of that
is happening: tickets are created by hand, status moves by hand, and
the kanban diverges from the actual state of work.

The agent API closes that loop. From inside a session, an agent
should be able to:

- Find (or create) the ticket for the plan it's working on, using
  only the plan's slug.
- Move that ticket between **Todo · Doing · Waiting · Done** by name.
- Leave a short activity note ("started Phase 3", "blocked on
  upstream lib bug") so the board reflects *why* a ticket sits where
  it does.
- Report **git state** for the work — current branch, whether there
  are uncommitted or unpushed changes, and whether the branch has
  been merged into `main` (or a release-candidate branch). These
  surface as chip badges on the card, not as separate columns.

The design goal is **one round-trip per state change, with no prior
knowledge of internal IDs**. An agent that knows its plan slug can
move the ticket; everything else is the server's job.

## Non-goals (v0.1)

- Authentication / per-agent identity. Localhost-only for now; auth
  is a v0.2 concern once this is exposed beyond the loopback.
- Two-way sync between `plans/<slug>.md` checklists and ticket
  sub-items. Interesting, but a separate plan once the basics work.
- Replacing the human CRUD API. The agent endpoints are an
  ergonomic shell over the same tables; nothing the human UI does
  needs to change.
- Webhooks / push notifications back to the agent. Polling is fine
  for v0.1.

## Design principles

1. **Slug-addressable.** Agents reference tickets by plan slug
   (`agent-api`, `kanban`, …), not numeric task ID. The slug is the
   filename of the plan minus `.md`, lowercased.
2. **Idempotent transitions.** `PUT /api/agent/plans/:slug/status`
   with `{status: "doing"}` is safe to retry. Same payload → same
   resulting state, no duplicate side effects.
3. **Upsert, don't fail.** If a slug doesn't have a ticket yet, the
   first call that names it creates one. Agents shouldn't have to
   know whether they're the first to mention a plan.
4. **Status by name, not column index.** The four statuses are a
   stable enum. Columns can be reordered in the UI without breaking
   anything.
5. **Self-describing root.** `GET /api/agent` returns a small
   manifest (allowed statuses, endpoint shapes, server version) so
   a new agent can bootstrap without reading docs.

## Endpoints

All under `/api/agent/*`. JSON in, JSON out. Status codes are
conventional (`200`/`201`/`404`/`409`/`422`).

```
GET    /api/agent
       → { version, statuses: ["todo","doing","waiting","done"],
           endpoints: { ... } }

GET    /api/agent/plans
       → [{ slug, title, status, project, updated_at }, ...]

GET    /api/agent/plans/:slug
       → { slug, title, status, project, priority, due_text,
           tags, git: { branch, uncommitted, unpushed, merged_into },
           activity: [...], task_id }
       404 if no ticket exists yet.

PUT    /api/agent/plans/:slug
       body: { title?, project?, priority?, tags?, due_text? }
       Upsert. Creates the ticket if missing (status defaults to
       "todo"); patches fields if present. Returns the full record.

PUT    /api/agent/plans/:slug/status
       body: { status: "todo"|"doing"|"waiting"|"done", note? }
       Idempotent. Records an activity entry when status changes
       or when `note` is non-empty. Returns the full record.

POST   /api/agent/plans/:slug/notes
       body: { text }
       Append a free-form activity entry without changing status.
       Useful for "still working, ETA today" pings.

GET    /api/agent/plans/:slug/activity
       → [{ ts, kind, from?, to?, text? }]
       kind ∈ {"create","status","note","git"}

PUT    /api/agent/plans/:slug/git
       body: { branch?, uncommitted?, unpushed?, merged_into? }
       Any subset of fields is accepted; omitted fields are left
       unchanged. `merged_into` is a branch name (e.g. "main",
       "rc/v0.6") or null to clear. Writes a "git" activity entry
       only when something actually changed.
```

`PUT /api/agent/plans/:slug` does double duty as "ensure exists"
and "patch metadata" — agents that just want to claim a ticket can
send `{}` as the body.

## Data model additions

Three changes on top of [[kanban.md]]'s schema:

```
task                add columns:
                      plan_slug      TEXT UNIQUE NULL
                      git_branch     TEXT NULL
                      git_uncommitted  INTEGER NOT NULL DEFAULT 0
                      git_unpushed     INTEGER NOT NULL DEFAULT 0
                      git_merged_into  TEXT NULL
                    plan_slug is NULL for human-created tasks; set
                    for agent-owned tickets. Unique so upsert is a
                    single INSERT ... ON CONFLICT.
                    git_* fields are all NULL/0 for human tasks and
                    populated only by agent reports.

activity            id, task_id, ts, kind, from_status, to_status,
                    text
                    kind ∈ {"create","status","note","git"}
                    For "git" entries, `text` is a short rendering
                    of the change (e.g. "branch → feat/foo",
                    "merged into main").
```

`plan_slug` being `NULL`able keeps the human flow untouched: any
task created via the existing `POST /api/tasks` simply has no slug.
The agent endpoints only ever look at rows where `plan_slug IS NOT
NULL`.

Activity entries are append-only. The board UI can surface the most
recent one inline on a card to make agent activity visible.

## Frontend touch-points

Minimal for v0.1, but worth listing so the board doesn't feel
disconnected from the agent layer:

- Card badge when `plan_slug` is set — small "🤖" or "plan" chip,
  using existing CSS vars (no new deps).
- Git-state chips on the card, all CSS-only:
  - **branch name** (always shown when set, e.g. `feat/agent-api`),
  - **uncommitted** dot when `git_uncommitted = 1`,
  - **unpushed** dot when `git_unpushed = 1`,
  - **merged → main** / **merged → rc/v0.6** when
    `git_merged_into` is set.
  Two distinct colors are enough: a warning tone for uncommitted/
  unpushed (work-in-flight) and the accent tone for merged
  (settled). Branch name is neutral.
- Hover/expand surfaces the most recent activity entry.
- Clicking the badge could link to `plans/<slug>.md` on the host;
  defer to v0.2 since rendering that needs a markdown view.

## Phased checklist

### Phase 1 — Schema + slug plumbing
- [x] Add `plan_slug` column to `task` with a unique index allowing
      NULL.
- [ ] Add `git_branch`, `git_uncommitted`, `git_unpushed`,
      `git_merged_into` columns to `task`.
- [x] Add `activity` table + migration.
- [x] `internal/store` helpers: `UpsertPlan`, `SetPlanStatus`,
      `AppendPlanNote`, `GetTaskByPlanSlug`, `ListPlanActivity`,
      `ListPlanTasks`. (`SetGitState` deferred to phase 3.)

### Phase 2 — Agent endpoints
- [ ] `GET /api/agent` manifest.
- [x] `GET /api/agent/plans` + `GET /api/agent/plans/:slug`.
- [x] `PUT /api/agent/plans/:slug` (upsert metadata + body).
- [x] `PUT /api/agent/plans/:slug/status` with activity write.
- [ ] `PUT /api/agent/plans/:slug/git` with activity write on
      changes.
- [x] `POST /api/agent/plans/:slug/notes`.
- [x] `GET /api/agent/plans/:slug/activity`.
- [x] Table tests covering: first-touch upsert, idempotent same-
      status PUT, status transitions write activity, unknown slug
      on `GET` returns 404, invalid status returns 422.
      (Git-PUT test deferred with the git endpoints.)

### Phase 3 — Board surface
- [ ] Card chip when `plan_slug` is set.
- [ ] Git chips: branch name, uncommitted/unpushed dots, merged-
      into badge.
- [ ] Most-recent activity rendered on hover/expand.

### Phase 4 — Agent ergonomics
- [ ] Minimal client snippet documented in `README.md` — three or
      four lines of `curl` or one Python helper showing claim →
      move → note.
- [ ] CHANGELOG `[Unreleased] → Added` entry for the agent API.

## Open questions

- **Slug source of truth.** Slug is just the plan filename, but
  nothing on the server validates that the file exists. Should the
  backend (a) trust the agent, (b) check a configured `plans/`
  directory at startup, or (c) accept any slug but mark "orphan"
  tickets in the UI? Leaning (a) for v0.1, (c) later.
- **Project assignment.** Plans don't currently carry a project.
  Default new agent tickets to a `"plans"` project? Or leave
  `project_id` null and let the UI bucket them under "Unassigned"?
- **Done vs. delivered.** When an agent flips a ticket to `done`,
  should the corresponding plan be auto-moved to `plans/done/`?
  Tempting, but file moves from inside a request handler are
  spooky. Probably surface a "Move plan to done/" suggestion in
  the UI instead.
- **Git state freshness.** Fields go stale the moment an agent
  stops reporting (commits land, branches merge, work moves on).
  v0.1 trusts whatever was last reported; surfacing a "reported N
  minutes ago" timestamp on the chip is a cheap follow-up if stale
  state turns out to be confusing in practice.
- **Merge detection.** `git_merged_into` is agent-reported, not
  derived. Auto-detecting it would mean the server runs `git`
  against a checkout it doesn't own — out of scope. The agent
  setting it (after running `git branch --merged`) is fine.
- **Concurrency.** Two agents touching the same slug simultaneously
  is rare but possible. The unique-slug constraint plus
  `INSERT ... ON CONFLICT` handles upsert; status writes are
  last-write-wins, which is fine — the activity log preserves the
  history.
