package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var validStatus = map[string]bool{
	"todo": true, "doing": true, "waiting": true, "done": true, "backlog": true,
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
		where = append(where, "title LIKE ?")
		args = append(args, "%"+f.Query+"%")
	}

	q := `SELECT id, title, body, status, priority, due_text, project_id,
	       sort_order, created_at, updated_at, completed_at
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

	tagsByTask, err := s.tagsForTasks(ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tags = tagsByTask[out[i].ID]
		if out[i].Tags == nil {
			out[i].Tags = []string{}
		}
	}
	return out, nil
}

func (s *Store) GetTask(id int64) (Task, error) {
	row := s.db.QueryRow(
		`SELECT id, title, body, status, priority, due_text, project_id,
		        sort_order, created_at, updated_at, completed_at
		   FROM task WHERE id = ?`, id,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	tags, err := s.tagsForTasks([]int64{id})
	if err != nil {
		return Task{}, err
	}
	t.Tags = tags[id]
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return t, nil
}

func (s *Store) CreateTask(t Task) (Task, error) {
	if strings.TrimSpace(t.Title) == "" {
		return Task{}, fmt.Errorf("title required")
	}
	if t.Status == "" {
		t.Status = "todo"
	}
	if !validStatus[t.Status] {
		return Task{}, fmt.Errorf("invalid status %q", t.Status)
	}
	if t.Priority < 0 || t.Priority > 3 {
		return Task{}, fmt.Errorf("priority must be 0-3")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO task (title, body, status, priority, due_text, project_id, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Title, t.Body, t.Status, t.Priority, t.DueText, t.ProjectID, t.SortOrder,
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
			return Task{}, fmt.Errorf("invalid status %q", *patch.Status)
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
			return Task{}, fmt.Errorf("priority must be 0-3")
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
	var completedAt sql.NullString
	err := r.Scan(&t.ID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.DueText,
		&projID, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt, &completedAt)
	if err != nil {
		return Task{}, err
	}
	if projID.Valid {
		t.ProjectID = &projID.Int64
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.String
	}
	return t, nil
}

func (s *Store) tagsForTasks(ids []int64) (map[int64][]string, error) {
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
	q := `SELECT tt.task_id, t.name
	      FROM task_tag tt JOIN tag t ON t.id = tt.tag_id
	      WHERE tt.task_id IN (` + placeholders + `)
	      ORDER BY t.name`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		var name string
		if err := rows.Scan(&taskID, &name); err != nil {
			return nil, err
		}
		out[taskID] = append(out[taskID], name)
	}
	return out, rows.Err()
}

func setTaskTags(tx *sql.Tx, taskID int64, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM task_tag WHERE task_id = ?`, taskID); err != nil {
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
			`INSERT OR IGNORE INTO task_tag (task_id, tag_id) VALUES (?, ?)`,
			taskID, tagID,
		); err != nil {
			return err
		}
	}
	return nil
}
