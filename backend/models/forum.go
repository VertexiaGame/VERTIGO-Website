package models

import (
	"html/template"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)
var forumTones = []int{277, 262, 292, 248, 305, 270}

type ForumBoard struct {
	ID          int
	Name        string
	Description string
}

func (b *ForumBoard) Path() string {
	return "/forum/s/" + strconv.Itoa(b.ID)
}

func (b *ForumBoard) Letter() string {
	for _, r := range strings.TrimSpace(b.Name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "#"
}

func (b *ForumBoard) ToneStyle() template.CSS {
	idx := (b.ID - 1) % len(forumTones)
	if idx < 0 {
		idx += len(forumTones)
	}
	return template.CSS("--fbd-h: " + strconv.Itoa(forumTones[idx]) + ";")
}

type ForumThread struct {
	ID                int
	BoardID           int
	UserID            int
	AuthorUsername    string
	AuthorDisplayName string
	Title             string
	Body              string
	RepliesCount      int
	Pinned            bool
	Locked            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Tag               *ClanTag
}

func (t *ForumThread) Path() string {
	if t == nil {
		return "/forum"
	}
	return "/forum/t/" + strconv.Itoa(t.ID)
}

func (t *ForumThread) Author() string {
	if t == nil {
		return "Player"
	}
	if strings.TrimSpace(t.AuthorDisplayName) != "" {
		return t.AuthorDisplayName
	}
	if strings.TrimSpace(t.AuthorUsername) != "" {
		return t.AuthorUsername
	}
	return "Player"
}

func (t *ForumThread) TimeAgo() string {
	if t == nil {
		return ""
	}
	return RelativeClanTime(t.CreatedAt)
}

func (t *ForumThread) DatePosted() string {
	if t == nil || t.CreatedAt.IsZero() {
		return "Recently"
	}
	now := time.Now()
	if t.CreatedAt.Year() == now.Year() {
		return t.CreatedAt.Format("Jan 2")
	}
	return t.CreatedAt.Format("Jan 2, 2006")
}

func (t *ForumThread) FullDate() string {
	if t == nil || t.CreatedAt.IsZero() {
		return ""
	}
	return t.CreatedAt.Format("Jan 2, 2006 at 3:04 PM")
}

func (t *ForumThread) RepliesLabel() string {
	if t == nil || t.RepliesCount == 0 {
		return "0 replies"
	}
	if t.RepliesCount == 1 {
		return "1 reply"
	}
	return strconv.Itoa(t.RepliesCount) + " replies"
}

func (t *ForumThread) Excerpt() string {
	if t == nil {
		return ""
	}
	clean := strings.Join(strings.Fields(strings.TrimSpace(t.Body)), " ")
	if utf8.RuneCountInString(clean) <= 140 {
		return clean
	}
	runes := []rune(clean)
	return strings.TrimSpace(string(runes[:140])) + "…"
}
