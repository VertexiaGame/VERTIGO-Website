package models

import (
	"strconv"
	"strings"
	"time"
)

type ProfileStats struct {
	JoinedLabel  string
	IsOnline     bool
	Status       string
	LastOnline   string
	ForumThreads int
	ForumReplies int
	Games        int
	GameVisits   int
}

func CompactCount(n int) string {
	if n < 0 {
		n = 0
	}
	format := func(v float64, suffix string) string {
		s := strconv.FormatFloat(v, 'f', 1, 64)
		s = strings.TrimSuffix(s, ".0")
		return s + suffix
	}
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 10_000:
		return format(float64(n/100)/10, "K")
	case n < 1_000_000:
		return strconv.Itoa(n/1000) + "K"
	case n < 10_000_000:
		return format(float64(n/100_000)/10, "M")
	default:
		return strconv.Itoa(n/1_000_000) + "M"
	}
}

func (s ProfileStats) GameVisitsLabel() string   { return CompactCount(s.GameVisits) }
func (s ProfileStats) ForumThreadsLabel() string { return CompactCount(s.ForumThreads) }
func (s ProfileStats) ForumRepliesLabel() string { return CompactCount(s.ForumReplies) }

func LastOnlineLabel(last, now time.Time) string {
	if last.IsZero() || last.Year() < 2000 {
		return ""
	}
	plural := func(n int, unit string) string {
		if n == 1 {
			return "1 " + unit + " ago"
		}
		return strconv.Itoa(n) + " " + unit + "s ago"
	}
	since := now.Sub(last)
	switch {
	case since < 5*time.Minute:
		return "Now"
	case since < time.Hour:
		return plural(int(since/time.Minute), "minute")
	case since < 24*time.Hour:
		return plural(int(since/time.Hour), "hour")
	case since < 30*24*time.Hour:
		return plural(int(since/(24*time.Hour)), "day")
	default:
		return last.Format("Jan 2, 2006")
	}
}
