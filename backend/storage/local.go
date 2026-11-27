package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// LocalStore keeps objects as files under a root directory.
type LocalStore struct {
	root string
}

// NewLocal returns a store rooted at dir. The directory is created when
// missing.
func NewLocal(dir string) (*LocalStore, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve %q: %w", dir, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create %q: %w", abs, err)
	}
	return &LocalStore{root: filepath.Clean(abs)}, nil
}

// Root returns the absolute root directory.
func (s *LocalStore) Root() string { return s.root }

// LocalPath returns the filesystem path for key after validating that it stays
// inside the root, including through symlinks in existing parent directories.
func (s *LocalStore) LocalPath(key string) (string, error) {
	clean, err := Key(key)
	if err != nil || clean != key {
		return "", ErrInvalidKey
	}
	full := filepath.Join(s.root, filepath.FromSlash(clean))
	if !within(s.root, full) {
		return "", ErrInvalidKey
	}
	// Resolve symlinks on the deepest existing ancestor so a link inside the
	// root cannot point writes or reads outside of it.
	rootReal, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(full)
	for dir != s.root && within(s.root, dir) {
		if _, statErr := os.Lstat(dir); statErr == nil {
			break
		}
		dir = filepath.Dir(dir)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	} else if resolved != rootReal && !within(rootReal, resolved) {
		return "", ErrInvalidKey
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(full)
		if err != nil || !within(rootReal, resolved) {
			return "", ErrInvalidKey
		}
	}
	return full, nil
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (s *LocalStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := s.LocalPath(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// MkdirAll may have created directories; re-check containment.
	if _, err := s.LocalPath(key); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	src := r
	if size >= 0 {
		src = io.LimitReader(r, size+1)
	}
	n, err := io.Copy(tmp, src)
	if err != nil {
		return err
	}
	if size >= 0 && n != size {
		return fmt.Errorf("storage: wrote %d bytes, expected %d", n, size)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, full); err != nil {
		return err
	}
	ok = true
	return nil
}

func (s *LocalStore) info(key string, fi os.FileInfo) ObjectInfo {
	sum := sha256.Sum256([]byte(key + "|" + strconv.FormatInt(fi.Size(), 10) + "|" + strconv.FormatInt(fi.ModTime().UnixNano(), 10)))
	return ObjectInfo{
		Key:         key,
		Size:        fi.Size(),
		ContentType: ContentTypeFor(key),
		ModTime:     fi.ModTime(),
		ETag:        hex.EncodeToString(sum[:12]),
	}
}

func (s *LocalStore) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, ObjectInfo{}, err
	}
	full, err := s.LocalPath(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, ObjectInfo{}, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, ObjectInfo{}, ErrNotFound
	}
	return f, s.info(key, fi), nil
}

func (s *LocalStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}
	full, err := s.LocalPath(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, err
	}
	if !fi.Mode().IsRegular() {
		return ObjectInfo{}, ErrNotFound
	}
	return s.info(key, fi), nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := s.LocalPath(key)
	if err != nil {
		return err
	}
	fi, err := os.Lstat(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.IsDir() {
		return ErrInvalidKey
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// PublicURL is never available for local storage; files are served by the
// application.
func (s *LocalStore) PublicURL(string) (string, bool) { return "", false }

// List returns regular files under prefix, sorted by key.
func (s *LocalStore) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	cleanPrefix, err := Key(prefix)
	if err != nil {
		return nil, err
	}
	dir, err := s.LocalPath(cleanPrefix)
	if err != nil {
		return nil, err
	}
	var out []ObjectInfo
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, s.info(key, fi))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
