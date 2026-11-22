package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGameMediaSubmissionReplacesPendingOnly(t *testing.T) {
	old := gameMediaPendingDir
	gameMediaPendingDir = t.TempDir()
	defer func() { gameMediaPendingDir = old }()
	makeFile := func() *multipart.FileHeader {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("icon", "icon.png")
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, color.White)
		if err := png.Encode(part, img); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		return req.MultipartForm.File["icon"][0]
	}
	if err := submitGameMedia(17, "icon", []*multipart.FileHeader{makeFile()}); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadDir(gameMediaPendingDir)
	if len(first) != 1 {
		t.Fatalf("got %d submissions", len(first))
	}
	id, kind, ok := parseGameMediaToken(first[0].Name())
	if !ok || id != 17 || kind != "icon" {
		t.Fatal("invalid queue token")
	}
	if _, err := os.Stat(filepath.Join(gameMediaPendingDir, first[0].Name(), "1.png")); err != nil {
		t.Fatal(err)
	}
	if err := submitGameMedia(17, "icon", []*multipart.FileHeader{makeFile()}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadDir(gameMediaPendingDir)
	if len(second) != 1 || second[0].Name() == first[0].Name() {
		t.Fatal("replacement did not invalidate old submission")
	}
	if _, _, ok := parseGameMediaToken("17_icon_../escape"); ok {
		t.Fatal("accepted unsafe token")
	}
}
