package handlers

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"vertexia-frontend/backend/storage"
)

// useTempStore points storage.Default at a fresh local store for one test.
func useTempStore(t *testing.T) *storage.LocalStore {
	t.Helper()
	s, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storage.SetDefault(s)
	t.Cleanup(func() { storage.SetDefault(nil) })
	return s
}

func TestGameMediaSubmissionReplacesPendingOnly(t *testing.T) {
	store := useTempStore(t)
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
	first, err := pendingGameMediaTokens()
	if err != nil || len(first) != 1 {
		t.Fatalf("got %d submissions (%v)", len(first), err)
	}
	var firstToken string
	for token, count := range first {
		firstToken = token
		if count != 1 {
			t.Fatalf("submission has %d images", count)
		}
	}
	id, kind, ok := parseGameMediaToken(firstToken)
	if !ok || id != 17 || kind != "icon" {
		t.Fatal("invalid queue token")
	}
	if ok, _ := storage.Exists(context.Background(), store, "uploads/game_media_pending/"+firstToken+"/1.png"); !ok {
		t.Fatal("pending image not stored under uploads/game_media_pending")
	}
	if err := submitGameMedia(17, "icon", []*multipart.FileHeader{makeFile()}); err != nil {
		t.Fatal(err)
	}
	second, _ := pendingGameMediaTokens()
	if _, stale := second[firstToken]; len(second) != 1 || stale {
		t.Fatal("replacement did not invalidate old submission")
	}
	for _, bad := range []string{"17_icon_../escape", "17_icon_1/../../x", "-1_icon_5", "+17_icon_5", "17_other_5", "17_icon_"} {
		if _, _, ok := parseGameMediaToken(bad); ok {
			t.Fatalf("accepted unsafe token %q", bad)
		}
		if _, err := gameMediaPendingKey(bad, 1); err == nil {
			t.Fatalf("built a key for unsafe token %q", bad)
		}
	}
}
