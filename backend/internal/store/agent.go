package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Slug shape: kebab-case, lowercased — matches the filename of a plan
// minus `.md`. Validated at the API edge; this regex is the source of truth.
var SlugRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidateSlug returns an error if slug doesn't conform.
func ValidateSlug(slug string) error {
	if !SlugRE.MatchString(slug) {
		return fmt.Errorf("invalid slug %q (expected kebab-case, lowercase)", slug)
	}
	return nil
}

// Slugify converts a free-form display string into a slug suitable for
// addressing — lowercase ASCII alnum + hyphens, no leading/trailing hyphen.
// Returns "" if input has no alphanumerics.
func Slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := byte('-')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b.WriteByte(c)
			prev = c
		case prev != '-':
			b.WriteByte('-')
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}

// uniqueProjectSlug picks a slug that isn't already taken by another project.
// If `base` is empty, falls back to "project". `selfID` is the row being
// updated (0 if inserting); it's excluded from the collision check.
func uniqueProjectSlug(db queryer, base string, selfID int64) string {
	if base == "" {
		base = "project"
	}
	candidate := base
	for n := 2; ; n++ {
		var existingID int64
		err := db.QueryRow(
			`SELECT id FROM project WHERE slug = ? AND id != ?`,
			candidate, selfID,
		).Scan(&existingID)
		if err != nil {
			// sql.ErrNoRows (or any other error) → no collision under this
			// candidate name, take it. Errors from the unique index itself
			// surface later at INSERT time.
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, n)
	}
}

// queryer is the subset of *sql.DB / *sql.Tx we need for slug uniqueness.
type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

// PlanFilter narrows a list-plans request by project and/or text search.
// An empty PlanFilter returns every plan-owned task.
type PlanFilter struct {
	ProjectSlug string
	Query       string
}

