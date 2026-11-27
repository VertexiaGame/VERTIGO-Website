package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"vertexia-frontend/backend/config"
)

func TestKey(t *testing.T) {
	cases := []struct {
		in   []string
		want string
		ok   bool
	}{
		{[]string{"uploads", "assets", "12", "a.obj"}, "uploads/assets/12/a.obj", true},
		{[]string{"static/uploads/games", "7_0.png"}, "static/uploads/games/7_0.png", true},
		{[]string{"./uploads/assets/12.obj"}, "uploads/assets/12.obj", true},
		{[]string{"uploads//assets/./x.png"}, "uploads/assets/x.png", true},
		{[]string{"uploads", "%2e%2e", "x"}, "uploads/%2e%2e/x", true},
		{[]string{"uploads/..%2fx"}, "uploads/..%2fx", true},
		{[]string{"../x"}, "", false},
		{[]string{"a/../../b"}, "", false},
		{[]string{"uploads/../uploads/x"}, "", false},
		{[]string{"uploads", ".."}, "", false},
		{[]string{"uploads", "../etc"}, "", false},
		{[]string{"/etc/passwd"}, "", false},
		{[]string{"uploads/assets", "/etc/passwd"}, "", false},
		{[]string{"a\\b"}, "", false},
		{[]string{"..\\x"}, "", false},
		{[]string{"a\x00b"}, "", false},
		{[]string{"a\nb"}, "", false},
		{[]string{""}, "", false},
		{[]string{"."}, "", false},
		{[]string{"./"}, "", false},
		{[]string{"uploads", ""}, "", false},
		{nil, "", false},
	}
	for _, tc := range cases {
		got, err := Key(tc.in...)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("Key(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("Key(%q) = %q; want error", tc.in, got)
		}
	}
}

func TestKeyFromPath(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"./uploads/assets/12.obj", "uploads/assets/12.obj", true},
		{"static/renders/meshes/3.png", "static/renders/meshes/3.png", true},
		{"./static/uploads/games/7_0.png", "static/uploads/games/7_0.png", true},
		{filepath.Join("static", "uploads", "banners", "4.png"), "static/uploads/banners/4.png", true},
		{"uploads/games/vrtx/9.vrtx", "uploads/games/vrtx/9.vrtx", true},
		{"/etc/passwd", "", false},
		{"../x", "", false},
		{"./uploads/../../x", "", false},
		{"uploads/../uploads/x", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := KeyFromPath(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("KeyFromPath(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("KeyFromPath(%q) = %q; want error", tc.in, got)
		}
	}
}

// The object key used by R2 for a legacy path must equal the path relative to
// the local storage root, so files can be synced between backends verbatim.
func TestR2KeyMatchesLocalRelativePath(t *testing.T) {
	root := t.TempDir()
	local, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{
		"static/uploads/games/7_0.png",
		"./static/uploads/games/thumbs/7_2.png",
		"./uploads/assets/12/abc.obj",
		"static/renders/shop/hat/5.png",
		"assets/uploads/shop/gear/3.obj",
	} {
		key, err := KeyFromPath(legacy)
		if err != nil {
			t.Fatal(err)
		}
		objKey, err := objectKey(key)
		if err != nil {
			t.Fatalf("objectKey(%q): %v", key, err)
		}
		full, err := local.LocalPath(key)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, full)
		if filepath.ToSlash(rel) != objKey || filepath.ToSlash(filepath.Clean(legacy)) != objKey {
			t.Errorf("legacy %q: object key %q, local rel %q", legacy, objKey, rel)
		}
	}
	if _, err := objectKey("uploads/../x"); err == nil {
		t.Error("objectKey accepted traversal")
	}
	if _, err := objectKey("uploads//x"); err == nil {
		t.Error("objectKey accepted a non-canonical key")
	}
}

