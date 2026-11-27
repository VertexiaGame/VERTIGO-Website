package renderer

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/fauxgl"
	"vertexia-frontend/backend/storage"
)

var (
	bsurl         = "https://vertexia.xyz"
	httpClient    = &http.Client{Timeout: 5 * time.Second}
	downloadMutex sync.Mutex
	fileLocks     = make(map[string]*sync.Mutex)
	texLRU        = newLRUCache[string, fauxgl.Texture](128)
	objLRU        = newLRUCache[string, *fauxgl.Mesh](128)
)

type lruPair[K comparable, V any] struct {
	key   K
	value V
}

type lruCache[K comparable, V any] struct {
	mu  sync.Mutex
	cap int
	ll  *list.List
	tab map[K]*list.Element
}

func newLRUCache[K comparable, V any](cap int) *lruCache[K, V] {
	return &lruCache[K, V]{cap: cap, ll: list.New(), tab: make(map[K]*list.Element)}
}

func (c *lruCache[K, V]) get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.tab[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(lruPair[K, V]).value, true
	}
	var zero V
	return zero, false
}

func (c *lruCache[K, V]) add(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.tab[key]; ok {
		c.ll.MoveToFront(el)
		el.Value = lruPair[K, V]{key, value}
		return
	}
	if c.ll.Len() >= c.cap {
		if back := c.ll.Back(); back != nil {
			c.ll.Remove(back)
			delete(c.tab, back.Value.(lruPair[K, V]).key)
		}
	}
	c.tab[key] = c.ll.PushFront(lruPair[K, V]{key, value})
}

func fileStatKey(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return path + "|" + strconv.FormatInt(fi.ModTime().UnixNano(), 10) + "|" + strconv.FormatInt(fi.Size(), 10)
}

// maxTextureDimension caps decoded texture sizes so a small compressed file
// cannot expand into gigabytes of pixels.
const maxTextureDimension = 8192

