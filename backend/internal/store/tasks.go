package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var validStatus = map[string]bool{
	"todo": true, "doing": true, "blocked": true,
	"awaiting_merge": true, "done": true, "backlog": true,
}

// validEffort mirrors Claude Code's reasoning-effort tiers. Model names are
// deliberately NOT enumerated — they churn faster than this code does.
var validEffort = map[string]bool{
	"low": true, "medium": true, "high": true, "xhigh": true, "max": true,
}

// normalizeModel trims and lowercases a suggested model; empty → nil.
func normalizeModel(m *string) (*string, error) {
	if m == nil {
		return nil, nil
	}
	v := strings.ToLower(strings.TrimSpace(*m))
	if v == "" {
		return nil, nil
	}
	if len(v) > 40 {
		return nil, fmt.Errorf("%w: model too long (max 40 chars)", ErrValidation)
	}
	return &v, nil
}

// normalizeEffort trims and lowercases a suggested effort and checks it
// against validEffort; empty → nil.
func normalizeEffort(e *string) (*string, error) {
	if e == nil {
		return nil, nil
	}
	v := strings.ToLower(strings.TrimSpace(*e))
	if v == "" {
		return nil, nil
	}
	if !validEffort[v] {
		return nil, fmt.Errorf("%w: invalid effort %q (low, medium, high, xhigh, max)", ErrValidation, v)
	}
	return &v, nil
}