func TestPublicURL(t *testing.T) {
	s, err := NewR2(R2Options{AccountID: "acct", Bucket: "b", AccessKeyID: "id", SecretAccessKey: "secret", PublicBaseURL: "https://cdn.example.com/"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.PublicURL("static/uploads/games/7_0.png")
	if !ok || got != "https://cdn.example.com/static/uploads/games/7_0.png" {
		t.Fatalf("PublicURL = %q, %v", got, ok)
	}
	got, ok = s.PublicURL("static/uploads/clans/a b#c.png")
	if !ok || got != "https://cdn.example.com/static/uploads/clans/a%20b%23c.png" {
		t.Fatalf("PublicURL escaping = %q", got)
	}
	if _, ok := s.PublicURL("../x"); ok {
		t.Fatal("PublicURL accepted traversal")
	}
	noPublic, err := NewR2(R2Options{AccountID: "acct", Bucket: "b", AccessKeyID: "id", SecretAccessKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := noPublic.PublicURL("static/uploads/games/7.png"); ok {
		t.Fatal("PublicURL available without a base URL")
	}
	if _, err := NewR2(R2Options{AccountID: "acct", Bucket: "b", AccessKeyID: "id", SecretAccessKey: "s", PublicBaseURL: "http://cdn.example.com"}); err == nil {
		t.Fatal("accepted non-https public base")
	}
	local, _ := NewLocal(t.TempDir())
	if _, ok := local.PublicURL("static/uploads/games/7.png"); ok {
		t.Fatal("local store returned a public URL")
	}
}

func TestR2EndpointHost(t *testing.T) {
	for _, tc := range []struct {
		opts R2Options
		want string
		ok   bool
	}{
		{R2Options{AccountID: "abc123"}, "abc123.r2.cloudflarestorage.com", true},
		{R2Options{AccountID: "abc", Endpoint: "https://custom.example.com"}, "custom.example.com", true},
		{R2Options{Endpoint: "custom.example.com"}, "custom.example.com", true},
		{R2Options{Endpoint: "http://insecure.example.com"}, "", false},
		{R2Options{Endpoint: "https://x.example.com/path"}, "", false},
		{R2Options{AccountID: "evil.com/x"}, "", false},
		{R2Options{}, "", false},
	} {
		got, err := R2EndpointHost(tc.opts)
		if tc.ok != (err == nil) || got != tc.want {
			t.Errorf("R2EndpointHost(%+v) = %q, %v", tc.opts, got, err)
		}
	}
}

func TestLocalStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := "static/uploads/games/thumbs/7_2.png"
	if _, err := s.Stat(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat missing = %v", err)
	}
	if _, _, err := s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v", err)
	}
	if ok, err := Exists(ctx, s, key); ok || err != nil {
		t.Fatalf("Exists missing = %v, %v", ok, err)
	}
	data := []byte("\x89PNG fake")
	if err := PutBytes(ctx, s, key, data, "image/png"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Stat(ctx, key)
	if err != nil || info.Size != int64(len(data)) || info.ContentType != "image/png" || info.ETag == "" || info.Key != key {
		t.Fatalf("Stat = %+v, %v", info, err)
	}
	got, err := GetBytes(ctx, s, key, 0)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("GetBytes = %q, %v", got, err)
	}
	if _, err := GetBytes(ctx, s, key, 3); err == nil {
		t.Fatal("GetBytes ignored limit")
	}
	fi, err := os.Stat(filepath.Join(s.Root(), "static", "uploads", "games", "thumbs", "7_2.png"))
	if err != nil || fi.Mode().Perm() != 0o644 {
		t.Fatalf("file mode = %v, %v", fi, err)
	}
	if err := s.Put(ctx, key, strings.NewReader("short"), 99, ""); err == nil {
		t.Fatal("Put accepted a size mismatch")
	}
	if got, _ := GetBytes(ctx, s, key, 0); !bytes.Equal(got, data) {
		t.Fatal("failed Put replaced the object")
	}
	if err := Move(ctx, s, key, "static/uploads/games/thumbs/7.png"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := Exists(ctx, s, key); ok {
		t.Fatal("Move left the source")
	}
	list, err := List(ctx, s, "static/uploads/games/")
	if err != nil || len(list) != 1 || list[0].Key != "static/uploads/games/thumbs/7.png" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if err := s.Delete(ctx, "static/uploads/games/thumbs/7.png"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "static/uploads/games/thumbs/7.png"); err != nil {
		t.Fatalf("Delete missing = %v", err)
	}
	if list, err := List(ctx, s, "nothing/here"); err != nil || len(list) != 0 {
		t.Fatalf("List empty = %+v, %v", list, err)
	}
	entries, _ := os.ReadDir(filepath.Join(s.Root(), "static", "uploads", "games", "thumbs"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestLocalStoreContainment(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../outside/secret.txt", "/etc/passwd", "a/../../outside/secret.txt", "a\\..\\x", "uploads//x", "./uploads/x"} {
		if _, _, err := s.Get(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Get(%q) = %v, want ErrInvalidKey", key, err)
		}
		if err := PutBytes(ctx, s, key, []byte("x"), ""); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q) = %v, want ErrInvalidKey", key, err)
		}
		if err := s.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Delete(%q) = %v, want ErrInvalidKey", key, err)
		}
	}
	// A symlinked directory inside the root must not reach outside of it.
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := s.Get(ctx, "link/secret.txt"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Get through symlink = %v", err)
	}
	if err := PutBytes(ctx, s, "link/new.txt", []byte("x"), ""); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Put through symlink = %v", err)
	}
	if err := PutBytes(ctx, s, "link/deeper/new.txt", []byte("x"), ""); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Put below symlink = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Error("write escaped the root")
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "file-link")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get(ctx, "file-link"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Get file symlink = %v", err)
	}
}

