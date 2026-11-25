package models

import (
	"strconv"
	"strings"
)

type ProfileStats struct {
	JoinedLabel  string
	IsOnline     bool
	Status       string
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