func (s *Store) ListTasks(f TaskFilter) ([]Task, error) {
	where := []string{}
	args := []any{}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.HasProj {
		if f.ProjectID == nil {
			where = append(where, "project_id IS NULL")
		} else {
			where = append(where, "project_id = ?")
			args = append(args, *f.ProjectID)
		}
	}
	if f.Query != "" {
		where = append(where, "id IN (SELECT rowid FROM task_fts WHERE task_fts MATCH ?)")
		args = append(args, ftsQuoteQuery(f.Query))
	}

	q := `SELECT id, title, body, status, priority, due_text, project_id,
	       sort_order, plan_slug, git_branch, model, effort, created_at, updated_at, completed_at
	       FROM task`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY sort_order, id"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Task{}
	ids := []int64{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.attachTags(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetTask(id int64) (Task, error) {
	row := s.db.QueryRow(
		`SELECT id, title, body, status, priority, due_text, project_id,
		        sort_order, plan_slug, git_branch, model, effort, created_at, updated_at, completed_at
		   FROM task WHERE id = ?`, id,
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

func (s *Store) CreateTask(t Task) (Task, error) {
	if strings.TrimSpace(t.Title) == "" {
		return Task{}, fmt.Errorf("%w: title required", ErrValidation)
	}
	if t.Status == "" {
		t.Status = "todo"
	}
	if !validStatus[t.Status] {
		return Task{}, fmt.Errorf("%w: invalid status %q", ErrValidation, t.Status)
	}
	if t.Priority < 0 || t.Priority > 3 {
		return Task{}, fmt.Errorf("%w: priority must be 0-3", ErrValidation)
	}
	model, err := normalizeModel(t.Model)
	if err != nil {
		return Task{}, err
	}
	effort, err := normalizeEffort(t.Effort)
	if err != nil {
		return Task{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO task (title, body, status, priority, due_text, project_id, sort_order, git_branch, model, effort)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Body, t.Status, t.Priority, t.DueText, t.ProjectID, t.SortOrder,
		nullStr(t.GitBranch), nullStr(model), nullStr(effort),
	)
	if err != nil {
		return Task{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Task{}, err
	}
	if err := setTaskTags(tx, id, t.Tags); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return s.GetTask(id)
}

func (s *Store) UpdateTask(id int64, patch TaskPatch) (Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	sets := []string{}
	args := []any{}
	if patch.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *patch.Title)
	}
	if patch.Body != nil {
		sets = append(sets, "body = ?")
		args = append(args, *patch.Body)
	}
	if patch.Status != nil {
		if !validStatus[*patch.Status] {
			return Task{}, fmt.Errorf("%w: invalid status %q", ErrValidation, *patch.Status)
		}
		sets = append(sets, "status = ?")
		args = append(args, *patch.Status)
		if *patch.Status == "done" {
			sets = append(sets, "completed_at = COALESCE(completed_at, datetime('now'))")
		} else {
			sets = append(sets, "completed_at = NULL")
		}
	}
	if patch.Priority != nil {
		if *patch.Priority < 0 || *patch.Priority > 3 {
			return Task{}, fmt.Errorf("%w: priority must be 0-3", ErrValidation)
		}
		sets = append(sets, "priority = ?")
		args = append(args, *patch.Priority)
	}
	if patch.DueText != nil {
		sets = append(sets, "due_text = ?")
		args = append(args, *patch.DueText)
	}
	if patch.ClearProjectID {
		sets = append(sets, "project_id = NULL")
	} else if patch.ProjectID != nil {
		sets = append(sets, "project_id = ?")
		args = append(args, *patch.ProjectID)
	}
	if patch.SortOrder != nil {
		sets = append(sets, "sort_order = ?")
		args = append(args, *patch.SortOrder)
	}
	if patch.ClearGitBranch {
		sets = append(sets, "git_branch = NULL")
	} else if patch.GitBranch != nil {
		sets = append(sets, "git_branch = ?")
		args = append(args, *patch.GitBranch)
	}
	if patch.ClearModel {
		sets = append(sets, "model = NULL")
	} else if patch.Model != nil {
		model, err := normalizeModel(patch.Model)
		if err != nil {
			return Task{}, err
		}
		sets = append(sets, "model = ?")
		args = append(args, nullStr(model))
	}
	if patch.ClearEffort {
		sets = append(sets, "effort = NULL")
	} else if patch.Effort != nil {
		effort, err := normalizeEffort(patch.Effort)
		if err != nil {
			return Task{}, err
		}
		sets = append(sets, "effort = ?")
		args = append(args, nullStr(effort))
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = datetime('now')")
		args = append(args, id)
		q := `UPDATE task SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
		res, err := tx.Exec(q, args...)
		if err != nil {
			return Task{}, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return Task{}, ErrNotFound
		}
	}

	if patch.Tags != nil {
		if err := setTaskTags(tx, id, *patch.Tags); err != nil {
			return Task{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return s.GetTask(id)
}

func (s *Store) DeleteTask(id int64) error {
	res, err := s.db.Exec(`DELETE FROM task WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTask(r rowScanner) (Task, error) {
	var t Task
	var projID sql.NullInt64
	var planSlug, gitBranch, model, effort, completedAt sql.NullString
	err := r.Scan(&t.ID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.DueText,
		&projID, &t.SortOrder, &planSlug, &gitBranch, &model, &effort,
		&t.CreatedAt, &t.UpdatedAt, &completedAt)
	if err != nil {
		return Task{}, err
	}
	if projID.Valid {
		t.ProjectID = &projID.Int64
	}
	if planSlug.Valid {
		t.PlanSlug = &planSlug.String
	}
	if gitBranch.Valid {
		t.GitBranch = &gitBranch.String
	}
	if model.Valid {
		t.Model = &model.String
	}
	if effort.Valid {
		t.Effort = &effort.String
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.String
	}
	return t, nil
}

// attachTags fills in Tags for each task in place: the task's own tags
// (sorted) followed by any tags its project stamps on that the task
// doesn't already carry. Project tags are merged here, on read, rather
// than copied onto task rows — so dropping a tag from a project drops it
// from every task at once, with nothing to backfill.
func (s *Store) attachTags(tasks []Task) error {
	for i := range tasks {
		tasks[i].Tags = []string{}
	}
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(tasks))
	projSet := map[int64]bool{}
	for _, t := range tasks {
		ids = append(ids, t.ID)
		if t.ProjectID != nil {
			projSet[*t.ProjectID] = true
		}
	}
	own, err := s.linkedTags("task_tag", "task_id", ids)
	if err != nil {
		return err
	}
	projIDs := make([]int64, 0, len(projSet))
	for id := range projSet {
		projIDs = append(projIDs, id)
	}
	inherited, err := s.linkedTags("project_tag", "project_id", projIDs)
	if err != nil {
		return err
	}
	for i := range tasks {
		t := &tasks[i]
		if tags := own[t.ID]; tags != nil {
			t.Tags = tags
		}
		if t.ProjectID == nil {
			continue
		}
		for _, tag := range inherited[*t.ProjectID] {
			if !contains(t.Tags, tag) {
				t.Tags = append(t.Tags, tag)
			}
		}
	}
	return nil
}

// linkedTags reads the tags joined to each id through a link table
// (`task_tag`/`task_id` or `project_tag`/`project_id`), sorted by name.
func (s *Store) linkedTags(table, keyCol string, ids []int64) (map[int64][]string, error) {
	out := map[int64][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := `SELECT l.` + keyCol + `, t.name
	      FROM ` + table + ` l JOIN tag t ON t.id = l.tag_id
	      WHERE l.` + keyCol + ` IN (` + placeholders + `)
	      ORDER BY t.name`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}

// setTaskTags replaces a task's own tags. Tags the task's project already
// stamps on are dropped here: they show up on read anyway (attachTags),
// and keeping them off the task row is what lets a project tag change
// propagate cleanly. Runs after the task row is written so project_id is
// current.
func setTaskTags(tx *sql.Tx, taskID int64, tags []string) error {
	rows, err := tx.Query(
		`SELECT t.name FROM project_tag pt
		   JOIN tag t ON t.id = pt.tag_id
		   JOIN task k ON k.project_id = pt.project_id
		  WHERE k.id = ?`, taskID)
	if err != nil {
		return err
	}
	inherited := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		inherited[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	own := make([]string, 0, len(tags))
	for _, raw := range tags {
		if name := strings.TrimSpace(raw); name != "" && !inherited[name] {
			own = append(own, name)
		}
	}
	return setLinkedTags(tx, "task_tag", "task_id", taskID, own)
}

// setLinkedTags replaces the tag set joined to `id` through a link table,
// creating tag rows as needed. Blank names are skipped.
func setLinkedTags(tx *sql.Tx, table, keyCol string, id int64, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM `+table+` WHERE `+keyCol+` = ?`, id); err != nil {
		return err
	}
	for _, raw := range tags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO tag (name) VALUES (?)`, name); err != nil {
			return err
		}
		var tagID int64
		if err := tx.QueryRow(`SELECT id FROM tag WHERE name = ?`, name).Scan(&tagID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO `+table+` (`+keyCol+`, tag_id) VALUES (?, ?)`, id, tagID,
		); err != nil {
			return err
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
