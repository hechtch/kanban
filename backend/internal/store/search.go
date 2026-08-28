package store

import "strings"

// ftsQuoteQuery turns user input into an FTS5 MATCH expression. Each
// whitespace-separated term is double-quoted so it's treated literally
// (avoids breaking on colons, dashes, or accidental FTS5 operators). All
// terms are AND'd together (FTS5's default for space-separated quoted
// strings).
//
//	"phase 3"          → `"phase" "3"`     (both terms must appear)
//	"foo-bar"          → `"foo-bar"`        (treated literally, not minus)
//	`bad"quote`        → `"badquote"`       (stripped — kept simple)
//	""                 → `""`               (caller should skip the filter)
func ftsQuoteQuery(q string) string {
	fields := strings.Fields(q)
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		// FTS5 doesn't allow unescaped double quotes inside a quoted string;
		// we just strip them. Phrase search remains accessible by passing a
		// single token; for v0.1 we don't expose explicit phrase syntax.
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		parts = append(parts, `"`+f+`"`)
	}
	return strings.Join(parts, " ")
}
