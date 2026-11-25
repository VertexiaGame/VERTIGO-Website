package handlers

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestForumQuoteID(t *testing.T) {
	if id, err := forumQuoteID("  "); err != nil || id != nil {
		t.Fatalf("empty: want nil, nil; got %v, %v", id, err)
	}
	if id, err := forumQuoteID("0"); err != nil || id == nil || *id != 0 {
		t.Fatalf("0: want original post; got %v, %v", id, err)
	}
	if id, err := forumQuoteID("42"); err != nil || id == nil || *id != 42 {
		t.Fatalf("42: got %v, %v", id, err)
	}
	for _, raw := range []string{"-1", "abc", "1.5"} {
		if _, err := forumQuoteID(raw); err == nil {
			t.Errorf("%q: want error", raw)
		}
	}
}

func TestAttachForumQuotes(t *testing.T) {
	thread := &models.ForumThread{ID: 8, UserID: 4, AuthorUsername: "op", Body: "Original body"}
	original := &models.ForumReply{ID: 1, Quote: &models.ForumQuote{ReplyID: 0}}
	samePage := &models.ForumReply{ID: 2, Quote: &models.ForumQuote{ReplyID: 1, Position: 0}}
	otherPage := &models.ForumReply{ID: 3, Quote: &models.ForumQuote{ReplyID: 99, Position: forumPageSize + 3}}
	missing := &models.ForumReply{ID: 4, Quote: &models.ForumQuote{ReplyID: 7, Missing: true}}
	attachForumQuotes(thread, []*models.ForumReply{original, samePage, otherPage, missing, {ID: 5}}, 1)

	if q := original.Quote; q.Href != "#thread" || q.Author() != "op" || q.Body != "Original body" || q.UserID != 4 {
		t.Errorf("original post quote = %+v", q)
	}
	if got := samePage.Quote.Href; got != "#reply-1" {
		t.Errorf("same page href = %q", got)
	}
	if got := otherPage.Quote.Href; got != "/forum/t/8?page=2#reply-99" {
		t.Errorf("other page href = %q", got)
	}
	if missing.Quote.Href != "" {
		t.Errorf("missing quote should have no link, got %q", missing.Quote.Href)
	}
}

func TestForumThreadRendersQuotes(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	board := &models.ForumBoard{ID: 2, Name: "General"}
	thread := &models.ForumThread{ID: 8, BoardID: 2, UserID: 4, AuthorUsername: "op", Title: "Hello", Body: "Message", CreatedAt: time.Now()}
	replies := []*models.ForumReply{
		{ID: 3, UserID: 5, AuthorUsername: "first", Body: "First reply", CreatedAt: time.Now()},
		{ID: 4, UserID: 6, Body: "Agreed", CreatedAt: time.Now(),
			Quote: &models.ForumQuote{ReplyID: 3, AuthorUsername: "first", Body: "First reply", Href: "#reply-3"}},
		{ID: 5, UserID: 6, Body: "Gone", CreatedAt: time.Now(), Quote: &models.ForumQuote{ReplyID: 9, Missing: true}},
	}
	data := fiber.Map{"AssetV": "1", "Board": board, "Thread": thread, "Replies": replies, "IsLoggedIn": true, "CSRF": "token",
		"Page": 1, "Pages": 1, "PrevPage": 0, "NextPage": 2, "FlashError": ""}

	var out bytes.Buffer
	if err := engine.Render(&out, "pages/forum_thread", data); err != nil {
		t.Fatal(err)
	}
	page := out.String()
	for _, want := range []string{
		`name="quote_id" id="forumReplyQuoteId"`,
		`data-forum-quote="0"`,
		`data-forum-quote="3"`,
		`class="forum-quote"`,
		`data-forum-quote-jump="reply-3"`,
		`href="#reply-3"`,
		"Original reply unavailable",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q", want)
		}
	}

	thread.Locked = true
	out.Reset()
	if err := engine.Render(&out, "pages/forum_thread", data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "data-forum-quote=") {
		t.Error("locked thread should not offer quote buttons")
	}
}
