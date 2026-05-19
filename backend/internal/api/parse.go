package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hechtch/kanban/backend/internal/store"
)

// Examples:
//   "email landlord about the leak by friday !! @admin #ping"
//   "ship v0.1 !!! by next wk #release"
//
// Order is intentionally fixed — strip from the end so we don't eat into the
// title. The "by <text>" clause is greedy to end-of-string.
var (
	reTag      = regexp.MustCompile(`(?:^|\s)#([\p{L}\p{N}_-]+)`)
	reProject  = regexp.MustCompile(`(?:^|\s)@([\p{L}\p{N}_-]+)`)
	rePriority = regexp.MustCompile(`(?:^|\s)(!{1,3})(?:\s|$)`)
	reDue      = regexp.MustCompile(`(?i)\s+by\s+(.+?)\s*$`)
	reSpaces   = regexp.MustCompile(`\s+`)
)

type parseInput struct {
	Body struct {
		Text string `json:"text" doc:"Natural-language task draft, e.g. 'email landlord by friday !! @admin #ping'"`
	}
}

type parseDraft struct {
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	DueText     string   `json:"due_text"`
	Tags        []string `json:"tags"`
	ProjectName string   `json:"project_name"`
	ProjectID   *int64   `json:"project_id"`
}

type parseOutput struct {
	Body parseDraft
}

func registerParse(api huma.API, st *store.Store) {
	huma.Register(api, huma.Operation{
		OperationID: "parse-task",
		Method:      http.MethodPost,
		Path:        "/api/tasks/parse",
		Summary:     "Parse a natural-language task draft",
		Description: "Returns a draft (no persistence). `@project` resolves to `project_id` when the name matches an existing project.",
		Tags:        []string{"Tasks"},
	}, func(_ context.Context, in *parseInput) (*parseOutput, error) {
		draft := parseText(in.Body.Text)
		if draft.ProjectName != "" {
			projects, err := st.ListProjects()
			if err == nil {
				lower := strings.ToLower(draft.ProjectName)
				for _, p := range projects {
					if strings.ToLower(p.Name) == lower {
						id := p.ID
						draft.ProjectID = &id
						break
					}
				}
			}
		}
		return &parseOutput{Body: draft}, nil
	})
}

// parseText is the pure-parsing core, exported package-internal so the test
// suite can exercise it without a store.
func parseText(raw string) parseDraft {
	text := raw

	// Strip tags / project / priority first — they're anchored on their own
	// sigils. Pull the trailing "by …" clause LAST so it only sweeps up plain
	// words at the end, never a stray tag like "#release".
	tags := []string{}
	seen := map[string]bool{}
	if matches := reTag.FindAllStringSubmatchIndex(text, -1); matches != nil {
		for _, m := range matches {
			tag := text[m[2]:m[3]]
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
		text = reTag.ReplaceAllString(text, "")
	}

	project := ""
	if m := reProject.FindStringSubmatchIndex(text); m != nil {
		project = text[m[2]:m[3]]
		text = reProject.ReplaceAllString(text, "")
	}

	priority := 0
	if m := rePriority.FindStringSubmatchIndex(text); m != nil {
		priority = m[3] - m[2]
		text = rePriority.ReplaceAllString(text, " ")
	}

	due := ""
	if m := reDue.FindStringSubmatchIndex(text); m != nil {
		due = strings.TrimSpace(text[m[2]:m[3]])
		text = text[:m[0]]
	}

	title := strings.TrimSpace(reSpaces.ReplaceAllString(text, " "))

	return parseDraft{
		Title:       title,
		Priority:    priority,
		DueText:     due,
		Tags:        tags,
		ProjectName: project,
	}
}
