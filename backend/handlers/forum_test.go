package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestForumTemplatesConnectActions(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	board := &models.ForumBoard{ID: 2, Name: "General"}
	thread := &models.ForumThread{ID: 8, BoardID: 2, UserID: 4, Title: "Hello", Body: "Message", CreatedAt: time.Now()}
	data := fiber.Map{"AssetV": "1", "Board": board, "Boards": []*models.ForumBoard{board}, "Thread": thread, "Threads": []*models.ForumThread{thread}, "Replies": []*models.ForumReply{{ID: 3, UserID: 5, Body: "Reply", CreatedAt: time.Now()}}, "IsLoggedIn": true, "CSRF": "token", "ThreadCount": 1, "Sort": "newest", "Page": 1, "Pages": 1, "PrevPage": 0, "NextPage": 2, "FlashError": "", "FlashSuccess": ""}
	for name, expected := range map[string][]string{
		"pages/forum":            {"/forum/s/2", "/forum/mine", "My Threads"},
		"pages/forum_board":      {"/forum/t/8", `name="csrf" value="token"`, `/forum/s/2/threads`, `/avatar/4.png`, `class="forum-post forum-thread-item"`},
		"pages/forum_thread":     {`/forum/t/8/replies`, `name="csrf" value="token"`, "Reply", `/avatar/4.png`, `/avatar/5.png`},
		"pages/forum_my_threads": {"My Threads", "/forum/t/8", `class="forum-post forum-thread-item"`},
	} {
		var out bytes.Buffer
		if err := engine.Render(&out, name, data); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range expected {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: missing %q", name, want)
			}
		}
	}
}

func TestForumMyThreadsGuestRedirect(t *testing.T) {
	app := fiber.New()
	app.Get("/forum/mine", ForumMyThreadsGet)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/forum/mine", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || resp.StatusCode != http.StatusSeeOther || location.Path != "/login" || location.Query().Get("next") != "/forum/mine" {
		t.Fatalf("guest redirect: status %d, location %q, error %v", resp.StatusCode, resp.Header.Get("Location"), err)
	}
}

func TestForumMyThreadsEmptyAndPagination(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	data := fiber.Map{"AssetV": "1", "ThreadCount": 0, "Page": 1, "Pages": 1}
	var out bytes.Buffer
	if err := engine.Render(&out, "pages/forum_my_threads", data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "haven't posted any threads") {
		t.Fatal("missing empty state")
	}
	data["Threads"] = []*models.ForumThread{{ID: 8, UserID: 4, Title: "Hello", Body: "Message", CreatedAt: time.Now()}}
	data["ThreadCount"], data["Pages"], data["NextPage"] = 51, 2, 2
	out.Reset()
	if err := engine.Render(&out, "pages/forum_my_threads", data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/forum/mine?page=2") || !strings.Contains(out.String(), "/forum/t/8") {
		t.Fatal("missing thread or next page link")
	}
}

func TestForumPaginationBounds(t *testing.T) {
	for _, tc := range []struct {
		raw                string
		total, page, pages int
	}{{"", 0, 1, 1}, {"-1", 51, 1, 2}, {"99", 51, 2, 2}, {"2", 100, 2, 2}} {
		page, pages := forumPage(tc.raw, tc.total)
		if page != tc.page || pages != tc.pages {
			t.Errorf("forumPage(%q, %d) = %d, %d", tc.raw, tc.total, page, pages)
		}
	}
}

func TestForumThreadReplyStates(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	thread := &models.ForumThread{ID: 8, UserID: 4, Title: "Hello", Body: "Hello world", CreatedAt: time.Now()}
	data := fiber.Map{"Board": &models.ForumBoard{ID: 2, Name: "General"}, "Thread": thread, "Pages": 1, "IsLoggedIn": false, "FlashError": "", "AssetV": "1"}
	render := func() string {
		var out bytes.Buffer
		if err := engine.Render(&out, "pages/forum_thread", data); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if out := render(); !strings.Contains(out, "Log in to reply") || strings.Contains(out, "id=\"ftdReplyForm\"") {
		t.Fatal("guest reply state incorrect")
	}
	thread.Locked = true
	if out := render(); !strings.Contains(out, "This thread is locked") || strings.Contains(out, "id=\"ftdReplyForm\"") {
		t.Fatal("locked reply state incorrect")
	}
	thread.Locked = false
	data["IsLoggedIn"] = true
	data["CSRF"] = "token"
	data["FlashError"] = "Please try again"
	if out := render(); !strings.Contains(out, "forum-reply-compose\" open") || !strings.Contains(out, "Please try again") {
		t.Fatal("error did not leave composer open")
	}
}
