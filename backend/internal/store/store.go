package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS project (
			id          INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			color       TEXT NOT NULL DEFAULT '#1f2430',
			sort_order  REAL NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS task (
			id            INTEGER PRIMARY KEY,
			title         TEXT NOT NULL,
			body          TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL CHECK (status IN ('todo','doing','waiting','done','backlog')),
			priority      INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 3),
			due_text      TEXT NOT NULL DEFAULT '',
			project_id    INTEGER REFERENCES project(id) ON DELETE SET NULL,
			sort_order    REAL NOT NULL DEFAULT 0,
			created_at    TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at  TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tag (
			id    INTEGER PRIMARY KEY,
			name  TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS task_tag (
			task_id  INTEGER NOT NULL REFERENCES task(id) ON DELETE CASCADE,
			tag_id   INTEGER NOT NULL REFERENCES tag(id)  ON DELETE CASCADE,
			PRIMARY KEY (task_id, tag_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_status ON task(status, sort_order)`,
		`CREATE INDEX IF NOT EXISTS idx_task_project ON task(project_id, sort_order)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", firstLine(stmt), err)
		}
	}
	if err := s.addColumnIfMissing("task", "body", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) addColumnIfMissing(table, column, decl string) error {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, decl))
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