// loadTextureFile decodes an image after checking its header dimensions.
func loadTextureFile(path string) (fauxgl.Texture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxTextureDimension || cfg.Height > maxTextureDimension {
		return nil, fmt.Errorf("texture dimensions %dx%d out of range", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	im, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return fauxgl.NewImageTexture(im), nil
}

// loadOBJ parses an OBJ file; the parser panics on some malformed input.
func loadOBJ(path string) (mesh *fauxgl.Mesh, err error) {
	defer func() {
		if r := recover(); r != nil {
			mesh, err = nil, fmt.Errorf("invalid mesh %s", filepath.Base(path))
		}
	}()
	return fauxgl.LoadOBJ(path)
}

func loadTextureCached(path string) (fauxgl.Texture, error) {
	if key := fileStatKey(path); key != "" {
		if tex, ok := texLRU.get(key); ok {
			return tex, nil
		}
		tex, err := loadTextureFile(path)
		if err != nil {
			return nil, err
		}
		texLRU.add(key, tex)
		return tex, nil
	}
	return loadTextureFile(path)
}

func loadMeshCached(path string) (*fauxgl.Mesh, error) {
	if key := fileStatKey(path); key != "" {
		if mesh, ok := objLRU.get(key); ok {
			return mesh, nil
		}
		mesh, err := loadOBJ(path)
		if err != nil || mesh == nil || len(mesh.Triangles) == 0 {
			if err == nil {
				err = fmt.Errorf("empty mesh: %s", path)
			}
			return nil, err
		}
		objLRU.add(key, mesh)
		return mesh, nil
	}
	mesh, err := loadOBJ(path)
	if err != nil || mesh == nil || len(mesh.Triangles) == 0 {
		if err == nil {
			err = fmt.Errorf("empty mesh: %s", path)
		}
		return nil, err
	}
	return mesh, nil
}

type CompositeTexture struct {
	Layers []fauxgl.Texture
	Color  fauxgl.Color
}

func (t CompositeTexture) Sample(u, v float64) fauxgl.Color {
	return t.blendLayers(u, v, false)
}

func (t CompositeTexture) BilinearSample(u, v float64) fauxgl.Color {
	return t.blendLayers(u, v, true)
}

func (t CompositeTexture) blendLayers(u, v float64, bilinear bool) fauxgl.Color {
	final := t.Color
	final.A = 1.0
	for _, l := range t.Layers {
		if l == nil {
			continue
		}
		var c fauxgl.Color
		if bilinear {
			c = l.BilinearSample(u, v)
		} else {
			c = l.Sample(u, v)
		}
		invA := 1.0 - c.A
		final.R = c.R*c.A + final.R*invA
		final.G = c.G*c.A + final.G*invA
		final.B = c.B*c.A + final.B*invA
	}
	return final
}

func getFileLock(path string) *sync.Mutex {
	downloadMutex.Lock()
	defer downloadMutex.Unlock()
	if lock, exists := fileLocks[path]; exists {
		return lock
	}
	lock := &sync.Mutex{}
	fileLocks[path] = lock
	return lock
}

func shopBaseURL() string {
	for _, key := range []string{"VERTEXIA_BASE_URL", "SITE_URL", "BASE_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return bsurl
}

func assetURL(itemType string, id int, ext string) string {
	return fmt.Sprintf("%s/assets/uploads/shop/%s/%d.%s?v=%d", shopBaseURL(), itemType, id, ext, time.Now().UnixNano())
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

func expandCandidates(rel []string) []string {
	base := exeDir()
	if base == "" {
		return rel
	}
	out := make([]string, 0, len(rel)*2)
	out = append(out, rel...)
	for _, r := range rel {
		out = append(out, filepath.Join(base, r))
	}
	return out
}

func downloadFile(url string, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code error: %d", resp.StatusCode)
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0755)
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(resp.Body, 32<<20))
	return err
}

// shopItemTypes is the allowlist of shop item directories. Item types reach
// file paths, so anything else is rejected.
var shopItemTypes = map[string]bool{"hat": true, "faces": true, "shirts": true, "tshirts": true, "pants": true, "gear": true}

const (
	maxShopTextureBytes = 64 << 20
	maxShopMeshBytes    = 32 << 20
)

func cleanShopType(itemType string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(itemType))
	return t, shopItemTypes[t]
}

// shopStoreKeys lists the storage keys an uploaded shop file may live under:
// assets/uploads/shop is current, the others are legacy locations.
func shopStoreKeys(itemType string, id int, ext string) []string {
	name := strconv.Itoa(id) + ext
	var keys []string
	for _, prefix := range []string{"assets/uploads/shop", "static/uploads/shop", "uploads/shop"} {
		if k, err := storage.Key(prefix, itemType, name); err == nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// localShopFile resolves a shop file to a local path: bundled read-only files
// under assets/shop first, then the object store (downloaded and cached on
// local disk when the store is remote).
func localShopFile(itemType string, id int, ext string, limit int64) string {
	for _, p := range expandCandidates([]string{filepath.Join("assets", "shop", itemType, strconv.Itoa(id)+ext)}) {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := storage.Default()
	for _, key := range shopStoreKeys(itemType, id, ext) {
		p, err := storage.CachedLocalPath(ctx, store, key, limit)
		if err == nil {
			return p
		}
		if !errors.Is(err, storage.ErrNotFound) {
			log.Printf("renderer: fetch %s: %v", key, err)
		}
	}
	return ""
}

func fetchTexture(itemType string, id int, fallback string) (fauxgl.Texture, error) {
	if id <= 0 {
		if fallback != "" {
			return loadTextureCached(fallback)
		}
		return nil, fmt.Errorf("invalid id")
	}
	cleanType, ok := cleanShopType(itemType)
	if !ok {
		if fallback != "" {
			return loadTextureCached(fallback)
		}
		return nil, fmt.Errorf("invalid item type")
	}

	if localPath := localShopFile(cleanType, id, ".png", maxShopTextureBytes); localPath != "" {
		if tex, err := loadTextureCached(localPath); err == nil {
			return tex, nil
		}
	}

	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d.png", cleanType, id))

	lock := getFileLock(tmpPath)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		if err := downloadFile(assetURL(cleanType, id, "png"), tmpPath); err != nil {
			os.Remove(tmpPath)
			if fallback != "" {
				return loadTextureCached(fallback)
			}
			return nil, err
		}
	}

	tex, err := loadTextureCached(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		if fallback != "" {
			return loadTextureCached(fallback)
		}
	}
	return tex, err
}

func fetchMesh(itemType string, id int) (*fauxgl.Mesh, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid id")
	}
	cleanType, ok := cleanShopType(itemType)
	if !ok {
		return nil, fmt.Errorf("invalid item type")
	}

	if localPath := localShopFile(cleanType, id, ".obj", maxShopMeshBytes); localPath != "" {
		if mesh, err := loadMeshCached(localPath); err == nil && mesh != nil {
			return mesh, nil
		}
	}

	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d.obj", cleanType, id))

	lock := getFileLock(tmpPath)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		if err := downloadFile(assetURL(cleanType, id, "obj"), tmpPath); err != nil {
			os.Remove(tmpPath)
			return nil, err
		}
	}

	mesh, err := loadMeshCached(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		log.Printf("failed loading mesh %s: %v", cleanType, err)
	}
	return mesh, err
}
