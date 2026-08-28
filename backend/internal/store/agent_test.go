package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestUpsertPlanCreatesAndPatches(t *testing.T) {
	s := newTestStore(t)

	body := "## Phase 1\n- [ ] do thing"
	title := "Agent API"
	pri := 2
	tags := []string{"infra"}

	first, created, err := s.UpsertPlan("agent-api", PlanUpsert{
		Title:    &title,
		Body:     &body,
		Priority: &pri,
		Tags:     &tags,
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created {
		t.Fatal("expected created=true on first touch")
	}
	if first.PlanSlug == nil || *first.PlanSlug != "agent-api" {
		t.Fatalf("plan_slug not set: %+v", first.PlanSlug)
	}
	if first.Status != "todo" {
		t.Fatalf("expected default status=todo, got %q", first.Status)
	}
	if first.Body != body || first.Title != "Agent API" || first.Priority != 2 {
		t.Fatalf("fields not persisted: %+v", first)
	}

	// "create" activity should exist exactly once.
	acts, err := s.ListPlanActivity("agent-api")
	if err != nil {
		t.Fatalf("activity: %v", err)
	}
	if len(acts) != 1 || acts[0].Kind != "create" {
		t.Fatalf("expected one create activity, got %+v", acts)
	}

	// Second upsert patches; no new create entry.
	newBody := body + "\n- [x] do another"
	second, created, err := s.UpsertPlan("agent-api", PlanUpsert{Body: &newBody})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created {
		t.Fatal("expected created=false on second touch")
	}
	if second.Body != newBody {
		t.Fatal("body not updated on patch")
	}
	if second.ID != first.ID {
		t.Fatal("upsert created a duplicate")
	}

	acts2, _ := s.ListPlanActivity("agent-api")
	if len(acts2) != 1 {
		t.Fatalf("expected still one activity, got %d", len(acts2))
	}
}

func TestUpsertPlanSlugUniqueness(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("kanban", PlanUpsert{}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Direct duplicate insert (bypassing UpsertPlan's existence check)
	// must fail on the unique index.
	_, err := s.db.Exec(
		`INSERT INTO task (title, status, plan_slug) VALUES ('dup', 'todo', 'kanban')`,
	)
	if err == nil {
		t.Fatal("expected unique-index violation, got nil")
	}
}

func TestSetPlanStatusIdempotent(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Same status, no note → no new activity row.
	before, _ := s.ListPlanActivity("agent-api")
	if _, err := s.SetPlanStatus("agent-api", "todo", ""); err != nil {
		t.Fatalf("set same status: %v", err)
	}
	after, _ := s.ListPlanActivity("agent-api")
	if len(after) != len(before) {
		t.Fatalf("expected no new activity on same-status PUT, got %d→%d",
			len(before), len(after))
	}

	// Same status WITH a note → activity row written.
	if _, err := s.SetPlanStatus("agent-api", "todo", "still working"); err != nil {
		t.Fatalf("set same status w/ note: %v", err)
	}
	withNote, _ := s.ListPlanActivity("agent-api")
	if len(withNote) != len(after)+1 {
		t.Fatalf("expected activity row for same-status note, got %d→%d",
			len(after), len(withNote))
	}
}

func TestSetPlanStatusTransitionWritesActivity(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	updated, err := s.SetPlanStatus("agent-api", "doing", "")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}
	if updated.Status != "doing" {
		t.Fatalf("status not updated: %q", updated.Status)
	}
	acts, _ := s.ListPlanActivity("agent-api")
	// create + status
	if len(acts) != 2 {
		t.Fatalf("expected 2 activities, got %d (%+v)", len(acts), acts)
	}
	last := acts[len(acts)-1]
	if last.Kind != "status" || last.FromStatus == nil || *last.FromStatus != "todo" {
		t.Fatalf("status activity wrong: %+v", last)
	}
	if last.ToStatus == nil || *last.ToStatus != "doing" {
		t.Fatalf("to_status wrong: %+v", last.ToStatus)
	}
}

func TestSetPlanStatusValidatesStatus(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.SetPlanStatus("agent-api", "bogus", ""); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestGetPlanByUnknownSlug(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.GetTaskByPlanSlug("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestValidateSlug(t *testing.T) {
	good := []string{"agent-api", "kanban", "a", "a1", "long-multi-part-slug"}
	bad := []string{"", "Agent-Api", "plan_slug", "-leading", "trailing-", "has space", "Upper"}
	for _, g := range good {
		if err := ValidateSlug(g); err != nil {
			t.Errorf("expected %q valid: %v", g, err)
		}
	}
	for _, b := range bad {
		if err := ValidateSlug(b); err == nil {
			t.Errorf("expected %q invalid", b)
		}
	}
}

func TestAppendPlanNoteWritesActivity(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	act, err := s.AppendPlanNote("agent-api", "blocked on upstream")
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if act.Kind != "note" || act.Text != "blocked on upstream" {
		t.Fatalf("note activity wrong: %+v", act)
	}
}

func TestStatusConstraintMigrationFromWaitingToBlocked(t *testing.T) {
	// Seed a temp DB with the old schema (CHECK constraint + 'waiting' row),
	// then run Open() which should run migrate() and rename to 'blocked'.
	dir := t.TempDir()
	path := dir + "/old.db"
	raw, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE task (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('todo','doing','waiting','done','backlog')),
			priority INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 3),
			due_text TEXT NOT NULL DEFAULT '',
			project_id INTEGER,
			sort_order REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at TEXT
		)`,
		`INSERT INTO task (title, status) VALUES ('legacy waiting', 'waiting')`,
		`INSERT INTO task (title, status) VALUES ('legacy doing', 'doing')`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	raw.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated: %v", err)
	}
	defer store.Close()

	all, err := store.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	statuses := map[string]string{}
	for _, tk := range all {
		statuses[tk.Title] = tk.Status
	}
	if statuses["legacy waiting"] != "blocked" {
		t.Fatalf("waiting → blocked migration didn't run: %q", statuses["legacy waiting"])
	}
	if statuses["legacy doing"] != "doing" {
		t.Fatalf("doing should be untouched: %q", statuses["legacy doing"])
	}

	// And the new statuses are now usable.
	if _, err := store.CreateTask(Task{Title: "x", Status: "awaiting_merge"}); err != nil {
		t.Fatalf("awaiting_merge should be valid post-migration: %v", err)
	}
}

func TestGitBranchRoundTrip(t *testing.T) {
	s := newTestStore(t)
	branch := "feat/agent-api"
	task, _, err := s.UpsertPlan("agent-api", PlanUpsert{GitBranch: &branch})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if task.GitBranch == nil || *task.GitBranch != branch {
		t.Fatalf("git_branch not set on create: %+v", task.GitBranch)
	}

	newBranch := "fix/bug-42"
	task2, _, err := s.UpsertPlan("agent-api", PlanUpsert{GitBranch: &newBranch})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if task2.GitBranch == nil || *task2.GitBranch != newBranch {
		t.Fatalf("git_branch not updated: %+v", task2.GitBranch)
	}

	// Clear via TaskPatch.
	task3, err := s.UpdateTask(task2.ID, TaskPatch{ClearGitBranch: true})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if task3.GitBranch != nil {
		t.Fatalf("git_branch not cleared: %+v", task3.GitBranch)
	}
}

func TestAwaitingMergeStatusAccepted(t *testing.T) {
	s := newTestStore(t)
	tk, err := s.CreateTask(Task{Title: "merge me", Status: "awaiting_merge"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tk.Status != "awaiting_merge" {
		t.Fatalf("status not stored: %q", tk.Status)
	}
	if tk.CompletedAt != nil {
		t.Fatalf("awaiting_merge should not set completed_at: %+v", tk.CompletedAt)
	}
}

func TestBlockedStatusAcceptedAndWaitingRejected(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateTask(Task{Title: "b", Status: "blocked"}); err != nil {
		t.Fatalf("blocked should be valid: %v", err)
	}
	if _, err := s.CreateTask(Task{Title: "w", Status: "waiting"}); err == nil {
		t.Fatal("expected 'waiting' to be rejected after rename")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Kanban Work Tracker": "kanban-work-tracker",
		"Phase 2 — Refactor!": "phase-2-refactor",
		"  weird   spaces  ":  "weird-spaces",
		"already-kebab":       "already-kebab",
		"UPPERCASE":           "uppercase",
		"---":                 "",
		"a.b.c":               "a-b-c",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateProjectAutoSlug(t *testing.T) {
	s := newTestStore(t)
	p1, err := s.CreateProject(Project{Name: "Kanban Work Tracker"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p1.Slug != "kanban-work-tracker" {
		t.Errorf("slug not auto-derived: %q", p1.Slug)
	}

	// Same name → suffixed slug, no unique-index collision.
	p2, err := s.CreateProject(Project{Name: "Kanban Work Tracker"})
	if err != nil {
		t.Fatalf("create duplicate: %v", err)
	}
	if p2.Slug != "kanban-work-tracker-2" {
		t.Errorf("expected -2 suffix, got %q", p2.Slug)
	}
}

func TestUpsertProjectBySlug(t *testing.T) {
	s := newTestStore(t)
	name := "Infrastructure"
	color := "#abc123"
	p, created, err := s.UpsertProjectBySlug("infra", ProjectUpsert{Name: &name, Color: &color})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created || p.Slug != "infra" || p.Name != "Infrastructure" || p.Color != "#abc123" {
		t.Fatalf("create wrong: created=%v, %+v", created, p)
	}

	newColor := "#fff"
	p2, created, err := s.UpsertProjectBySlug("infra", ProjectUpsert{Color: &newColor})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if created {
		t.Fatal("expected created=false on second upsert")
	}
	if p2.ID != p.ID || p2.Color != "#fff" || p2.Name != "Infrastructure" {
		t.Fatalf("patch merged wrong: %+v", p2)
	}
}

func TestUpsertPlanResolvesProjectSlug(t *testing.T) {
	s := newTestStore(t)
	name := "API"
	if _, _, err := s.UpsertProjectBySlug("api", ProjectUpsert{Name: &name}); err != nil {
		t.Fatalf("project: %v", err)
	}

	projSlug := "api"
	task, _, err := s.UpsertPlan("agent-api", PlanUpsert{ProjectSlug: &projSlug})
	if err != nil {
		t.Fatalf("upsert plan w/ project: %v", err)
	}
	if task.ProjectID == nil {
		t.Fatal("project_id not set from project_slug")
	}
}

func TestUpsertPlanUnknownProjectSlug(t *testing.T) {
	s := newTestStore(t)
	missing := "ghost"
	_, _, err := s.UpsertPlan("agent-api", PlanUpsert{ProjectSlug: &missing})
	if err == nil || !strings.Contains(err.Error(), `unknown project_slug "ghost"`) {
		t.Fatalf("expected unknown-project-slug error, got %v", err)
	}
}

func TestUpsertPlanClearProjectViaEmptySlug(t *testing.T) {
	s := newTestStore(t)
	name := "X"
	if _, _, err := s.UpsertProjectBySlug("x", ProjectUpsert{Name: &name}); err != nil {
		t.Fatalf("project: %v", err)
	}
	projSlug := "x"
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{ProjectSlug: &projSlug}); err != nil {
		t.Fatalf("set: %v", err)
	}
	empty := ""
	task, _, err := s.UpsertPlan("agent-api", PlanUpsert{ProjectSlug: &empty})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if task.ProjectID != nil {
		t.Fatalf("project_id not cleared: %+v", task.ProjectID)
	}
}

func TestListPlanTasksByProjectFilter(t *testing.T) {
	s := newTestStore(t)
	name := "Infra"
	if _, _, err := s.UpsertProjectBySlug("infra", ProjectUpsert{Name: &name}); err != nil {
		t.Fatalf("project: %v", err)
	}
	projSlug := "infra"
	if _, _, err := s.UpsertPlan("plan-a", PlanUpsert{ProjectSlug: &projSlug}); err != nil {
		t.Fatalf("plan-a: %v", err)
	}
	if _, _, err := s.UpsertPlan("plan-b", PlanUpsert{}); err != nil {
		t.Fatalf("plan-b: %v", err)
	}

	filtered, err := s.ListPlanTasks("infra")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].PlanSlug == nil || *filtered[0].PlanSlug != "plan-a" {
		t.Fatalf("expected only plan-a, got %+v", filtered)
	}

	if _, err := s.ListPlanTasks("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown project filter, got %v", err)
	}
}

func TestListPlanTasksFiltersOutHumanTasks(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateTask(Task{Title: "human one"}); err != nil {
		t.Fatalf("create human task: %v", err)
	}
	if _, _, err := s.UpsertPlan("agent-api", PlanUpsert{}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	plans, err := s.ListPlanTasks()
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans) != 1 || plans[0].PlanSlug == nil || *plans[0].PlanSlug != "agent-api" {
		t.Fatalf("expected only the agent-api plan, got %+v", plans)
	}
}

func TestModelEffortRoundTrip(t *testing.T) {
	s := newTestStore(t)
	model, effort := " Fable ", "XHIGH"
	task, _, err := s.UpsertPlan("audit", PlanUpsert{Model: &model, Effort: &effort})
	if err != nil {
		t.Fatal(err)
	}
	// Normalised: trimmed + lowercased.
	if task.Model == nil || *task.Model != "fable" {
		t.Fatalf("model not set on create: %+v", task.Model)
	}
	if task.Effort == nil || *task.Effort != "xhigh" {
		t.Fatalf("effort not set on create: %+v", task.Effort)
	}

	// Patch path through the upsert.
	sonnet, high := "sonnet", "high"
	task2, _, err := s.UpsertPlan("audit", PlanUpsert{Model: &sonnet, Effort: &high})
	if err != nil {
		t.Fatal(err)
	}
	if *task2.Model != "sonnet" || *task2.Effort != "high" {
		t.Fatalf("not updated: %v / %v", *task2.Model, *task2.Effort)
	}

	// Bad effort is rejected, and nothing else changes.
	bogus := "ultra"
	if _, _, err := s.UpsertPlan("audit", PlanUpsert{Effort: &bogus}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for bad effort, got %v", err)
	}
	if _, err := s.CreateTask(Task{Title: "x", Effort: &bogus}); err == nil {
		t.Fatal("expected invalid effort to be rejected on create")
	}

	// Clear both.
	task3, err := s.UpdateTask(task2.ID, TaskPatch{ClearModel: true, ClearEffort: true})
	if err != nil {
		t.Fatal(err)
	}
	if task3.Model != nil || task3.Effort != nil {
		t.Fatalf("not cleared: %v / %v", task3.Model, task3.Effort)
	}

	// Empty string behaves like clear.
	empty := ""
	task4, _, err := s.UpsertPlan("audit", PlanUpsert{Model: &sonnet})
	if err != nil {
		t.Fatal(err)
	}
	task4, err = s.UpdateTask(task4.ID, TaskPatch{Model: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if task4.Model != nil {
		t.Fatalf("empty model should clear, got %v", *task4.Model)
	}
}