func TestLocalCopy(t *testing.T) {
	ctx := context.Background()
	s, _ := NewLocal(t.TempDir())
	if err := PutBytes(ctx, s, "uploads/assets/1/a.obj", []byte("v 0 0 0"), ""); err != nil {
		t.Fatal(err)
	}
	p, cleanup, err := LocalCopy(ctx, s, "uploads/assets/1/a.obj", 0)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
	if b, err := os.ReadFile(p); err != nil || string(b) != "v 0 0 0" {
		t.Fatalf("local copy = %q, %v", b, err)
	}
	if _, _, err := LocalCopy(ctx, s, "uploads/assets/1/missing.obj", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if _, _, err := LocalCopy(ctx, s, "uploads/assets/1/a.obj", 2); err == nil {
		t.Fatal("limit ignored")
	}
	// A store without LocalPath downloads to a temporary file.
	wrapped := struct{ Store }{s}
	p, cleanup, err = LocalCopy(ctx, wrapped, "uploads/assets/1/a.obj", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, os.TempDir()) || filepath.Ext(p) != ".obj" {
		t.Fatalf("temp copy path %q", p)
	}
	cleanup()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("cleanup left the temp file")
	}
	cached, err := CachedLocalPath(ctx, wrapped, "uploads/assets/1/a.obj", 0)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CachedLocalPath(ctx, wrapped, "uploads/assets/1/a.obj", 0)
	if err != nil || again != cached {
		t.Fatalf("cache miss on second fetch: %q vs %q (%v)", cached, again, err)
	}
	os.Remove(cached)
}

func TestInitAndDefault(t *testing.T) {
	defer SetDefault(nil)
	SetDefault(nil)
	if Default() == nil {
		t.Fatal("Default returned nil before Init")
	}
	if _, ok := Default().(*LocalStore); !ok {
		t.Fatal("pre-Init default is not local")
	}
	dir := t.TempDir()
	if err := Init(&config.Config{StorageDir: dir}); err != nil {
		t.Fatal(err)
	}
	if ls, ok := Default().(*LocalStore); !ok || ls.Root() != dir {
		t.Fatalf("Init local = %#v", Default())
	}
	if err := Init(&config.Config{AppEnv: "production"}); err == nil {
		t.Fatal("production without R2 accepted")
	}
	if err := Init(&config.Config{R2Bucket: "b", R2AccountID: "acct", R2AccessKeyID: "id", R2SecretAccessKey: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := Default().(*R2Store); !ok {
		t.Fatal("Init did not select R2")
	}
}

func TestR2ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err      error
		notFound bool
	}{
		{minio.ErrorResponse{Code: "NoSuchKey", StatusCode: 404}, true},
		{minio.ErrorResponse{Code: "NotFound", StatusCode: 404}, true},
		{minio.ErrorResponse{StatusCode: 404}, true},
		{minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: 404}, false},
		{minio.ErrorResponse{Code: "AccessDenied", StatusCode: 403}, false},
		{errors.New("network down"), false},
	} {
		if got := errors.Is(mapR2Error(tc.err), ErrNotFound); got != tc.notFound {
			t.Errorf("mapR2Error(%v) not-found = %v, want %v", tc.err, got, tc.notFound)
		}
	}
	if mapR2Error(nil) != nil {
		t.Fatal("mapR2Error(nil) != nil")
	}
}
