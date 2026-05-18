package store

type Project struct {
	ID        int64   `json:"id"`
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
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt *string  `json:"completed_at"`
	Tags        []string `json:"tags"`
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
}
