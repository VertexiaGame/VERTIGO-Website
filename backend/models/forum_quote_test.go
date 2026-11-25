package models

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestForumQuoteExcerpt(t *testing.T) {
	q := &ForumQuote{ReplyID: 3, Body: "  line one\r\n\r\n\r\n\r\nline two  \n"}
	if got := q.Excerpt(); got != "line one\nline two" {
		t.Errorf("excerpt = %q", got)
	}
	long := &ForumQuote{Body: strings.Repeat("é", ForumQuoteExcerptMax+50)}
	got := long.Excerpt()
	if !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) != ForumQuoteExcerptMax+1 {
		t.Errorf("long excerpt has %d runes", utf8.RuneCountInString(got))
	}
}

func TestForumQuoteAnchorAndAuthor(t *testing.T) {
	original := &ForumQuote{ReplyID: 0, AuthorUsername: "op"}
	if !original.IsOriginal() || original.Anchor() != "thread" || original.Author() != "op" {
		t.Errorf("original post quote: %v %q %q", original.IsOriginal(), original.Anchor(), original.Author())
	}
	reply := &ForumQuote{ReplyID: 12, AuthorUsername: "user", AuthorDisplayName: "Display"}
	if reply.IsOriginal() || reply.Anchor() != "reply-12" || reply.Author() != "Display" {
		t.Errorf("reply quote: %v %q %q", reply.IsOriginal(), reply.Anchor(), reply.Author())
	}
	if (&ForumQuote{}).Author() != "Player" {
		t.Error("empty author should fall back to Player")
	}
}
