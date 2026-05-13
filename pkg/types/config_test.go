package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardConfig_SubComponentRefsMatching(t *testing.T) {
	cfg := &DashboardConfig{
		Components: []*Component{
			{
				Name: "Alpha", Slug: "alpha", ShipTeam: "team-a",
				Subcomponents: []SubComponent{
					{Name: "One", Slug: "one", Tags: []string{"net", "ci"}},
					{Name: "Two", Slug: "two"},
				},
			},
			{
				Name: "Beta", Slug: "beta", ShipTeam: "team-b",
				Subcomponents: []SubComponent{
					{Name: "One", Slug: "one", Tags: []string{"net"}},
				},
			},
		},
	}

	tests := []struct {
		name            string
		componentFilter string
		subFilter       string
		tagFilter       string
		teamFilter      string
		expected        []SubComponentRef
	}{
		{
			name:     "no filters all refs",
			expected: []SubComponentRef{{ComponentSlug: "alpha", SubSlug: "one"}, {ComponentSlug: "alpha", SubSlug: "two"}, {ComponentSlug: "beta", SubSlug: "one"}},
		},
		{
			name:            "component filter",
			componentFilter: "alpha",
			expected:        []SubComponentRef{{ComponentSlug: "alpha", SubSlug: "one"}, {ComponentSlug: "alpha", SubSlug: "two"}},
		},
		{
			name:       "team filter",
			teamFilter: "team-b",
			expected:   []SubComponentRef{{ComponentSlug: "beta", SubSlug: "one"}},
		},
		{
			name:      "tag filter",
			tagFilter: "net",
			expected:  []SubComponentRef{{ComponentSlug: "alpha", SubSlug: "one"}, {ComponentSlug: "beta", SubSlug: "one"}},
		},
		{
			name:            "component and sub slug",
			componentFilter: "beta",
			subFilter:       "one",
			expected:        []SubComponentRef{{ComponentSlug: "beta", SubSlug: "one"}},
		},
		{
			name:            "component tag and",
			componentFilter: "alpha",
			tagFilter:       "ci",
			expected:        []SubComponentRef{{ComponentSlug: "alpha", SubSlug: "one"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.SubComponentRefsMatching(tt.componentFilter, tt.subFilter, tt.tagFilter, tt.teamFilter)
			assert.ElementsMatch(t, tt.expected, got)
		})
	}
}
