package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/chrishecht/kanban/backend/internal/store"
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
)

type parseInput struct {
	Text string `json:"text"`
}

type parseOutput struct {
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	DueText     string   `json:"due_text"`
	Tags        []string `json:"tags"`
	ProjectName string   `json:"project_name"`
	ProjectID   *int64   `json:"project_id"`
}

func parseTask(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in parseInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out := parseDraft(in.Text)

		if out.ProjectName != "" {
			projects, err := st.ListProjects()
			if err == nil {
				lower := strings.ToLower(out.ProjectName)
				for _, p := range projects {
					if strings.ToLower(p.Name) == lower {
						id := p.ID
						out.ProjectID = &id
						break
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func parseDraft(raw string) parseOutput {
	text := raw

	// Strip tags / project / priority first — they're anchored on their own
	// sigils. Pull the trailing "by …" clause LAST so it only sweeps up plain
	// words at the end, never a stray tag like "#release".

	// tags
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

	// project (first @mention wins)
	project := ""
	if m := reProject.FindStringSubmatchIndex(text); m != nil {
		project = text[m[2]:m[3]]
		text = reProject.ReplaceAllString(text, "")
	}

	// priority — count bangs
	priority := 0
	if m := rePriority.FindStringSubmatchIndex(text); m != nil {
		priority = m[3] - m[2]
		text = rePriority.ReplaceAllString(text, " ")
	}

	// due — must run after the sigil-stripping so leftover sigils never end
	// up inside due_text.
	due := ""
	if m := reDue.FindStringSubmatchIndex(text); m != nil {
		due = strings.TrimSpace(text[m[2]:m[3]])
		text = text[:m[0]]
	}

	title := strings.TrimSpace(reSpaces.ReplaceAllString(text, " "))

	return parseOutput{
		Title:       title,
		Priority:    priority,
		DueText:     due,
		Tags:        tags,
		ProjectName: project,
	}
}

var reSpaces = regexp.MustCompile(`\s+`)
