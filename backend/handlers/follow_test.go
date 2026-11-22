package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestFollowListTemplate(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	data := fiber.Map{
		"TargetUser": &models.User{ID: 2, Username: "alice", DisplayName: "Alice"},
		"Followers":  []*models.FriendUserInfo{{UserID: 3, Username: "bob", DisplayName: "Bob"}},
		"Following":  []*models.FriendUserInfo{{UserID: 4, Username: "carol", DisplayName: "Carol"}},
		"AssetV":     "1", "ActiveTab": "followers",
	}
	var out bytes.Buffer
	if err := engine.Render(&out, "pages/follows", data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="title">Followers`, "/follows/2?tab=followers", "/follows/2?tab=following", "/user/3", "Bob", "@bob", "Carol", "Search users", `id="follow-tab-followers" class="settabcntnt active"`, `id="follow-tab-following" class="settabcntnt"`, "switchFollowTab", `hx-boost="false" onclick="return window.switchFollowTab`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	data["ActiveTab"], data["Following"] = "following", nil
	out.Reset()
	if err := engine.Render(&out, "pages/follows", data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `id="follow-tab-following" class="settabcntnt active"`) {
		t.Error("following tab was not selected")
	}
	if !strings.Contains(out.String(), "Not following anyone yet.") {
		t.Error("missing following empty state")
	}
}

func TestFollowUpdateTemplate(t *testing.T) {
	engine := html.New("../../views", ".html")
	for _, tc := range []struct {
		following bool
		path      string
		label     string
	}{
		{true, "/unfollow/2", "Unfollow User"},
		{false, "/follow/2", "Follow User"},
	} {
		var out bytes.Buffer
		data := fiber.Map{"ProfileUser": fiber.Map{"ID": 2}, "IsFollowing": tc.following, "CountAvailable": true, "FollowersCount": 0}
		if err := engine.Render(&out, "components/profile/follow_update", data); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{tc.path, tc.label, `hx-target="this" hx-swap="outerHTML"`, `id="profileFollowersCount" hx-swap-oob="innerHTML">0</strong>`} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("missing %q in follow update", want)
			}
		}
	}
}

func TestFollowRoutesRequireLogin(t *testing.T) {
	app := fiber.New()
	app.Get("/follows/:id", FollowsGet)
	app.Post("/follow/:id", FollowUserPost)
	app.Post("/unfollow/:id", FollowUserPost)
	for _, tc := range []struct{ method, path string }{{"GET", "/follows/2"}, {"POST", "/follow/2"}, {"POST", "/unfollow/2"}} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.method == http.MethodPost {
			req.Header.Set("HX-Request", "true")
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if tc.method == http.MethodGet && resp.StatusCode != http.StatusSeeOther {
			t.Errorf("%s: got %d", tc.path, resp.StatusCode)
		}
		if tc.method == http.MethodPost && (resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("HX-Redirect") != "/login") {
			t.Errorf("%s: got %d, redirect %q", tc.path, resp.StatusCode, resp.Header.Get("HX-Redirect"))
		}
	}
}
