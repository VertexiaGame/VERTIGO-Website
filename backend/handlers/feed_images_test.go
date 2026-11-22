package handlers

import (
	"bytes"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestFeedImageUploadValidation(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	fullSize := make([]byte, 2<<20)
	copy(fullSize, encoded.Bytes())
	app := fiber.New(fiber.Config{BodyLimit: forumImageRequestLimit})
	app.Use(RequestBodyLimit(), session.New(), func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		sess.Set("username", "uploader")
		sess.Set("csrf", "test-token")
		return c.Next()
	})
	app.Post("/api/v1/feed", PostFeed)
	for _, tc := range []struct {
		name, csrf, message string
		images              [][]byte
		status              int
	}{
		{"three full-size images", "test-token", "Feed is unavailable", [][]byte{fullSize, fullSize, fullSize}, http.StatusServiceUnavailable},
		{"too many", "test-token", "attach up to 3 images", [][]byte{encoded.Bytes(), encoded.Bytes(), encoded.Bytes(), encoded.Bytes()}, http.StatusBadRequest},
		{"too large", "test-token", "each image must be 2 MB or smaller", [][]byte{make([]byte, (2<<20)+1)}, http.StatusBadRequest},
		{"invalid CSRF", "", "Invalid CSRF token", nil, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			if err := form.WriteField("csrf", tc.csrf); err != nil {
				t.Fatal(err)
			}
			for _, data := range tc.images {
				file, err := form.CreateFormFile("images", "attachment.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write(data); err != nil {
					t.Fatal(err)
				}
			}
			if err := form.Close(); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/feed", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			req.Header.Set("Accept", "application/json")
			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var result bytes.Buffer
			if _, err := result.ReadFrom(response.Body); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status || !strings.Contains(result.String(), tc.message) {
				t.Fatalf("response: %d %s", response.StatusCode, result.String())
			}
		})
	}
}

func TestFeedImageView(t *testing.T) {
	engine := html.New("../../views", ".html")
	var out bytes.Buffer
	data := fiber.Map{
		"CSRF": "test-token",
		"Feed": []FeedPost{{ID: 1, FeedType: "friends", Images: []int{12, 13}}},
	}
	if err := engine.Render(&out, "components/dashboard/feed", data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`enctype="multipart/form-data"`,
		`name="images" accept="image/png,image/jpeg" multiple`,
		`data-feed-type="friends"`,
		"test-token", "/feed/images/12", "/feed/images/13", "Previous image", "Next image", "1 / 2",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("feed missing %q", want)
		}
	}
}

func TestFeedImageModerationRequiresAdmin(t *testing.T) {
	app := fiber.New()
	app.Get("/admin/feed-images/:id", AdminFeedImageGet)
	app.Post("/admin/feed-images/:id", AdminFeedImageReviewPost)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		res, err := app.Test(httptest.NewRequest(method, "/admin/feed-images/1", nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized || res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s returned %d, cache control %q", method, res.StatusCode, res.Header.Get("Cache-Control"))
		}
	}
}

func TestAdminFeedImageQueueView(t *testing.T) {
	engine := html.New("../../views", ".html")
	var out bytes.Buffer
	data := fiber.Map{
		"ActiveTab": "feedimages",
		"FeedImages": fiber.Map{
			"Images": []models.PendingFeedImage{{ID: 21, FeedID: 12, FeedType: "friends", Author: "poster"}},
			"CSRF":   "test-token",
			"Next":   21,
		},
		"TotalLogPages": 1, "TotalAssetPages": 1, "TotalShopItemPages": 1, "TotalBannerPages": 1, "TotalClanMediaPages": 1,
	}
	if err := engine.Render(&out, "pages/admin", data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Feed Image Approvals", "Friends", "Feed post #12", "/admin/feed-images/21",
		"test-token", "@poster", "value=\"approve\"", "value=\"reject\"", "tab=feedimages&amp;after=21", "hx-history=\"false\""} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("queue missing %q", want)
		}
	}
}
