package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/storage"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngWithHeaderSize rewrites the IHDR dimensions of a valid PNG, keeping the
// CRC valid, to simulate a decompression bomb without allocating it.
func pngWithHeaderSize(t *testing.T, w, h uint32) []byte {
	t.Helper()
	data := testPNG(t, 1, 1)
	// 8-byte signature, 4-byte length, "IHDR", then width and height.
	binary.BigEndian.PutUint32(data[16:20], w)
	binary.BigEndian.PutUint32(data[20:24], h)
	crc := crc32.ChecksumIEEE(data[12:29])
	binary.BigEndian.PutUint32(data[29:33], crc)
	return data
}

func doRequest(t *testing.T, app *fiber.App, method, target string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

func storageTestApp() *fiber.App {
	app := fiber.New()
	for _, mount := range []string{"/static/uploads/*", "/static/renders/*", "/assets/uploads/*"} {
		app.Get(mount, StoredFileGet)
	}
	// Registered as a route (not Use) so the automatic HEAD routes keep the
	// same order as the GET routes.
	app.Get("/*", func(c fiber.Ctx) error { return c.Status(fiber.StatusTeapot).SendString("next") })
	return app
}

func TestStoredFileGetServesPublicKeys(t *testing.T) {
	store := useTempStore(t)
	ctx := context.Background()
	pngData := testPNG(t, 2, 2)
	for _, key := range []string{
		"static/uploads/games/7_0.png",
		"static/uploads/games/thumbs/7_2.png",
		"static/uploads/banners/4.png",
		"static/uploads/clans/1_icon_abc.png",
		"static/renders/shop/hat/5.png",
		"static/renders/outfits/3.png",
		"assets/uploads/shop/hat/5.png",
		// Private keys must never be served by the public route.
		"static/renders/meshes/9.png",
		"static/uploads/../uploads/assets/1/a.png",
	} {
		if k, err := storage.Key(key); err == nil {
			if err := storage.PutBytes(ctx, store, k, pngData, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := storage.PutBytes(ctx, store, "uploads/assets/1/a.png", pngData, ""); err != nil {
		t.Fatal(err)
	}
	if err := storage.PutBytes(ctx, store, "assets/uploads/shop/hat/5.obj", []byte("v 0 0 0\n"), ""); err != nil {
		t.Fatal(err)
	}
	app := storageTestApp()

	resp, body := doRequest(t, app, "GET", "/static/uploads/games/7_0.png")
	if resp.StatusCode != 200 || !bytes.Equal(body, pngData) {
		t.Fatalf("public GET = %d (%d bytes)", resp.StatusCode, len(body))
	}
	if resp.Header.Get("Content-Type") != "image/png" || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "max-age=") || resp.Header.Get("ETag") == "" ||
		resp.Header.Get("Last-Modified") == "" || resp.ContentLength != int64(len(pngData)) {
		t.Fatalf("headers = %v (length %d)", resp.Header, resp.ContentLength)
	}
	etag := resp.Header.Get("ETag")
	if resp, _ := doRequest(t, app, "GET", "/static/uploads/games/7_0.png", "If-None-Match", etag); resp.StatusCode != 304 {
		t.Fatalf("conditional GET = %d", resp.StatusCode)
	}
	if resp, body := doRequest(t, app, "HEAD", "/static/uploads/games/7_0.png"); resp.StatusCode != 200 || len(body) != 0 {
		t.Fatalf("HEAD = %d, %d bytes", resp.StatusCode, len(body))
	}
	if resp, _ := doRequest(t, app, "GET", "/assets/uploads/shop/hat/5.obj"); resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("shop model = %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, p := range []string{
		"/static/uploads/games/thumbs/7_2.png",
		"/static/uploads/banners/4.png",
		"/static/uploads/clans/1_icon_abc.png",
		"/static/renders/shop/hat/5.png",
		"/static/renders/outfits/3.png",
		"/assets/uploads/shop/hat/5.png",
	} {
		if resp, _ := doRequest(t, app, "GET", p); resp.StatusCode != 200 {
			t.Errorf("GET %s = %d", p, resp.StatusCode)
		}
	}
	// Missing, private and traversal paths fall through to the next handler.
	for _, p := range []string{
		"/static/uploads/games/404.png",
		"/static/renders/meshes/9.png",
		"/static/renders/avatars/full/1.png",
		"/static/uploads/../../uploads/assets/1/a.png",
		"/static/uploads/%2e%2e/%2e%2e/uploads/assets/1/a.png",
		"/static/uploads/games/..%2f..%2f..%2fuploads/assets/1/a.png",
		"/static/uploads/games\\..\\..\\uploads/assets/1/a.png",
		"/assets/uploads/../../uploads/assets/1/a.png",
	} {
		resp, body := doRequest(t, app, "GET", p)
		if resp.StatusCode != fiber.StatusTeapot || string(body) != "next" {
			t.Errorf("GET %s = %d %q, want fall-through", p, resp.StatusCode, body)
		}
	}
}

type publicURLStore struct{ storage.Store }

func (publicURLStore) PublicURL(key string) (string, bool) {
	return "https://cdn.example.com/" + key, true
}

func TestStoredFileGetRedirectsToPublicURL(t *testing.T) {
	local := useTempStore(t)
	storage.SetDefault(publicURLStore{local})
	if err := storage.PutBytes(context.Background(), local, "static/uploads/games/7.png", testPNG(t, 1, 1), ""); err != nil {
		t.Fatal(err)
	}
	app := storageTestApp()
	resp, _ := doRequest(t, app, "GET", "/static/uploads/games/7.png")
	if resp.StatusCode != fiber.StatusFound || resp.Header.Get("Location") != "https://cdn.example.com/static/uploads/games/7.png" {
		t.Fatalf("redirect = %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	// A missing object still falls through so render-on-miss handlers run.
	if resp, _ := doRequest(t, app, "GET", "/static/uploads/games/8.png"); resp.StatusCode != fiber.StatusTeapot {
		t.Fatalf("missing object = %d", resp.StatusCode)
	}
	// Private keys are never redirected to the public bucket URL.
	if err := storage.PutBytes(context.Background(), local, "static/renders/meshes/1.png", testPNG(t, 1, 1), ""); err != nil {
		t.Fatal(err)
	}
	if resp, _ := doRequest(t, app, "GET", "/static/renders/meshes/1.png"); resp.StatusCode != fiber.StatusTeapot {
		t.Fatalf("private key = %d", resp.StatusCode)
	}
}

func TestShopRenderRejectsTraversal(t *testing.T) {
	store := useTempStore(t)
	cached := testPNG(t, 3, 3)
	if err := storage.PutBytes(context.Background(), store, "static/renders/shop/hat/5.png", cached, ""); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/avatar/shop/:type/:id", ShopRenderGet)
	app.Get("/avatar/outfit/:id", AvatarOutfitGet)
	for _, p := range []string{
		"/avatar/shop/../5.png",
		"/avatar/shop/%2e%2e/5.png",
		"/avatar/shop/..%2f..%2fuploads/5.png",
		"/avatar/shop/etc/5.png",
		"/avatar/shop/hat.png/5.png",
		"/avatar/shop/hat/..",
		"/avatar/shop/hat/5.png%2f..",
		"/avatar/shop/hat/-5.png",
		"/avatar/shop/hat/+5.png",
		"/avatar/shop/hat/5x.png",
		"/avatar/shop/hat/0.png",
		"/avatar/shop/hat/9999999999.png",
		"/avatar/outfit/..",
		"/avatar/outfit/1%2f..%2f..%2fx",
		"/avatar/outfit/-1",
		"/avatar/outfit/1.png.png",
	} {
		resp, _ := doRequest(t, app, "GET", p)
		if resp.StatusCode != fiber.StatusBadRequest && resp.StatusCode != fiber.StatusNotFound {
			t.Errorf("GET %s = %d, want 400", p, resp.StatusCode)
		}
	}
	// Plural and singular category names map to the same allowlisted key.
	for _, p := range []string{"/avatar/shop/hat/5.png", "/avatar/shop/hats/5", "/avatar/shop/HAT/5.png"} {
		resp, body := doRequest(t, app, "GET", p)
		if resp.StatusCode != 200 || !bytes.Equal(body, cached) {
			t.Errorf("GET %s = %d", p, resp.StatusCode)
		}
	}
}

func TestGameserverShopFileRejectsTraversal(t *testing.T) {
	store := useTempStore(t)
	old := config.Global
	config.Global = &config.Config{GameserverAPIKey: "test-gameserver-key"}
	t.Cleanup(func() { config.Global = old })
	texture := testPNG(t, 2, 2)
	if err := storage.PutBytes(context.Background(), store, "assets/uploads/shop/hat/5.png", texture, ""); err != nil {
		t.Fatal(err)
	}
	if err := storage.PutBytes(context.Background(), store, "uploads/shop/gear/6.obj", []byte("v 0 0 0\n"), ""); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/gameserver/shop/texture/:type/:id", GameserverShopTextureGet)
	app.Get("/gameserver/shop/mesh/:type/:id", GameserverShopMeshGet)
	key := []string{"X-Gameserver-Key", "test-gameserver-key"}
	for _, p := range []string{
		"/gameserver/shop/texture/../5.png",
		"/gameserver/shop/texture/..%2f..%2fuploads/5.png",
		"/gameserver/shop/texture/secret/5.png",
		"/gameserver/shop/texture/hat/5.png%2f..",
		"/gameserver/shop/texture/hat/-5",
		"/gameserver/shop/mesh/hat/5.obj.obj",
	} {
		if resp, _ := doRequest(t, app, "GET", p, key...); resp.StatusCode != fiber.StatusBadRequest && resp.StatusCode != fiber.StatusNotFound {
			t.Errorf("GET %s = %d", p, resp.StatusCode)
		}
	}
	resp, body := doRequest(t, app, "GET", "/gameserver/shop/texture/hats/5.png", key...)
	if resp.StatusCode != 200 || !bytes.Equal(body, texture) || resp.Header.Get("X-Avatar-Asset-Type") != "hat" {
		t.Fatalf("texture = %d %v", resp.StatusCode, resp.Header)
	}
	if resp, _ := doRequest(t, app, "GET", "/gameserver/shop/mesh/tool/6", key...); resp.StatusCode != 200 {
		t.Fatalf("legacy mesh location = %d", resp.StatusCode)
	}
	if resp, _ := doRequest(t, app, "GET", "/gameserver/shop/mesh/gear/7", key...); resp.StatusCode != 404 {
		t.Fatalf("missing mesh = %d", resp.StatusCode)
	}
}

func TestStorageKeyHelpersRejectTraversal(t *testing.T) {
	for _, fp := range []string{"../x", "1/../../x", "/etc/passwd", "1\\..\\x", "", "1/../../../uploads/games/vrtx/1.vrtx"} {
		if key, err := assetFileKey(fp); err == nil {
			t.Errorf("assetFileKey(%q) = %q", fp, key)
		}
	}
	if key, err := assetFileKey("12/abc.obj"); err != nil || key != "uploads/assets/12/abc.obj" {
		t.Fatalf("assetFileKey = %q, %v", key, err)
	}
	for _, u := range []string{
		"/static/uploads/clans/..",
		"/static/uploads/clans/../banners/1.png",
		"/static/uploads/clans/../../../uploads/assets/1/a.png",
		"/static/uploads/other/1_icon_abc.png",
		"https://evil.example/static/uploads/clans/1_icon_abc.png",
		"/static/uploads/clans/1_icon_abc.png/..",
		"/static/uploads/clans/1_avatar_abc.png",
	} {
		if key := clanMediaFilePath(u); key != "" {
			t.Errorf("clanMediaFilePath(%q) = %q", u, key)
		}
	}
	if key := clanMediaFilePath("/static/uploads/clans/12_icon_k9x2.png?v=17"); key != "static/uploads/clans/12_icon_k9x2.png" {
		t.Fatalf("clanMediaFilePath valid = %q", key)
	}
	for _, it := range []string{"../hat", "hat/..", "", "all", "hats"} {
		if key, err := shopItemKey(it, 5, ".png"); err == nil {
			t.Errorf("shopItemKey(%q) = %q", it, key)
		}
	}
	if key, err := shopItemKey("gear", 5, ".obj"); err != nil || key != "assets/uploads/shop/gear/5.obj" {
		t.Fatalf("shopItemKey = %q, %v", key, err)
	}
	if models.BannerImageKey(4) != "static/uploads/banners/4.png" || gameIconPath(7) != "static/uploads/games/7.png" ||
		gameThumbPath(7, 1) != "static/uploads/games/thumbs/7.png" || gameThumbPath(7, 3) != "static/uploads/games/thumbs/7_3.png" ||
		gameVrtxDiskPath(9) != "uploads/games/vrtx/9.vrtx" {
		t.Fatal("legacy key layout changed")
	}
	for _, raw := range []string{"5", "5.png"} {
		if id, ok := parseStrictID(raw, ".png"); !ok || id != 5 {
			t.Errorf("parseStrictID(%q) = %d, %v", raw, id, ok)
		}
	}
	for _, raw := range []string{"", "0", "-1", "+1", " 1", "1 ", "1.png.png", "1/..", "0x10", "1e3", "12345678901"} {
		if _, ok := parseStrictID(raw, ".png"); ok {
			t.Errorf("parseStrictID(%q) accepted", raw)
		}
	}
}

func TestUploadImageValidation(t *testing.T) {
	pngData := testPNG(t, 4, 4)
	var jpg, gifData bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifData, image.NewPaletted(image.Rect(0, 0, 4, 4), []color.Color{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	if f, _, err := sniffImage(pngData, imgPNG, imgJPEG); err != nil || f != imgPNG {
		t.Fatalf("png = %q, %v", f, err)
	}
	if f, _, err := sniffImage(jpg.Bytes(), imgPNG, imgJPEG); err != nil || f != imgJPEG {
		t.Fatalf("jpeg = %q, %v", f, err)
	}
	if _, _, err := sniffImage(gifData.Bytes(), imgPNG, imgJPEG); err == nil {
		t.Fatal("gif accepted where only png/jpeg are allowed")
	}
	if f, _, err := sniffImage(gifData.Bytes(), imgPNG, imgJPEG, imgGIF, imgWEBP); err != nil || f != imgGIF {
		t.Fatalf("gif = %q, %v", f, err)
	}
	for name, data := range map[string][]byte{
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":      []byte("<html><script>alert(1)</script></html>"),
		"truncated": pngData[:20],
		"empty":     nil,
		"polyglot":  append([]byte("GIF89a"), pngData...),
	} {
		if _, _, err := sniffImage(data, imgPNG, imgJPEG, imgGIF, imgWEBP); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for _, dims := range [][2]uint32{{8193, 1}, {1, 8193}, {8192, 8192}, {100000, 100000}} {
		if _, _, err := sniffImage(pngWithHeaderSize(t, dims[0], dims[1]), imgPNG); err != errImageTooLarge {
			t.Errorf("%dx%d = %v, want errImageTooLarge", dims[0], dims[1], err)
		}
	}
	if _, _, err := sniffImage(pngWithHeaderSize(t, 8192, 16), imgPNG); err != nil {
		t.Errorf("8192x16 rejected: %v", err)
	}
	// Re-encoding drops anything appended after the image.
	withTrailer := append(append([]byte{}, pngData...), []byte("<script>alert(1)</script>")...)
	out, err := reencodePNG(withTrailer)
	if err != nil || bytes.Contains(out, []byte("<script>")) {
		t.Fatalf("reencodePNG = %v", err)
	}
	if _, err := reencodePNG(jpg.Bytes()); err != nil {
		t.Fatalf("reencodePNG jpeg = %v", err)
	}
	if _, err := readLimited(bytes.NewReader(make([]byte, 11)), 10); err != errUploadTooLarge {
		t.Fatalf("readLimited oversize = %v", err)
	}
	if data, err := readLimited(bytes.NewReader(make([]byte, 10)), 10); err != nil || len(data) != 10 {
		t.Fatalf("readLimited exact = %d, %v", len(data), err)
	}
	if _, err := readLimited(bytes.NewReader(nil), 10); err != errUploadEmpty {
		t.Fatalf("readLimited empty = %v", err)
	}
	old := config.Global
	config.Global = &config.Config{UploadMaxBytes: 1 << 20}
	t.Cleanup(func() { config.Global = old })
	if uploadLimit(8<<20) != 1<<20 || uploadLimit(512) != 512 {
		t.Fatal("uploadLimit does not apply UPLOAD_MAX_BYTES")
	}
}

func TestAssetUploadExtensionComesFromContent(t *testing.T) {
	pngData := testPNG(t, 2, 2)
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		assetType, clientExt string
		data                 []byte
		want                 string
		ok                   bool
	}{
		{models.AssetTypeImage, ".png", pngData, ".png", true},
		{models.AssetTypeImage, ".jpeg", jpg.Bytes(), ".jpg", true},
		{models.AssetTypeImage, ".webp", pngData, ".png", true},
		{models.AssetTypeImage, ".png", jpg.Bytes(), "", false},
		{models.AssetTypeImage, ".jpg", []byte("\xff\xd8 not really a jpeg"), "", false},
		{models.AssetTypeImage, ".png", pngWithHeaderSize(t, 20000, 20000), "", false},
		{models.AssetTypeMesh, ".obj", []byte("v 0 0 0\n"), ".obj", true},
		{models.AssetTypeMesh, ".glb", []byte("glTF\x02\x00\x00\x00"), ".glb", true},
		{models.AssetTypeMesh, ".glb", []byte("v 0 0 0\n"), "", false},
		{models.AssetTypeSound, ".mp3", []byte("OggS\x00\x02"), ".ogg", true},
		{models.AssetTypeSound, ".ogg", []byte("ID3\x04\x00"), ".mp3", true},
		{models.AssetTypeSound, ".wav", []byte("RIFF\x00\x00\x00\x00WAVEfmt "), ".wav", true},
		{models.AssetTypeSound, ".mp3", []byte("<html>"), "", false},
	}
	for _, tc := range cases {
		got, err := validateAssetUpload(tc.assetType, tc.clientExt, tc.data)
		if tc.ok != (err == nil) || got != tc.want {
			t.Errorf("validateAssetUpload(%s, %s) = %q, %v", tc.assetType, tc.clientExt, got, err)
		}
	}
}
