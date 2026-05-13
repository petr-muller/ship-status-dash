package utils

import (
	"errors"
	"strings"
	"time"
)

// ParseRFC3339OrNanoUTC parses s as RFC3339Nano or RFC3339 and returns the instant in UTC.
func ParseRFC3339OrNanoUTC(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time value")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// OutagesDuringQueryBounds computes [queryStart, queryEnd] for an overlap-style outage query.
// Caller must ensure at least one of startStr or endStr is non-empty.
func OutagesDuringQueryBounds(startStr, endStr string) (queryStart, queryEnd time.Time, errMsg string) {
	if startStr != "" {
		st, err := ParseRFC3339OrNanoUTC(startStr)
		if err != nil {
			return time.Time{}, time.Time{}, "invalid start time"
		}
		if endStr == "" {
			return st, st, ""
		}
		en, err := ParseRFC3339OrNanoUTC(endStr)
		if err != nil {
			return time.Time{}, time.Time{}, "invalid end time"
		}
		if st.After(en) {
			return time.Time{}, time.Time{}, "start must be before or equal to end"
		}
		return st, en, ""
	}
	en, err := ParseRFC3339OrNanoUTC(endStr)
	if err != nil {
		return time.Time{}, time.Time{}, "invalid end time"
	}
	return en, en, ""
}
