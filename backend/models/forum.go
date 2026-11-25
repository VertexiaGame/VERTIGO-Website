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

func (b *ForumBoard) ImageURL() string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(b.Name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			dash = false
		} else if !dash && sb.Len() > 0 {
			sb.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		return ""
	}
	return "/static/useful/pages/forum/" + slug + ".png"
}

func (b *ForumBoard) ImageStyle() template.CSS {
	u := b.ImageURL()
	if u == "" {
		return ""
	}
	return template.CSS("background-image: url('" + u + "');")
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
	Images            []int
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

func (t *ForumThread) QuoteExcerpt() string {
	if t == nil {
		return ""
	}
	return forumExcerpt(t.Body, ForumQuoteExcerptMax)
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

type ForumReply struct {
	ID, ThreadID, UserID                    int
	AuthorUsername, AuthorDisplayName, Body string
	CreatedAt                               time.Time
	Tag                                     *ClanTag
	Images                                  []int
	Quote                                   *ForumQuote
}

const ForumQuoteExcerptMax = 280

type ForumQuote struct {
	ReplyID                                 int
	UserID                                  int
	AuthorUsername, AuthorDisplayName, Body string
	Position int
	Missing bool
	Href string
}

func (q *ForumQuote) IsOriginal() bool { return q != nil && q.ReplyID == 0 }

func (q *ForumQuote) Anchor() string {
	if q == nil || q.ReplyID == 0 {
		return "thread"
	}
	return "reply-" + strconv.Itoa(q.ReplyID)
}

func (q *ForumQuote) Author() string {
	if q == nil {
		return "Player"
	}
	if strings.TrimSpace(q.AuthorDisplayName) != "" {
		return q.AuthorDisplayName
	}
	if strings.TrimSpace(q.AuthorUsername) != "" {
		return q.AuthorUsername
	}
	return "Player"
}

func (q *ForumQuote) Excerpt() string {
	if q == nil {
		return ""
	}
	return forumExcerpt(q.Body, ForumQuoteExcerptMax)
}

func forumExcerpt(s string, max int) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = strings.TrimRightFunc(line, unicode.IsSpace); line != "" {
			kept = append(kept, line)
		}
	}
	clean := strings.Join(kept, "\n")
	if utf8.RuneCountInString(clean) <= max {
		return clean
	}
	runes := []rune(clean)
	return strings.TrimRightFunc(string(runes[:max]), unicode.IsSpace) + "…"
}

func (r *ForumReply) Excerpt() string { return forumExcerpt(r.Body, ForumQuoteExcerptMax) }

func (r *ForumReply) Author() string {
	if strings.TrimSpace(r.AuthorDisplayName) != "" {
		return r.AuthorDisplayName
	}
	if strings.TrimSpace(r.AuthorUsername) != "" {
		return r.AuthorUsername
	}
	return "Player"
}
func (r *ForumReply) TimeAgo() string  { return RelativeClanTime(r.CreatedAt) }
func (r *ForumReply) FullDate() string { return r.CreatedAt.Format("Jan 2, 2006 at 3:04 PM") }
