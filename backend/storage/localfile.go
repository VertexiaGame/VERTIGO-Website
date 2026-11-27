package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// localPather is implemented by stores whose objects already live on disk.
type localPather interface {
	LocalPath(key string) (string, error)
}

func safeExt(key string) string {
	ext := strings.ToLower(path.Ext(key))
	if len(ext) > 8 {
		return ""
	}
	for _, r := range ext[min(1, len(ext)):] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return ext
}

// LocalCopy makes the object available as a local file for code that needs a
// path (renderers, mesh parsers). For a local store it returns the file in
// place; otherwise it downloads to os.TempDir. cleanup must always be called.
// limit caps the download size when > 0.
func LocalCopy(ctx context.Context, s Store, key string, limit int64) (string, func(), error) {
	noop := func() {}
	if lp, ok := s.(localPather); ok {
		p, err := lp.LocalPath(key)
		if err != nil {
			return "", noop, err
		}
		fi, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", noop, ErrNotFound
			}
			return "", noop, err
		}
		if !fi.Mode().IsRegular() {
			return "", noop, ErrNotFound
		}
		if limit > 0 && fi.Size() > limit {
			return "", noop, errors.New("storage: object too large")
		}
		return p, noop, nil
	}
	rc, info, err := s.Get(ctx, key)
	if err != nil {
		return "", noop, err
	}
	defer rc.Close()
	if limit > 0 && info.Size > limit {
		return "", noop, errors.New("storage: object too large")
	}
	tmp, err := os.CreateTemp("", "vertexia-obj-*"+safeExt(key))
	if err != nil {
		return "", noop, err
	}
	name := tmp.Name()
	cleanup := func() { os.Remove(name) }
	var r io.Reader = rc
	if limit > 0 {
		r = io.LimitReader(rc, limit+1)
	}
	n, err := io.Copy(tmp, r)
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil && limit > 0 && n > limit {
		err = errors.New("storage: object too large")
	}
	if err != nil {
		cleanup()
		return "", noop, err
	}
	return name, cleanup, nil
}

var cacheMu sync.Mutex

// CachedLocalPath is LocalCopy for read-mostly objects reused across requests
// (shop textures and meshes). Remote objects are downloaded once per ETag into
// a cache directory under os.TempDir. File names are hashes, never derived
// from user input.
func CachedLocalPath(ctx context.Context, s Store, key string, limit int64) (string, error) {
	if _, ok := s.(localPather); ok {
		// Local objects are used in place; cleanup is a no-op for them.
		p, cleanup, err := LocalCopy(ctx, s, key, limit)
		cleanup()
		return p, err
	}
	info, err := s.Stat(ctx, key)
	if err != nil {
		return "", err
	}
	if limit > 0 && info.Size > limit {
		return "", errors.New("storage: object too large")
	}
	sum := sha256.Sum256([]byte(key + "|" + info.ETag + "|" + info.ModTime.UTC().String()))
	dir := filepath.Join(os.TempDir(), "vertexia-store-cache")
	dst := filepath.Join(dir, hex.EncodeToString(sum[:16])+safeExt(key))
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if fi, err := os.Stat(dst); err == nil && fi.Mode().IsRegular() && fi.Size() == info.Size {
		return dst, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmpPath, cleanup, err := LocalCopy(ctx, s, key, limit)
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		cleanup()
		return "", err
	}
	return dst, nil
}
