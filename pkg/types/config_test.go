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

	t.Run("no filters all refs", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("", "", "", "")
		assert.ElementsMatch(t, []SubComponentRef{
			{ComponentSlug: "alpha", SubSlug: "one"},
			{ComponentSlug: "alpha", SubSlug: "two"},
			{ComponentSlug: "beta", SubSlug: "one"},
		}, got)
	})

	t.Run("component filter", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("alpha", "", "", "")
		assert.ElementsMatch(t, []SubComponentRef{
			{ComponentSlug: "alpha", SubSlug: "one"},
			{ComponentSlug: "alpha", SubSlug: "two"},
		}, got)
	})

	t.Run("team filter", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("", "", "", "team-b")
		assert.Equal(t, []SubComponentRef{{ComponentSlug: "beta", SubSlug: "one"}}, got)
	})

	t.Run("tag filter", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("", "", "net", "")
		assert.ElementsMatch(t, []SubComponentRef{
			{ComponentSlug: "alpha", SubSlug: "one"},
			{ComponentSlug: "beta", SubSlug: "one"},
		}, got)
	})

	t.Run("component and sub slug", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("beta", "one", "", "")
		assert.Equal(t, []SubComponentRef{{ComponentSlug: "beta", SubSlug: "one"}}, got)
	})

	t.Run("component tag and", func(t *testing.T) {
		got := cfg.SubComponentRefsMatching("alpha", "", "ci", "")
		assert.Equal(t, []SubComponentRef{{ComponentSlug: "alpha", SubSlug: "one"}}, got)
	})
}
