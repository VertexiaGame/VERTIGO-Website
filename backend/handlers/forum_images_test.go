package handlers

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/models"
)

func TestForumImageUploads(t *testing.T) {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	pngBytes := pngData.Bytes()
	largePNG := make([]byte, 2<<20)
	copy(largePNG, pngBytes)
	app := fiber.New(fiber.Config{BodyLimit: forumImageRequestLimit})
	app.Use(RequestBodyLimit())
	app.Post("/forum/t/1/replies", func(c fiber.Ctx) error {
		images, err := forumImageUploads(c)
		if err != nil {
			return c.Status(400).SendString(err.Error())
		}
		for _, image := range images {
			if image.SavedID != 0 {
				continue
			}
			if image.ContentType != "image/png" || (!bytes.Equal(image.Content, pngBytes) && !bytes.Equal(image.Content, largePNG)) {
				t.Error("validated image did not retain its actual MIME type or content")
			}
		}
		return c.SendString(strconv.Itoa(len(images)))
	})
	for _, tc := range []struct {
		name   string
		files  [][]byte
		saved  []string
		status int
	}{
		{"text only", nil, nil, 200},
		{"three images", [][]byte{pngBytes, pngBytes, pngBytes}, nil, 200},
		{"three 2 MB images", [][]byte{largePNG, largePNG, largePNG}, nil, 200},
		{"four images", [][]byte{pngBytes, pngBytes, pngBytes, pngBytes}, nil, 400},
		{"invalid content", [][]byte{[]byte("<svg/>")}, nil, 400},
		{"truncated png", [][]byte{pngBytes[:40]}, nil, 400},
		{"oversized", [][]byte{make([]byte, (2<<20)+1)}, nil, 400},
		{"mixed uploads and saved images", [][]byte{pngBytes}, []string{"5", "6"}, 200},
		{"combined attachment limit", [][]byte{pngBytes, pngBytes}, []string{"5", "6"}, 400},
		{"duplicate saved image", nil, []string{"5", "5"}, 400},
		{"invalid saved image", nil, []string{"-5"}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			for _, id := range tc.saved {
				if err := form.WriteField("saved_images", id); err != nil {
					t.Fatal(err)
				}
			}
			for _, content := range tc.files {
				file, err := form.CreateFormFile("images", "upload.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write(content); err != nil {
					t.Fatal(err)
				}
			}
			if err := form.Close(); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/forum/t/1/replies", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("got status %d, want %d", resp.StatusCode, tc.status)
			}
			if tc.status == http.StatusOK {
				result, err := io.ReadAll(resp.Body)
				if err != nil || string(result) != strconv.Itoa(len(tc.files)+len(tc.saved)) {
					t.Fatalf("upload count: %q, %v", result, err)
				}
			}
		})
	}
}

func TestForumImageViews(t *testing.T) {
	engine := html.New("../../views", ".html")
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := engine.Render(&out, "components/forum/image_gallery", []int{11, 12, 13}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/forum/images/11", "/forum/images/13", `class="forum-gallery carcnt"`, `class="cartrk"`, `class="carsld"`, "Previous image", "Next image", "1 / 3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("gallery missing %q", want)
		}
	}
	out.Reset()
	data := fiber.Map{"Images": []models.PendingForumImage{{ID: 11, ThreadID: 8}}, "CSRF": "test-token"}
	if err := engine.Render(&out, "components/admin/forum_images", data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/admin/forum-images/11", "/forum/t/8", `value="approve"`, `value="reject"`, "test-token"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("admin queue missing %q", want)
		}
	}
	out.Reset()
	data = fiber.Map{"ActiveTab": "forumimages", "ForumImages": data, "TotalLogPages": 1,
		"TotalAssetPages": 1, "TotalShopItemPages": 1, "TotalBannerPages": 1, "TotalClanMediaPages": 1}
	if err := engine.Render(&out, "pages/admin", data); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="/admin?tab=forumimages"`, `id="tab-forumimages" class="settabcntnt active"`, `hx-history="false"`, "/admin/forum-images/11"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("admin tab missing %q", want)
		}
	}
}

func TestForumImagePreviewRequiresAdmin(t *testing.T) {
	app := fiber.New()
	app.Get("/admin/forum-images/:id", AdminForumImageGet)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/forum-images/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("unauthenticated preview: status %d, cache control %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
}
