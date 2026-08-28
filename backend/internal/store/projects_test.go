package store

import (
	"testing"
)

func TestProjectArchivedRoundTrip(t *testing.T) {
	s := newTestStore(t)
	p, err := s.CreateProject(Project{Name: "2025 Taxes"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Archived {
		t.Fatal("new project should not be archived")
	}
	yes := true
	p, err = s.UpdateProject(p.ID, ProjectPatch{Archived: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Archived {
		t.Fatal("expected archived after patch")
	}
	list, err := s.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].Archived {
		t.Fatalf("ListProjects should carry archived: %+v", list)
	}
	no := false
	p, _, err = s.UpsertProjectBySlug(p.Slug, ProjectUpsert{Archived: &no})
	if err != nil {
		t.Fatal(err)
	}
	if p.Archived {
		t.Fatal("upsert should un-archive")
	}
}

func TestProjectTagsMergeIntoTasks(t *testing.T) {
	s := newTestStore(t)
	p, err := s.CreateProject(Project{Name: "2026 Taxes", Tags: []string{"tax", " finance "}})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Tags; len(got) != 2 || got[0] != "finance" || got[1] != "tax" {
		t.Fatalf("project tags = %v, want [finance tax]", got)
	}

	// A task in the project carries its own tags plus the project's.
	task, err := s.CreateTask(Task{Title: "W-2", ProjectID: &p.ID, Tags: []string{"docs", "tax"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := task.Tags; len(got) != 3 || got[0] != "docs" || got[1] != "finance" || got[2] != "tax" {
		t.Fatalf("task tags = %v, want [docs finance tax]", got)
	}
	// "tax" was inherited, so it must not have been written onto the task row.
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM task_tag tt JOIN tag ON tag.id = tt.tag_id WHERE tt.task_id = ? AND tag.name = 'tax'`,
		task.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("inherited tag should not be stored on the task row")
	}

	// Dropping a tag from the project drops it from the task at once.
	p, err = s.UpdateProject(p.ID, ProjectPatch{Tags: &[]string{"finance"}})
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := task.Tags; len(got) != 2 || got[0] != "docs" || got[1] != "finance" {
		t.Fatalf("after project tag change: %v, want [docs finance]", got)
	}

	// Moving the task out of the project leaves only its own tags.
	task, err = s.UpdateTask(task.ID, TaskPatch{ClearProjectID: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := task.Tags; len(got) != 1 || got[0] != "docs" {
		t.Fatalf("after leaving project: %v, want [docs]", got)
	}

	// ListTasks merges the same way as GetTask.
	other, err := s.CreateTask(Task{Title: "1099", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, lt := range list {
		if lt.ID == other.ID && (len(lt.Tags) != 1 || lt.Tags[0] != "finance") {
			t.Fatalf("ListTasks tags for %d = %v, want [finance]", lt.ID, lt.Tags)
		}
	}

	// Plan tasks go through the same merge.
	plan, _, err := s.UpsertPlan("tax-plan", PlanUpsert{ProjectSlug: &p.Slug})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Tags; len(got) != 1 || got[0] != "finance" {
		t.Fatalf("plan tags = %v, want [finance]", got)
	}
	plans, err := s.ListPlanTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || len(plans[0].Tags) != 1 {
		t.Fatalf("ListPlanTasks tags = %+v", plans)
	}
}
