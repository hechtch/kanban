# Search & filter

> Delivered in v0.2.0 — 2026-08-28 (PR #1). See CHANGELOG.md. Phases
> 1–3 shipped; the Phase 4 polish items below were explicitly deferred
> and remain unchecked. As of 2026-09-04 they are recorded here only —
> pick them up under a new plan if they matter.

Search across task content (title **and** markdown body) and filter
the board / list views by project. Today the kanban shows everything
on every screen — fine when there are a dozen tasks, breaks down
once plans start accumulating per project and Claude agents fill the
board with multi-step work.

## Motivation

- The `agent-api` work bucket alone is on track for 20+ plans over
  a few weeks. Multiply by a handful of projects and the board
  becomes a wall of cards across five columns with no way to focus.
- Plan bodies are now substantial markdown documents (often kilobytes
  of text). Finding "the task that mentioned the Phase 3 caveat"
  requires opening every card with no leverage.
- The current `GET /api/tasks?q=` substring search is **title-only**
  and case-sensitive — useful for *naming* a task but useless once
  the title has settled and the work is in the body.

## Non-goals (v0.1)

- Cross-instance search (searching plans on a different host).
- Saved searches / filter presets.
- Fuzzy matching beyond what SQLite FTS5 ships out of the box.
- Search across activity notes — those are append-only history,
  not a corpus you typically search; revisit if the use case
  surfaces.

## Design principles

1. **Server-side filtering, client-side rendering.** All filter
   work happens in SQL; the frontend passes parameters and renders
   the result. No fetch-everything-then-filter-client-side.
2. **`?q=` covers title AND body in one parameter.** A single
   search term is the common case. If we later need finer control
   (`?title=` vs `?body=`), add it then; don't pre-build.
3. **Project filter is URL view-state, not a stored preference.**
   Selecting a project sets `?project=<slug>` in the URL; reload
   persists the filter naturally; sharing the URL shares the view.
   `localStorage` is overkill.
4. **The sidebar project list gets a filter input when project
   count crosses a threshold** (~8). Not a search engine — just
   an incremental name match.

## Data model additions

```
task_fts            virtual table (SQLite FTS5)
                    columns: title, body
                    content='task', content_rowid='id'
                    triggers on task INSERT/UPDATE/DELETE keep
                    task_fts in sync with the source rows.
```

No new columns on `task`. The FTS5 table is a mirror; `task` stays
the source of truth.

On migration: backfill `task_fts` from existing `task` rows once,
then leave it alone — triggers handle steady-state.

## API surface

Extend existing routes; no new endpoints.

```
GET /api/tasks?q=foo                  → matches in title OR body
GET /api/tasks?q=foo&project_id=3     → AND project filter
GET /api/tasks?project_id=3           → no search, just project filter
                                        (already supported)
GET /api/agent/plans?q=foo            → same search semantics on
                                        plan-owned rows
GET /api/agent/plans?q=foo&project=p  → AND project (already supports
                                        ?project=<slug>)
```

Match shape: SQLite FTS5 default tokenization (lower-cased, split
on word boundaries, AND semantics across terms). Returns the same
task records the UI already renders. Snippets are a Phase 4
follow-up — not in the initial response shape.

`q=` empty / missing → no search filter applied (current behavior).

OpenAPI spec updates fall out of the typed handler structs; only
the `doc:` strings need a refresh to mention body search.

## Frontend touch-points

Search is its **own tab**, not chrome layered onto the existing
views. Board and List stay focused on their respective layouts
(drag-drop kanban / grouped rows); Search is where you go to find
or filter.

- **New `/search` route** with a new `Search` tab in the header
  nav (between `List` and the capture button). Lazy-loaded
  standalone component, same shape as `/board` and `/list`.
- **Search page layout**:
  - Big text input at the top, autofocused on route entry. Types
    against `?q=` in the URL (debounced 200ms).
  - Below the input: a single-select project filter row
    (`All · Project A · Project B · …`) sets `?project=<slug>` in
    the URL.
  - Below that: a status filter row (`Any · todo · doing ·
    blocked · awaiting_merge · done · backlog`) sets `?status=`.
  - Results render as a flat list of rows (re-use the List
    view's row component if practical) — denser than cards,
    optimized for scanning matches. Each row links to
    `/task/<id>`.
- **URL is the source of view-state.** `?q=`, `?project=`,
  `?status=` round-trip through the URL so a search is shareable
  and survives reload.
- **Global keyboard shortcut.** `⌘F` (and `/` when not in an
  input) navigates to `/search` and focuses the input — works
  from any route. Browser default `⌘F` page-search is fine to
  let go; the kanban's value is in DB search, not DOM search.
- **Sidebar project filter input.** Independent of the Search
  tab: above the project list, a small `Search projects…` input
  appears when project count > 8. Incremental name match, no API
  call — purely client-side filter of the projects signal.
- **Empty / loading / no-results states.** Spell out the active
  filter ("no matches for `foo` in project A") so an empty
  result set isn't confusing.

Board and List **do not** gain an inline search input. Going to
the Search tab is the explicit "I want to find something" gesture.

## Phased checklist

### Phase 1 — Backend FTS
- [x] Add `task_fts` FTS5 virtual table + INSERT/UPDATE/DELETE
      triggers in `store.go` migrations.
- [x] Backfill `task_fts` from existing rows on first run after
      the migration. Idempotent.
- [x] Extend `ListTasks(filter)` to use FTS when `q` is set;
      preserve LIKE-style behavior for empty `q`.
- [x] Extend `ListPlanTasks(slug, q)` similarly (via new
      `PlanFilter{ProjectSlug, Query}`).
- [x] Tests: substring match in body, multi-word AND, case-
      insensitive, FTS index stays in sync across edits and deletes.

### Phase 2 — API
- [x] Update `doc:` strings on `ListTasksInput.Q` and the agent-
      plan equivalent to advertise title+body coverage; verify the
      OpenAPI spec reflects the new wording.
- [x] Confirm `q` + `project_id` / `project_slug` AND together.

### Phase 3 — Frontend
- [x] New `/search` route + `Search` tab in the header nav,
      lazy-loaded standalone component.
- [x] Search page: text input (autofocus, 200ms debounce),
      project filter row, status filter row, results list.
- [x] URL ↔ search inputs two-way sync (`?q=`, `?project=`,
      `?status=` round-trip).
- [x] Global `⌘F` / `/` shortcut: navigate to `/search` from any
      route and focus the input.
- [ ] Sidebar project search input when project count > 8
      (client-side filter, no API). *(Deferred — only ~2 projects
      today; will revisit when the threshold matters.)*
- [x] Empty / no-results state with the active filter spelled
      out ("no matches for `foo` in project A").

### Phase 4 — Polish
- [ ] Highlight the matched substring in card titles (CSS only,
      no JS library).
- [ ] Optional: snippet display under the title when the match is
      in the body, not the title. Server returns a `match_snippet`
      field via FTS5's `snippet()` function.
- [x] CHANGELOG entry.

## Open questions

- **Single-select or multi-select project filter?** Single is
  simpler and matches the existing sidebar UX (one project
  highlighted at a time). Multi is more powerful but needs
  different URL handling (`?project=a,b,c`). Leaning **single**
  for v0.1.
- **Should the existing sidebar still highlight a "filtered"
  project when one is selected on the Search tab?** Probably
  yes — it's a visual cue that the filter is active even when
  you've navigated away from Search. The state is in the URL,
  not the sidebar; the sidebar just reads it.
- **Does `/search` show all tasks when `?q=` is empty?** Two
  options: (a) show everything (essentially "list everything,
  faceted by project/status"), or (b) show a "Type to search…"
  empty state. Leaning (a) so the page is useful as a filter
  even without a query.
- **Snippet display on cards.** When a search matches the body
  but not the title, should the card show "…matched text in
  context…" below the title? Nice UX, separable; deferred to
  Phase 4 unless basically free.
- **Filter persistence on navigation.** If a user filters the
  board to "project A" and switches to the list view, the filter
  should carry over — URL params survive route changes naturally.
  Confirm this works with the existing Angular router setup.
- **Activity / notes search.** Out of scope here. If we find we
  lose useful context (e.g. "which plan did I leave that
  blocked-on-X note on?"), revisit — the activity table is small
  enough that a simple LIKE would do; no FTS needed.
- **Search across plan slugs?** Currently you'd hit
  `GET /api/agent/plans/<slug>` by exact slug. Should `?q=foo`
  also match slug substrings? Lean yes — slugs are part of the
  identity and titles are derived from them at creation time, so
  matching on slug is intuitive.
