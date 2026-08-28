package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

// ErrValidation marks a caller error (bad status, effort, priority …) so the
// API layer can answer 422 instead of 500. Wrap with fmt.Errorf("%w: …").
var ErrValidation = errors.New("validation")

const projectCols = `id, COALESCE(slug,''), name, color, sort_order, archived`

func scanProject(r rowScanner) (Project, error) {
	var p Project
	var archived int
	if err := r.Scan(&p.ID, &p.Slug, &p.Name, &p.Color, &p.SortOrder, &archived); err != nil {
		return Project{}, err
	}
	p.Archived = archived != 0
	p.Tags = []string{}
	return p, nil
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT ` + projectCols + ` FROM project ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.attachProjectTags(out)
}

func (s *Store) CreateProject(p Project) (Project, error) {
	if strings.TrimSpace(p.Name) == "" {
		return Project{}, fmt.Errorf("%w: name required", ErrValidation)
	}
	if p.Color == "" {
		p.Color = "#1f2430"
	}
	if p.Slug == "" {
		p.Slug = uniqueProjectSlug(s.db, Slugify(p.Name), 0)
	} else if err := ValidateSlug(p.Slug); err != nil {
		return Project{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		`INSERT INTO project (slug, name, color, sort_order, archived) VALUES (?, ?, ?, ?, ?)`,
		p.Slug, p.Name, p.Color, p.SortOrder, boolInt(p.Archived),
	)
	if err != nil {
		return Project{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	if err := setLinkedTags(tx, "project_tag", "project_id", id, p.Tags); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, err
	}
	return s.GetProject(id)
}

func (s *Store) UpdateProject(id int64, patch ProjectPatch) (Project, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback() }()

	sets := []string{}
	args := []any{}
	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return Project{}, fmt.Errorf("%w: name required", ErrValidation)
		}
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
	if patch.Archived != nil {
		sets = append(sets, "archived = ?")
		args = append(args, boolInt(*patch.Archived))
	}
	if len(sets) > 0 {
		args = append(args, id)
		q := `UPDATE project SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
		res, err := tx.Exec(q, args...)
		if err != nil {
			return Project{}, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return Project{}, ErrNotFound
		}
	}
	if patch.Tags != nil {
		if err := setLinkedTags(tx, "project_tag", "project_id", id, *patch.Tags); err != nil {
			return Project{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Project{}, err
	}
	return s.GetProject(id)
}

func (s *Store) GetProject(id int64) (Project, error) {
	p, err := scanProject(s.db.QueryRow(`SELECT `+projectCols+` FROM project WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	ps := []Project{p}
	if err := s.attachProjectTags(ps); err != nil {
		return Project{}, err
	}
	return ps[0], nil
}

// GetProjectBySlug returns the project with the given slug, or ErrNotFound.
func (s *Store) GetProjectBySlug(slug string) (Project, error) {
	p, err := scanProject(s.db.QueryRow(`SELECT `+projectCols+` FROM project WHERE slug = ?`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	ps := []Project{p}
	if err := s.attachProjectTags(ps); err != nil {
		return Project{}, err
	}
	return ps[0], nil
}

// UpsertProjectBySlug creates a project with the given slug, or patches an
// existing one. Returns (project, created, err). The slug must already be
// valid (validated at the API edge).
func (s *Store) UpsertProjectBySlug(slug string, up ProjectUpsert) (Project, bool, error) {
	existing, err := s.GetProjectBySlug(slug)
	if err == nil {
		patch := ProjectPatch{
			Name: up.Name, Color: up.Color, SortOrder: up.SortOrder,
			Archived: up.Archived, Tags: up.Tags,
		}
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
	var archived bool
	if up.Archived != nil {
		archived = *up.Archived
	}
	var tags []string
	if up.Tags != nil {
		tags = *up.Tags
	}
	created, err := s.CreateProject(Project{
		Slug: slug, Name: name, Color: color, SortOrder: sortOrder, Archived: archived, Tags: tags,
	})
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

// attachProjectTags fills in Tags for each project in place.
func (s *Store) attachProjectTags(ps []Project) error {
	if len(ps) == 0 {
		return nil
	}
	ids := make([]int64, len(ps))
	for i := range ps {
		ids[i] = ps[i].ID
	}
	byProject, err := s.linkedTags("project_tag", "project_id", ids)
	if err != nil {
		return err
	}
	for i := range ps {
		if tags := byProject[ps[i].ID]; tags != nil {
			ps[i].Tags = tags
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
