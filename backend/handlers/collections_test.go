package handlers

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
)

func TestCollectionsTemplate(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ tab, title, grid, create string }{
		{"assets", "My Assets", "assetGrid", "/create?type=asset"},
		{"items", "My Items", "itemGrid", "/create?type=shop"},
		{"games", "My Games", "myGamesGrid", "/create?type=game"},
		{"clans", "My Clans", "myClansGrid", "/create?type=clan"},
	} {
		t.Run(tc.tab, func(t *testing.T) {
			var out bytes.Buffer
			if err := engine.Render(&out, "pages/assets", fiber.Map{"ActiveTab": tc.tab, "AssetV": "1"}); err != nil {
				t.Fatal(err)
			}
			rendered := out.String()
			for _, want := range []string{tc.title, `id="` + tc.grid + `"`, tc.create, "/create/assets?tab=items", "/create/assets?tab=games", "/create/assets?tab=clans"} {
				if !strings.Contains(rendered, want) {
					t.Errorf("missing %q", want)
				}
			}
			for _, panel := range []string{"assets", "items", "games", "clans"} {
				if !strings.Contains(rendered, `id="collection-`+panel+`"`) {
					t.Errorf("missing %s panel", panel)
				}
			}
			if !strings.Contains(rendered, `id="collection-`+tc.tab+`" class="settabcntnt active"`) {
				t.Errorf("%s panel not active", tc.tab)
			}
			if strings.Contains(rendered, `hx-get="/create/assets?tab=`) {
				t.Error("tab navigation should not request the page")
			}
			if strings.Count(rendered, `aria-current="page"`) != 1 {
				t.Error("expected exactly one active tab")
			}
		})
	}
}

func TestCollectionsRequireLogin(t *testing.T) {
	app := fiber.New()
	app.Get("/create/assets", AssetsPage)
	for _, tab := range []string{"assets", "items", "games", "clans"} {
		resp, err := app.Test(httptest.NewRequest("GET", "/create/assets?tab="+tab, nil))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.Header.Get("Location") != "/login" {
			t.Errorf("%s: got redirect %q", tab, resp.Header.Get("Location"))
		}
	}
}
