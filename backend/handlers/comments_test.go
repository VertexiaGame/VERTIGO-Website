package handlers

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestCommentKindParam(t *testing.T) {
	for raw, want := range map[string]string{
		"shop": models.CommentTargetShop, "item": models.CommentTargetShop, "Items": models.CommentTargetShop,
		"game": models.CommentTargetGame, "games": models.CommentTargetGame, "wall": "", "": "",
	} {
		if got := commentKindParam(raw); got != want {
			t.Errorf("commentKindParam(%q) = %q, want %q", raw, got, want)
		}
	}
	for raw, want := range map[string]string{"": "about", "servers": "servers", "comments": "comments", "store": "about"} {
		if got := gameDetailTab(raw); got != want {
			t.Errorf("gameDetailTab(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := models.CommentPostURL(models.CommentTargetGame, 4); got != "/play/4/comments" {
		t.Errorf("game post URL = %q", got)
	}
	if got := models.CommentPostURL(models.CommentTargetShop, 4); got != "/shop/item/4/comments" {
		t.Errorf("item post URL = %q", got)
	}
}

func TestCommentSectionRenders(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	section := &models.CommentSection{
		TargetType: models.CommentTargetGame, TargetID: 9, CanPost: true, IsLoggedIn: true, CSRF: "tok", HasMore: true,
		Comments: []*models.Comment{{
			ID: 5, UserID: 2, Username: "bob", Body: "<b>hi</b>", CreationDate: now, TimeAgo: "1m ago", CanDelete: true,
			Replies: []*models.Comment{{ID: 6, UserID: 3, ParentID: 5, Username: "carol", Body: "reply", CreationDate: now}},
		}, {ID: 4, UserID: 3, Username: "carol", Body: "older", CreationDate: now}},
	}
	var out bytes.Buffer
	if err := engine.Render(&out, "components/comments/section", section); err != nil {
		t.Fatal(err)
	}
	page := out.String()
	for _, want := range []string{
		`action="/play/9/comments"`, `name="csrf" value="tok"`, `name="parent_id"`,
		`id="comment-5"`, `id="comment-6"`, `data-comment-remove="5"`, `data-comment-reply="6"`,
		`&lt;b&gt;hi&lt;/b&gt;`, `@bob`, `data-before="4"`, `href="/user/2"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(page, `data-comment-remove="4"`) || strings.Contains(page, "<b>hi</b>") {
		t.Error("remove offered without permission, or body not escaped")
	}

	out.Reset()
	if err := engine.Render(&out, "components/comments/section", &models.CommentSection{TargetType: models.CommentTargetShop, TargetID: 1}); err != nil {
		t.Fatal(err)
	}
	page = out.String()
	if strings.Contains(page, "shpitmFeedBox") || !strings.Contains(page, "nobody has commented yet") || !strings.Contains(page, `href="/login"`) {
		t.Errorf("logged-out empty section rendered wrong:\n%s", page)
	}
}