// ListPlanTasks returns tasks whose plan_slug is set, newest-first.
// Filters by project_slug (returns ErrNotFound if unknown) and/or by `q`
// (FTS5 match on title + body).
//
// Variadic for backwards compatibility: existing callers pass a single
// project slug string; new callers can pass a PlanFilter.
func (s *Store) ListPlanTasks(filter ...any) ([]Task, error) {
	var f PlanFilter
	if len(filter) > 0 {
		switch v := filter[0].(type) {
		case PlanFilter:
			f = v
		case string:
			f.ProjectSlug = v
		}
	}

	q := `SELECT id, title, body, status, priority, due_text, project_id,
	             sort_order, plan_slug, git_branch, model, effort,
	             created_at, updated_at, completed_at
	        FROM task WHERE plan_slug IS NOT NULL`
	args := []any{}
	if f.ProjectSlug != "" {
		proj, err := s.GetProjectBySlug(f.ProjectSlug)
		if err != nil {
			return nil, err
		}
		q += " AND project_id = ?"
		args = append(args, proj.ID)
	}
	if f.Query != "" {
		q += " AND id IN (SELECT rowid FROM task_fts WHERE task_fts MATCH ?)"
		args = append(args, ftsQuoteQuery(f.Query))
	}
	q += " ORDER BY updated_at DESC, id DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachTags(out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTaskByPlanSlug returns the task with the given slug, or ErrNotFound.
func (s *Store) GetTaskByPlanSlug(slug string) (Task, error) {
	row := s.db.QueryRow(
		`SELECT id, title, body, status, priority, due_text, project_id,
		        sort_order, plan_slug, git_branch, model, effort,
		        created_at, updated_at, completed_at
		   FROM task WHERE plan_slug = ?`, slug,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	ts := []Task{t}
	if err := s.attachTags(ts); err != nil {
		return Task{}, err
	}
	return ts[0], nil
}

// UpsertPlan inserts a new task with the given slug, or patches the existing
// one. Returns (task, created, err). A "create" activity entry is written
// only on first insert. If p.ProjectSlug is set, it's resolved to a
// project_id here (returns ErrNotFound if the project slug is unknown —
// the handler turns that into 422).
func (s *Store) UpsertPlan(slug string, p PlanUpsert) (Task, bool, error) {
	if err := ValidateSlug(slug); err != nil {
		return Task{}, false, err
	}

	// project_slug takes precedence over project_id (slug-first API).
	if p.ProjectSlug != nil {
		trimmed := strings.TrimSpace(*p.ProjectSlug)
		if trimmed == "" {
			p.ClearProjectID = true
			p.ProjectID = nil
		} else {
			proj, err := s.GetProjectBySlug(trimmed)
			if errors.Is(err, ErrNotFound) {
				return Task{}, false, fmt.Errorf("%w: unknown project_slug %q", ErrValidation, trimmed)
			}
			if err != nil {
				return Task{}, false, err
			}
			pid := proj.ID
			p.ProjectID = &pid
			p.ClearProjectID = false
		}
	}

	existing, err := s.GetTaskByPlanSlug(slug)
	if err == nil {
		// Patch path. Translate PlanUpsert into TaskPatch.
		patch := TaskPatch{
			Title:          p.Title,
			Body:           p.Body,
			Priority:       p.Priority,
			DueText:        p.DueText,
			ProjectID:      p.ProjectID,
			ClearProjectID: p.ClearProjectID,
			Tags:           p.Tags,
			GitBranch:      p.GitBranch,
			ClearGitBranch: p.ClearGitBranch,
			Model:          p.Model,
			ClearModel:     p.ClearModel,
			Effort:         p.Effort,
			ClearEffort:    p.ClearEffort,
		}
		updated, err := s.UpdateTask(existing.ID, patch)
		return updated, false, err
	}
	if !errors.Is(err, ErrNotFound) {
		return Task{}, false, err
	}

	// Create path.
	title := slug
	if p.Title != nil && strings.TrimSpace(*p.Title) != "" {
		title = *p.Title
	}
	priority := 0
	if p.Priority != nil {
		priority = *p.Priority
	}
	due := ""
	if p.DueText != nil {
		due = *p.DueText
	}
	body := ""
	if p.Body != nil {
		body = *p.Body
	}
	var projID *int64
	if p.ProjectID != nil {
		projID = p.ProjectID
	}
	tags := []string{}
	if p.Tags != nil {
		tags = *p.Tags
	}
	var gitBranch *string
	if p.GitBranch != nil && !p.ClearGitBranch {
		gitBranch = p.GitBranch
	}
	var model, effort *string
	if p.Model != nil && !p.ClearModel {
		if model, err = normalizeModel(p.Model); err != nil {
			return Task{}, false, err
		}
	}
	if p.Effort != nil && !p.ClearEffort {
		if effort, err = normalizeEffort(p.Effort); err != nil {
			return Task{}, false, err
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO task (title, body, status, priority, due_text, project_id, sort_order, plan_slug, git_branch, model, effort)
		 VALUES (?, ?, 'todo', ?, ?, ?, 0, ?, ?, ?, ?)`,
		title, body, priority, due, projID, slug, nullStr(gitBranch), nullStr(model), nullStr(effort),
	)
	if err != nil {
		return Task{}, false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Task{}, false, err
	}
	if err := setTaskTags(tx, id, tags); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(
		`INSERT INTO activity (task_id, kind, to_status, text)
		 VALUES (?, 'create', 'todo', ?)`,
		id, "claimed plan "+slug,
	); err != nil {
		return Task{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, false, err
	}
	created, err := s.GetTask(id)
	return created, true, err
}

// SetPlanStatus updates a plan-owned task's status. Writes a "status" activity
// entry when the status actually changes, or when note is non-empty.
// Returns ErrNotFound if no task with that slug exists.
func (s *Store) SetPlanStatus(slug, status, note string) (Task, error) {
	if !validStatus[status] {
		return Task{}, fmt.Errorf("%w: invalid status %q", ErrValidation, status)
	}
	existing, err := s.GetTaskByPlanSlug(slug)
	if err != nil {
		return Task{}, err
	}
	changed := existing.Status != status
	if !changed && note == "" {
		return existing, nil
	}
	updated, err := s.UpdateTask(existing.ID, TaskPatch{Status: &status})
	if err != nil {
		return Task{}, err
	}
	if changed || note != "" {
		text := note
		if text == "" {
			text = fmt.Sprintf("%s → %s", existing.Status, status)
		}
		var from *string
		if changed {
			fs := existing.Status
			from = &fs
		}
		ts := status
		if _, aerr := s.db.Exec(
			`INSERT INTO activity (task_id, kind, from_status, to_status, text)
			 VALUES (?, 'status', ?, ?, ?)`,
			existing.ID, nullStr(from), &ts, text,
		); aerr != nil {
			return Task{}, aerr
		}
	}
	return updated, nil
}

// AppendPlanNote writes a free-form 'note' activity entry and bumps the
// task's updated_at. A note is what an agent leaves when the work moved
// without changing column, and the board's "Updated" filter reads
// updated_at — so a plan that only ever gets notes still shows as moving.
// Returns ErrNotFound if the slug is unknown.
func (s *Store) AppendPlanNote(slug, text string) (Activity, error) {
	t, err := s.GetTaskByPlanSlug(slug)
	if err != nil {
		return Activity{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Activity{}, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Exec(
		`INSERT INTO activity (task_id, kind, text) VALUES (?, 'note', ?)`,
		t.ID, text,
	)
	if err != nil {
		return Activity{}, err
	}
	if _, err := tx.Exec(`UPDATE task SET updated_at = datetime('now') WHERE id = ?`, t.ID); err != nil {
		return Activity{}, err
	}
	if err := tx.Commit(); err != nil {
		return Activity{}, err
	}
	id, _ := res.LastInsertId()
	return s.getActivity(id)
}

// ListPlanActivity returns activity entries for a slug, oldest first.
func (s *Store) ListPlanActivity(slug string) ([]Activity, error) {
	t, err := s.GetTaskByPlanSlug(slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, task_id, ts, kind, from_status, to_status, text
		   FROM activity WHERE task_id = ? ORDER BY id`,
		t.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) getActivity(id int64) (Activity, error) {
	row := s.db.QueryRow(
		`SELECT id, task_id, ts, kind, from_status, to_status, text
		   FROM activity WHERE id = ?`, id,
	)
	return scanActivity(row)
}

func scanActivity(r rowScanner) (Activity, error) {
	var a Activity
	var from, to sql.NullString
	if err := r.Scan(&a.ID, &a.TaskID, &a.TS, &a.Kind, &from, &to, &a.Text); err != nil {
		return Activity{}, err
	}
	if from.Valid {
		a.FromStatus = &from.String
	}
	if to.Valid {
		a.ToStatus = &to.String
	}
	return a, nil
}

// nullStr returns a value suitable for passing to db.Exec for a nullable
// TEXT column — either a string or nil.
func nullStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
