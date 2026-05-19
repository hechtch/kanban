package store

type Project struct {
	ID        int64   `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
	SortOrder float64 `json:"sort_order"`
}

type Task struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	DueText     string   `json:"due_text"`
	ProjectID   *int64   `json:"project_id"`
	SortOrder   float64  `json:"sort_order"`
	PlanSlug    *string  `json:"plan_slug,omitempty"`
	GitBranch   *string  `json:"git_branch,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt *string  `json:"completed_at"`
	Tags        []string `json:"tags"`
}

// Activity is an append-only log entry for an agent-owned task.
type Activity struct {
	ID         int64   `json:"id"`
	TaskID     int64   `json:"task_id"`
	TS         string  `json:"ts"`
	Kind       string  `json:"kind"`
	FromStatus *string `json:"from,omitempty"`
	ToStatus   *string `json:"to,omitempty"`
	Text       string  `json:"text,omitempty"`
}

// PlanUpsert is the patch payload for PUT /api/agent/plans/:slug.
// nil fields are left alone; non-nil overwrite. ProjectSlug takes precedence
// over ProjectID when both are set; ClearProjectID clears either.
type PlanUpsert struct {
	Title          *string
	Body           *string
	Priority       *int
	DueText        *string
	ProjectID      *int64
	ProjectSlug    *string
	ClearProjectID bool
	Tags           *[]string
	GitBranch      *string
	ClearGitBranch bool
}

// ProjectUpsert is the payload for PUT /api/agent/projects/:slug.
type ProjectUpsert struct {
	Name      *string
	Color     *string
	SortOrder *float64
}

type TaskFilter struct {
	Status    string
	ProjectID *int64
	HasProj   bool
	Query     string
}

// ProjectPatch — nil means "leave alone". Name/Color/SortOrder are non-null
// columns; no need to model "clear to null".
type ProjectPatch struct {
	Name      *string
	Color     *string
	SortOrder *float64
}

// TaskPatch — nil means "leave alone". ProjectID uses a separate ClearProjectID
// flag so the handler can distinguish "absent" from "explicit null".
type TaskPatch struct {
	Title          *string
	Body           *string
	Status         *string
	Priority       *int
	DueText        *string
	ProjectID      *int64
	ClearProjectID bool
	SortOrder      *float64
	Tags           *[]string
	GitBranch      *string
	ClearGitBranch bool
}
