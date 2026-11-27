package models

import (
	"testing"
	"time"
)

func TestCompactCount(t *testing.T) {
	cases := map[int]string{
		-5:         "0",
		0:          "0",
		999:        "999",
		1000:       "1K",
		1234:       "1.2K",
		9999:       "9.9K",
		10_000:     "10K",
		345_678:    "345K",
		1_000_000:  "1M",
		5_670_000:  "5.6M",
		12_000_000: "12M",
	}
	for in, want := range cases {
		if got := CompactCount(in); got != want {
			t.Errorf("CompactCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestLastOnlineLabel(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		last time.Time
		want string
	}{
		{time.Time{}, ""},
		{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), ""},
		{now.Add(time.Minute), "Now"},
		{now.Add(-4 * time.Minute), "Now"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-59 * time.Minute), "59 minutes ago"},
		{now.Add(-time.Hour), "1 hour ago"},
		{now.Add(-23*time.Hour - 59*time.Minute), "23 hours ago"},
		{now.Add(-24 * time.Hour), "1 day ago"},
		{now.Add(-29 * 24 * time.Hour), "29 days ago"},
		{now.Add(-30 * 24 * time.Hour), "Oct 26, 2026"},
	}
	for _, c := range cases {
		if got := LastOnlineLabel(c.last, now); got != c.want {
			t.Errorf("LastOnlineLabel(%v) = %q, want %q", c.last, got, c.want)
		}
	}
}
