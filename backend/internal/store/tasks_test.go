package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProjectsCRUD(t *testing.T) {
	s := newTestStore(t)

	p, err := s.CreateProject(Project{Name: "Site", Color: "#abc", SortOrder: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 || p.Name != "Site" {
		t.Fatalf("unexpected project: %+v", p)
	}

	name := "Site v2"
	up, err := s.UpdateProject(p.ID, ProjectPatch{Name: &name})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if up.Name != "Site v2" || up.Color != "#abc" {
		t.Fatalf("update merged wrong: %+v", up)
	}

	all, err := s.ListProjects()
	if err != nil || len(all) != 1 {
		t.Fatalf("list: %v / %d", err, len(all))
	}

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetProject(p.ID); err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestProjectRequiresName(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateProject(Project{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTaskCRUDAndFilter(t *testing.T) {
	s := newTestStore(t)
	proj, _ := s.CreateProject(Project{Name: "P"})

	a, err := s.CreateTask(Task{Title: "alpha", ProjectID: &proj.ID, Tags: []string{"ping", "urgent"}})
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if len(a.Tags) != 2 {
		t.Fatalf("tags not persisted: %+v", a)
	}
	b, _ := s.CreateTask(Task{Title: "beta", Status: "doing"})
	_, _ = s.CreateTask(Task{Title: "gamma alpha", Status: "todo", ProjectID: &proj.ID})

	all, err := s.ListTasks(TaskFilter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("list all: %v / %d", err, len(all))
	}

	doing, _ := s.ListTasks(TaskFilter{Status: "doing"})
	if len(doing) != 1 || doing[0].ID != b.ID {
		t.Fatalf("status filter: %+v", doing)
	}

	noProj, _ := s.ListTasks(TaskFilter{HasProj: true})
	if len(noProj) != 1 || noProj[0].ID != b.ID {
		t.Fatalf("null project filter: %+v", noProj)
	}

	withProj, _ := s.ListTasks(TaskFilter{HasProj: true, ProjectID: &proj.ID})
	if len(withProj) != 2 {
		t.Fatalf("project filter: %d", len(withProj))
	}

	q, _ := s.ListTasks(TaskFilter{Query: "alpha"})
	if len(q) != 2 {
		t.Fatalf("query filter: %d", len(q))
	}
}

func TestTaskPatchAndCompletedAt(t *testing.T) {
	s := newTestStore(t)
	tk, _ := s.CreateTask(Task{Title: "x"})

	done := "done"
	updated, err := s.UpdateTask(tk.ID, TaskPatch{Status: &done})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.Status != "done" || updated.CompletedAt == nil {
		t.Fatalf("expected completed_at set: %+v", updated)
	}

	todo := "todo"
	updated, _ = s.UpdateTask(tk.ID, TaskPatch{Status: &todo})
	if updated.CompletedAt != nil {
		t.Fatalf("expected completed_at cleared")
	}

	proj, _ := s.CreateProject(Project{Name: "P"})
	updated, _ = s.UpdateTask(tk.ID, TaskPatch{ProjectID: &proj.ID})
	if updated.ProjectID == nil || *updated.ProjectID != proj.ID {
		t.Fatalf("project not set: %+v", updated.ProjectID)
	}
	updated, _ = s.UpdateTask(tk.ID, TaskPatch{ClearProjectID: true})
	if updated.ProjectID != nil {
		t.Fatalf("project not cleared: %+v", updated.ProjectID)
	}

	tags := []string{"a", "b"}
	updated, _ = s.UpdateTask(tk.ID, TaskPatch{Tags: &tags})
	if len(updated.Tags) != 2 {
		t.Fatalf("tags not set: %+v", updated.Tags)
	}
	empty := []string{}
	updated, _ = s.UpdateTask(tk.ID, TaskPatch{Tags: &empty})
	if len(updated.Tags) != 0 {
		t.Fatalf("tags not cleared: %+v", updated.Tags)
	}
}

func TestTaskValidation(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateTask(Task{}); err == nil {
		t.Fatal("expected title required")
	}
	if _, err := s.CreateTask(Task{Title: "x", Status: "bogus"}); err == nil {
		t.Fatal("expected status validation")
	}
	if _, err := s.CreateTask(Task{Title: "x", Priority: 9}); err == nil {
		t.Fatal("expected priority validation")
	}
}

func TestTaskDeleteCascadesTags(t *testing.T) {
	s := newTestStore(t)
	tk, _ := s.CreateTask(Task{Title: "x", Tags: []string{"foo"}})
	if err := s.DeleteTask(tk.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM task_tag").Scan(&n)
	if n != 0 {
		t.Fatalf("task_tag rows leaked: %d", n)
	}
}
