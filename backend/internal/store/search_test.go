package store

import "testing"

func TestFTSQuoteQuery(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"   ":          "",
		"phase":        `"phase"`,
		"phase 3":      `"phase" "3"`,
		"  phase  3  ": `"phase" "3"`,
		"foo-bar":      `"foo-bar"`,
		`bad"quote`:    `"badquote"`,
	}
	for in, want := range cases {
		if got := ftsQuoteQuery(in); got != want {
			t.Errorf("ftsQuoteQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestListTasksSearchesTitleAndBody(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateTask(Task{
		Title: "fix the proxy bug",
		Body:  "ng serve hits port 4200; need a proxy.conf.json",
	}); err != nil {
		t.Fatalf("create 1: %v", err)
	}
	if _, err := s.CreateTask(Task{
		Title: "rename status enum",
		Body:  "waiting → blocked migration",
	}); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if _, err := s.CreateTask(Task{
		Title: "smoke test",
		Body:  "just a regular task",
	}); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	// Title-only match — should find the proxy task.
	got, err := s.ListTasks(TaskFilter{Query: "proxy"})
	if err != nil {
		t.Fatalf("title match: %v", err)
	}
	if len(got) != 1 || got[0].Title != "fix the proxy bug" {
		t.Fatalf("expected proxy task, got %+v", got)
	}

	// Body-only match — searching "migration" should find task 2 even
	// though its title doesn't contain the word.
	got, err = s.ListTasks(TaskFilter{Query: "migration"})
	if err != nil {
		t.Fatalf("body match: %v", err)
	}
	if len(got) != 1 || got[0].Title != "rename status enum" {
		t.Fatalf("expected body-only match, got %+v", got)
	}

	// Multi-word AND — both terms must appear (anywhere across title or body).
	got, err = s.ListTasks(TaskFilter{Query: "proxy port"})
	if err != nil {
		t.Fatalf("multi-word: %v", err)
	}
	if len(got) != 1 || got[0].Title != "fix the proxy bug" {
		t.Fatalf("expected single proxy match for 'proxy port', got %+v", got)
	}

	// No-match returns empty.
	got, err = s.ListTasks(TaskFilter{Query: "nonexistent-term"})
	if err != nil {
		t.Fatalf("no-match: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}

	// Empty query → no filter, returns everything.
	got, err = s.ListTasks(TaskFilter{Query: ""})
	if err != nil {
		t.Fatalf("empty query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("empty query should return all, got %d", len(got))
	}
}

func TestFTSIndexStaysInSyncOnUpdate(t *testing.T) {
	s := newTestStore(t)
	tk, err := s.CreateTask(Task{Title: "originaltitle", Body: "alpha"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Initial body matches "alpha".
	got, _ := s.ListTasks(TaskFilter{Query: "alpha"})
	if len(got) != 1 {
		t.Fatalf("expected initial match, got %d", len(got))
	}

	// Update the body — old term should disappear, new term should match.
	newBody := "completely different content with beta term"
	if _, err := s.UpdateTask(tk.ID, TaskPatch{Body: &newBody}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := s.ListTasks(TaskFilter{Query: "alpha"}); len(got) != 0 {
		t.Fatalf("old term should not match after update, got %d", len(got))
	}
	if got, _ := s.ListTasks(TaskFilter{Query: "beta"}); len(got) != 1 {
		t.Fatalf("new term should match after update, got %d", len(got))
	}
}

func TestFTSIndexStaysInSyncOnDelete(t *testing.T) {
	s := newTestStore(t)
	tk, _ := s.CreateTask(Task{Title: "deletemetask", Body: "uniqueword"})
	if got, _ := s.ListTasks(TaskFilter{Query: "uniqueword"}); len(got) != 1 {
		t.Fatalf("setup: expected match before delete")
	}
	if err := s.DeleteTask(tk.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, _ := s.ListTasks(TaskFilter{Query: "uniqueword"}); len(got) != 0 {
		t.Fatalf("term should not match after delete, got %d", len(got))
	}
}

func TestListPlanTasksSearchesTitleAndBody(t *testing.T) {
	s := newTestStore(t)
	if _, _, err := s.UpsertPlan("plan-a", PlanUpsert{
		Body: strPtr("contains the keyword needle in the body"),
	}); err != nil {
		t.Fatalf("upsert a: %v", err)
	}
	if _, _, err := s.UpsertPlan("plan-b", PlanUpsert{
		Body: strPtr("nothing special"),
	}); err != nil {
		t.Fatalf("upsert b: %v", err)
	}

	got, err := s.ListPlanTasks(PlanFilter{Query: "needle"})
	if err != nil {
		t.Fatalf("plan search: %v", err)
	}
	if len(got) != 1 || got[0].PlanSlug == nil || *got[0].PlanSlug != "plan-a" {
		t.Fatalf("expected plan-a, got %+v", got)
	}
}

func strPtr(s string) *string { return &s }
