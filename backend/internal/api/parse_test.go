package api

import (
	"reflect"
	"testing"
)

func TestParseDraft(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want parseOutput
	}{
		{
			name: "full example",
			in:   "email landlord about the leak by friday !! @admin #ping",
			want: parseOutput{
				Title:       "email landlord about the leak",
				Priority:    2,
				DueText:     "friday",
				Tags:        []string{"ping"},
				ProjectName: "admin",
			},
		},
		{
			name: "ship release",
			in:   "ship v0.1 !!! by next wk #release",
			want: parseOutput{
				Title:       "ship v0.1",
				Priority:    3,
				DueText:     "next wk",
				Tags:        []string{"release"},
			},
		},
		{
			name: "plain title",
			in:   "buy milk",
			want: parseOutput{Title: "buy milk", Tags: []string{}},
		},
		{
			name: "multi-tag dedup",
			in:   "fix #bug stuff #bug #urgent",
			want: parseOutput{
				Title: "fix stuff",
				Tags:  []string{"bug", "urgent"},
			},
		},
		{
			name: "by-clause swallows trailing tags",
			in:   "call mom by tomorrow",
			want: parseOutput{
				Title:   "call mom",
				DueText: "tomorrow",
				Tags:    []string{},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDraft(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseDraft(%q)\n got: %+v\nwant: %+v", tc.in, got, tc.want)
			}
		})
	}
}
