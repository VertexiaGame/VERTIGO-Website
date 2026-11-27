// Package storage keeps every runtime-written persistent file in an object
// store: Cloudflare R2 (S3 API) in production and a directory on local disk in
// development. Keys are forward-slash paths that match the legacy on-disk
// layout relative to the working directory, for example
// "static/uploads/games/7.png" or "uploads/assets/12/abc.obj", so existing
// files and URLs keep working after a move between backends.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vertexia-frontend/backend/config"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ErrInvalidKey is returned when a key or legacy path is unsafe.
var ErrInvalidKey = errors.New("storage: invalid key")

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ModTime     time.Time
	ETag        string
}

// Store is an object store. Keys must come from Key or KeyFromPath.
type Store interface {
	// Put writes the object. size may be -1 when unknown.
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get opens the object. The caller must close the reader.
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// Delete removes the object. Deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// PublicURL returns a direct URL for the object when a public base URL is
	// configured (R2_PUBLIC_BASE_URL). Callers must only use it for keys that
	// are meant to be public.
	PublicURL(key string) (string, bool)
}

// Lister is implemented by stores that can enumerate keys under a prefix.
type Lister interface {
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}

var (
	mu           sync.RWMutex
	current      Store
	fallbackOnce sync.Once
	fallback     Store
)

// Init selects the backend from the configuration: R2 when R2_BUCKET is set,
// otherwise local disk rooted at STORAGE_DIR. Production without R2 is an
// error because the container filesystem is ephemeral.
func Init(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("storage: missing configuration")
	}
	var (
		s   Store
		err error
	)
	switch {
	case cfg.R2Enabled():
		s, err = NewR2(R2Options{
			AccountID:       cfg.R2AccountID,
			Bucket:          cfg.R2Bucket,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Endpoint:        cfg.R2Endpoint,
			PublicBaseURL:   cfg.R2PublicBaseURL,
		})
	case cfg.IsProduction():
		return errors.New("storage: R2_BUCKET is required in production: the container filesystem is ephemeral")
	default:
		root := cfg.StorageDir
		if strings.TrimSpace(root) == "" {
			root = "."
		}
		s, err = NewLocal(root)
	}
	if err != nil {
		return err
	}
	SetDefault(s)
	return nil
}

// Default returns the configured store. Before Init it returns a local store
// rooted at the working directory. It never returns nil.
func Default() Store {
	mu.RLock()
	s := current
	mu.RUnlock()
	if s != nil {
		return s
	}
	fallbackOnce.Do(func() {
		local, err := NewLocal(".")
		if err != nil {
			// "." always resolves; keep a usable value regardless.
			local = &LocalStore{root: "."}
		}
		fallback = local
	})
	return fallback
}

// SetDefault replaces the store returned by Default. Intended for Init and
// tests. Passing nil restores the pre-Init fallback.
func SetDefault(s Store) {
	mu.Lock()
	current = s
	mu.Unlock()
}

// Key joins elems into a clean forward-slash key. It rejects empty elements,
// elements starting with "/" (absolute paths), ".." segments, backslashes, NUL
// and other control characters. Percent escapes are not decoded. "." segments and duplicate
// slashes are removed.
func Key(elems ...string) (string, error) {
	if len(elems) == 0 {
		return "", ErrInvalidKey
	}
	parts := make([]string, 0, len(elems))
	for _, e := range elems {
		if e == "" {
			return "", ErrInvalidKey
		}
		for _, r := range e {
			if r == '\\' || r < 0x20 || r == 0x7f {
				return "", ErrInvalidKey
			}
		}
		if strings.HasPrefix(e, "/") {
			return "", ErrInvalidKey
		}
		for _, seg := range strings.Split(e, "/") {
			if seg == ".." {
				return "", ErrInvalidKey
			}
		}
		parts = append(parts, e)
	}
	joined := strings.Join(parts, "/")
	if strings.HasPrefix(joined, "/") {
		return "", ErrInvalidKey
	}
	clean := path.Clean(joined)
	if clean == "." || clean == "" || clean == "/" || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", ErrInvalidKey
	}
	return clean, nil
}

// KeyFromPath maps a legacy relative filesystem path such as
// "./uploads/assets/12.obj" or "static/renders/meshes/3.png" to its key
// ("uploads/assets/12.obj"). Absolute paths and traversal are rejected.
func KeyFromPath(p string) (string, error) {
	if p == "" || filepath.IsAbs(p) {
		return "", ErrInvalidKey
	}
	return Key(filepath.ToSlash(p))
}

// MustKey is Key for compile-time constant inputs; it panics on error.
func MustKey(elems ...string) string {
	k, err := Key(elems...)
	if err != nil {
		panic(fmt.Sprintf("storage: invalid constant key %q", elems))
	}
	return k
}

// HasPrefix reports whether key lies under the directory-style prefix.
func HasPrefix(key, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	return strings.HasPrefix(key, prefix)
}

// ContentTypeFor returns the content type implied by a key's extension.
func ContentTypeFor(key string) string {
	switch strings.ToLower(path.Ext(key)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".obj":
		return "text/plain; charset=utf-8"
	case ".glb":
		return "model/gltf-binary"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".vrtx":
		return "application/octet-stream"
	}
	if t := mime.TypeByExtension(path.Ext(key)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// PutBytes stores data under key in s.
func PutBytes(ctx context.Context, s Store, key string, data []byte, contentType string) error {
	if contentType == "" {
		contentType = ContentTypeFor(key)
	}
	return s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType)
}

// GetBytes reads the whole object, refusing objects larger than limit bytes
// when limit > 0.
func GetBytes(ctx context.Context, s Store, key string, limit int64) ([]byte, error) {
	rc, info, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if limit > 0 && info.Size > limit {
		return nil, fmt.Errorf("storage: object %s exceeds %d bytes", key, limit)
	}
	var r io.Reader = rc
	if limit > 0 {
		r = io.LimitReader(rc, limit+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(data)) > limit {
		return nil, fmt.Errorf("storage: object %s exceeds %d bytes", key, limit)
	}
	return data, nil
}

// Exists reports whether key exists. Errors other than ErrNotFound count as
// "does not exist" and are returned.
func Exists(ctx context.Context, s Store, key string) (bool, error) {
	_, err := s.Stat(ctx, key)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return false, err
}

// Copy duplicates src to dst within s.
func Copy(ctx context.Context, s Store, src, dst string) error {
	rc, info, err := s.Get(ctx, src)
	if err != nil {
		return err
	}
	defer rc.Close()
	ct := info.ContentType
	if ct == "" {
		ct = ContentTypeFor(dst)
	}
	return s.Put(ctx, dst, rc, info.Size, ct)
}

// Move copies src to dst and then deletes src.
func Move(ctx context.Context, s Store, src, dst string) error {
	if err := Copy(ctx, s, src, dst); err != nil {
		return err
	}
	return s.Delete(ctx, src)
}

// List enumerates keys under prefix when the store supports it.
func List(ctx context.Context, s Store, prefix string) ([]ObjectInfo, error) {
	l, ok := s.(Lister)
	if !ok {
		return nil, errors.New("storage: listing is not supported by this store")
	}
	return l.List(ctx, prefix)
}
