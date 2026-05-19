package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, COALESCE(slug,''), name, color, sort_order FROM project ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Color, &p.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProject(p Project) (Project, error) {
	if strings.TrimSpace(p.Name) == "" {
		return Project{}, fmt.Errorf("name required")
	}
	if p.Color == "" {
		p.Color = "#1f2430"
	}
	if p.Slug == "" {
		p.Slug = uniqueProjectSlug(s.db, Slugify(p.Name), 0)
	} else if err := ValidateSlug(p.Slug); err != nil {
		return Project{}, err
	}
	res, err := s.db.Exec(
		`INSERT INTO project (slug, name, color, sort_order) VALUES (?, ?, ?, ?)`,
		p.Slug, p.Name, p.Color, p.SortOrder,
	)
	if err != nil {
		return Project{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	p.ID = id
	return p, nil
}

func (s *Store) UpdateProject(id int64, patch ProjectPatch) (Project, error) {
	sets := []string{}
	args := []any{}
	if patch.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Color != nil {
		sets = append(sets, "color = ?")
		args = append(args, *patch.Color)
	}
	if patch.SortOrder != nil {
		sets = append(sets, "sort_order = ?")
		args = append(args, *patch.SortOrder)
	}
	if len(sets) > 0 {
		args = append(args, id)
		q := `UPDATE project SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
		res, err := s.db.Exec(q, args...)
		if err != nil {
			return Project{}, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return Project{}, ErrNotFound
		}
	}
	return s.GetProject(id)
}

func (s *Store) GetProject(id int64) (Project, error) {
	var p Project
	err := s.db.QueryRow(
		`SELECT id, COALESCE(slug,''), name, color, sort_order FROM project WHERE id = ?`, id,
	).Scan(&p.ID, &p.Slug, &p.Name, &p.Color, &p.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// GetProjectBySlug returns the project with the given slug, or ErrNotFound.
func (s *Store) GetProjectBySlug(slug string) (Project, error) {
	var p Project
	err := s.db.QueryRow(
		`SELECT id, COALESCE(slug,''), name, color, sort_order FROM project WHERE slug = ?`, slug,
	).Scan(&p.ID, &p.Slug, &p.Name, &p.Color, &p.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// UpsertProjectBySlug creates a project with the given slug, or patches an
// existing one. Returns (project, created, err). The slug must already be
// valid (validated at the API edge).
func (s *Store) UpsertProjectBySlug(slug string, up ProjectUpsert) (Project, bool, error) {
	existing, err := s.GetProjectBySlug(slug)
	if err == nil {
		patch := ProjectPatch{Name: up.Name, Color: up.Color, SortOrder: up.SortOrder}
		updated, err := s.UpdateProject(existing.ID, patch)
		return updated, false, err
	}
	if !errors.Is(err, ErrNotFound) {
		return Project{}, false, err
	}

	name := slug
	if up.Name != nil && strings.TrimSpace(*up.Name) != "" {
		name = *up.Name
	}
	color := "#1f2430"
	if up.Color != nil && *up.Color != "" {
		color = *up.Color
	}
	var sortOrder float64
	if up.SortOrder != nil {
		sortOrder = *up.SortOrder
	}
	created, err := s.CreateProject(Project{Slug: slug, Name: name, Color: color, SortOrder: sortOrder})
	return created, err == nil, err
}

func (s *Store) DeleteProject(id int64) error {
	res, err := s.db.Exec(`DELETE FROM project WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
