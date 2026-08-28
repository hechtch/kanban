package store

import (
	"database/sql"
	"fmt"
	"strings"

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
		// Status no longer carries a CHECK constraint at the DB level — the
		// Go layer's validStatus map is the source of truth. Drop-and-rebuild
		// is the only way to change a CHECK in SQLite, so removing it now
		// keeps future status renames cheap.
		`CREATE TABLE IF NOT EXISTS task (
			id            INTEGER PRIMARY KEY,
			title         TEXT NOT NULL,
			body          TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL,
			priority      INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 3),
			due_text      TEXT NOT NULL DEFAULT '',
			project_id    INTEGER REFERENCES project(id) ON DELETE SET NULL,
			sort_order    REAL NOT NULL DEFAULT 0,
			plan_slug     TEXT,
			git_branch    TEXT,
			model         TEXT,
			effort        TEXT,
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
		`CREATE TABLE IF NOT EXISTS activity (
			id           INTEGER PRIMARY KEY,
			task_id      INTEGER NOT NULL REFERENCES task(id) ON DELETE CASCADE,
			ts           TEXT NOT NULL DEFAULT (datetime('now')),
			kind         TEXT NOT NULL CHECK (kind IN ('create','status','note','git')),
			from_status  TEXT,
			to_status    TEXT,
			text         TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_task ON activity(task_id, ts)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", firstLine(stmt), err)
		}
	}
	if err := s.addColumnIfMissing("task", "body", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("task", "plan_slug", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("task", "git_branch", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("task", "model", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("task", "effort", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("project", "slug", "TEXT"); err != nil {
		return err
	}
	if err := s.migrateTaskStatusConstraint(); err != nil {
		return fmt.Errorf("migrate task status constraint: %w", err)
	}
	if err := s.setupTaskFTS(); err != nil {
		return fmt.Errorf("setup task_fts: %w", err)
	}
	// Partial unique indexes: enforce uniqueness only among non-NULL slugs.
	for _, idx := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_task_plan_slug
		   ON task(plan_slug) WHERE plan_slug IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_slug
		   ON project(slug) WHERE slug IS NOT NULL`,
	} {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	if err := s.backfillProjectSlugs(); err != nil {
		return fmt.Errorf("backfill project slugs: %w", err)
	}
	return nil
}

// migrateTaskStatusConstraint relaxes the old CHECK (status IN (...)) on
// task and renames 'waiting' → 'blocked'. The rebuild also drops the
// constraint entirely so future status changes only need a Go-side update.
// Idempotent: if the current schema has no status CHECK clause, this is a
// no-op.
func (s *Store) migrateTaskStatusConstraint() error {
	var schemaSQL string
	if err := s.db.QueryRow(
		`SELECT sql FROM sqlite_schema WHERE type='table' AND name='task'`,
	).Scan(&schemaSQL); err != nil {
		return err
	}
	if !strings.Contains(schemaSQL, "CHECK (status IN") &&
		!strings.Contains(schemaSQL, "CHECK(status IN") {
		// New-schema DBs and already-migrated DBs: nothing to do.
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`CREATE TABLE task_new (
		id            INTEGER PRIMARY KEY,
		title         TEXT NOT NULL,
		body          TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL,
		priority      INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 3),
		due_text      TEXT NOT NULL DEFAULT '',
		project_id    INTEGER REFERENCES project(id) ON DELETE SET NULL,
		sort_order    REAL NOT NULL DEFAULT 0,
		plan_slug     TEXT,
		git_branch    TEXT,
		model         TEXT,
		effort        TEXT,
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
		completed_at  TEXT
	)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO task_new (id, title, body, status, priority, due_text,
		                     project_id, sort_order, plan_slug, git_branch, model, effort,
		                     created_at, updated_at, completed_at)
		SELECT id, title, body, status, priority, due_text,
		       project_id, sort_order, plan_slug, git_branch, model, effort,
		       created_at, updated_at, completed_at
		  FROM task`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE task`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE task_new RENAME TO task`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE task SET status = 'blocked' WHERE status = 'waiting'`); err != nil {
		return err
	}
	// Indexes were tied to the dropped table; recreate them.
	for _, idx := range []string{
		`CREATE INDEX idx_task_status ON task(status, sort_order)`,
		`CREATE INDEX idx_task_project ON task(project_id, sort_order)`,
		`CREATE UNIQUE INDEX idx_task_plan_slug ON task(plan_slug) WHERE plan_slug IS NOT NULL`,
	} {
		if _, err := tx.Exec(idx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// setupTaskFTS creates the FTS5 virtual table that mirrors task.title and
// task.body, plus the triggers that keep it in sync. Idempotent — uses
// IF NOT EXISTS everywhere. On first creation, backfills from existing rows.
func (s *Store) setupTaskFTS() error {
	// Was task_fts already created?
	var existing int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='task_fts'`,
	).Scan(&existing); err != nil {
		return err
	}

	stmts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS task_fts USING fts5(
			title, body,
			content='task', content_rowid='id',
			tokenize='unicode61 remove_diacritics 2'
		)`,
		`CREATE TRIGGER IF NOT EXISTS task_fts_ai AFTER INSERT ON task BEGIN
			INSERT INTO task_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
		END`,
		`CREATE TRIGGER IF NOT EXISTS task_fts_ad AFTER DELETE ON task BEGIN
			INSERT INTO task_fts(task_fts, rowid, title, body)
			  VALUES('delete', old.id, old.title, old.body);
		END`,
		`CREATE TRIGGER IF NOT EXISTS task_fts_au AFTER UPDATE ON task BEGIN
			INSERT INTO task_fts(task_fts, rowid, title, body)
			  VALUES('delete', old.id, old.title, old.body);
			INSERT INTO task_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
		END`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", firstLine(stmt), err)
		}
	}

	// On first creation, backfill from existing task rows. Subsequent runs
	// see existing > 0 and skip this.
	if existing == 0 {
		if _, err := s.db.Exec(
			`INSERT INTO task_fts(rowid, title, body) SELECT id, title, body FROM task`,
		); err != nil {
			return fmt.Errorf("backfill task_fts: %w", err)
		}
	}
	return nil
}

// backfillProjectSlugs derives a slug for any project row that doesn't have
// one yet (rows created before the column existed). Idempotent.
func (s *Store) backfillProjectSlugs() error {
	rows, err := s.db.Query(`SELECT id, name FROM project WHERE slug IS NULL OR slug = ''`)
	if err != nil {
		return err
	}
	type pending struct {
		id   int64
		name string
	}
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.name); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range todo {
		slug := uniqueProjectSlug(s.db, Slugify(p.name), p.id)
		if _, err := s.db.Exec(`UPDATE project SET slug = ? WHERE id = ?`, slug, p.id); err != nil {
			return err
		}
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
