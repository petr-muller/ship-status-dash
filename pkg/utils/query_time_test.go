package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOutagesDuringQueryBounds(t *testing.T) {
	t.Parallel()
	a := time.Date(2025, 6, 10, 12, 0, 0, 0, time.UTC)
	b := time.Date(2025, 6, 10, 18, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		start     string
		end       string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   string
	}{
		{
			name:      "start_only",
			start:     a.Format(time.RFC3339),
			wantStart: a,
			wantEnd:   a,
		},
		{
			name:      "end_only",
			end:       b.Format(time.RFC3339Nano),
			wantStart: b,
			wantEnd:   b,
		},
		{
			name:      "both_range",
			start:     a.Format(time.RFC3339),
			end:       b.Format(time.RFC3339Nano),
			wantStart: a,
			wantEnd:   b,
		},
		{
			name:      "both_equal",
			start:     a.Format(time.RFC3339),
			end:       a.Format(time.RFC3339),
			wantStart: a,
			wantEnd:   a,
		},
		{
			name:    "start_after_end",
			start:   b.Format(time.RFC3339),
			end:     a.Format(time.RFC3339),
			wantErr: "start must be before or equal to end",
		},
		{
			name:    "invalid_start",
			start:   "not-a-time",
			end:     a.Format(time.RFC3339),
			wantErr: "invalid start time",
		},
		{
			name:    "invalid_end_with_start",
			start:   a.Format(time.RFC3339),
			end:     "not-a-time",
			wantErr: "invalid end time",
		},
		{
			name:    "invalid_end_only",
			end:     "bogus",
			wantErr: "invalid end time",
		},
		{
			name:    "empty_both_hits_end_branch",
			wantErr: "invalid end time",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			qs, qe, errMsg := OutagesDuringQueryBounds(tt.start, tt.end)
			assert.Equal(t, tt.wantErr, errMsg)
			if tt.wantErr != "" {
				assert.True(t, qs.IsZero() && qe.IsZero())
				return
			}
			assert.True(t, tt.wantStart.Equal(qs), "got queryStart %v", qs)
			assert.True(t, tt.wantEnd.Equal(qe), "got queryEnd %v", qe)
		})
	}
}
