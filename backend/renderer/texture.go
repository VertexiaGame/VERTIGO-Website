package renderer

import (
	"container/list"
	"fmt"
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

func loadTextureCached(path string) (fauxgl.Texture, error) {
	if key := fileStatKey(path); key != "" {
		if tex, ok := texLRU.get(key); ok {
			return tex, nil
		}
		tex, err := fauxgl.LoadTexture(path)
		if err != nil {
			return nil, err
		}
		texLRU.add(key, tex)
		return tex, nil
	}
	return fauxgl.LoadTexture(path)
}

func loadMeshCached(path string) (*fauxgl.Mesh, error) {
	if key := fileStatKey(path); key != "" {
		if mesh, ok := objLRU.get(key); ok {
			return mesh, nil
		}
		mesh, err := fauxgl.LoadOBJ(path)
		if err != nil || mesh == nil || len(mesh.Triangles) == 0 {
			if err == nil {
				err = fmt.Errorf("empty mesh: %s", path)
			}
			return nil, err
		}
		objLRU.add(key, mesh)
		return mesh, nil
	}
	mesh, err := fauxgl.LoadOBJ(path)
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

func fetchTexture(itemType string, id int, fallback string) (fauxgl.Texture, error) {
	if id <= 0 {
		if fallback != "" {
			return loadTextureCached(fallback)
		}
		return nil, fmt.Errorf("invalid id")
	}
	cleanType := strings.ToLower(filepath.Base(strings.TrimSpace(itemType)))

	localCandidates := expandCandidates([]string{
		filepath.Join("assets", "uploads", "shop", cleanType, fmt.Sprintf("%d.png", id)),
		filepath.Join("assets", "shop", cleanType, fmt.Sprintf("%d.png", id)),
		filepath.Join("static", "uploads", "shop", cleanType, fmt.Sprintf("%d.png", id)),
		filepath.Join("uploads", "shop", cleanType, fmt.Sprintf("%d.png", id)),
	})

	for _, localPath := range localCandidates {
		if _, err := os.Stat(localPath); err == nil {
			if tex, err := loadTextureCached(localPath); err == nil {
				return tex, nil
			}
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
	cleanType := strings.ToLower(filepath.Base(strings.TrimSpace(itemType)))

	localCandidates := expandCandidates([]string{
		filepath.Join("assets", "uploads", "shop", cleanType, fmt.Sprintf("%d.obj", id)),
		filepath.Join("assets", "shop", cleanType, fmt.Sprintf("%d.obj", id)),
		filepath.Join("static", "uploads", "shop", cleanType, fmt.Sprintf("%d.obj", id)),
		filepath.Join("uploads", "shop", cleanType, fmt.Sprintf("%d.obj", id)),
	})

	for _, localPath := range localCandidates {
		if _, err := os.Stat(localPath); err == nil {
			if mesh, err := loadMeshCached(localPath); err == nil && mesh != nil {
				return mesh, nil
			}
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