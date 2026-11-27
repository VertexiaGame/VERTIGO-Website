package handlers

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/storage"
)

// Storage key prefixes. Public prefixes are served directly from the object
// store by StoredFileGet (or redirected to R2_PUBLIC_BASE_URL). Everything
// else is private and only reachable through handlers that authorize the
// request first.
const (
	keyGameMedia      = "static/uploads/games"
	keyBanners        = "static/uploads/banners"
	keyClanMedia      = "static/uploads/clans"
	keyShopUploads    = "assets/uploads/shop"
	keyOutfitRenders  = "static/renders/outfits"
	keyShopRenders    = "static/renders/shop"
	keyMeshRenders    = "static/renders/meshes"      // private: renders of possibly unapproved assets
	keyAssetUploads   = "uploads/assets"             // private: moderated user assets
	keyGameVrtx       = "uploads/games/vrtx"         // private: studio and gameserver only
	keyGameMediaQueue = "uploads/game_media_pending" // private: moderation queue
	keyAvatarRenders  = "uploads/av"                 // served by AvatarGet
	storeOpTimeout    = 20 * time.Second
	storeStreamWindow = 2 * time.Minute
)

// PublicStoragePrefixes lists the key prefixes StoredFileGet may serve.
var PublicStoragePrefixes = []string{
	keyGameMedia,
	keyBanners,
	keyClanMedia,
	keyShopUploads,
	keyOutfitRenders,
	keyShopRenders,
}

// IsPublicStorageKey reports whether key may be served without authorization.
func IsPublicStorageKey(key string) bool {
	for _, p := range PublicStoragePrefixes {
		if storage.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func storeCtx(c fiber.Ctx) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if c != nil {
		parent = c.Context()
	}
	return context.WithTimeout(parent, storeOpTimeout)
}

func bgStoreCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), storeOpTimeout)
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnClose) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

// sendStoredObject streams key from the store with validators. contentType
// overrides the stored type when non-empty. It returns storage.ErrNotFound
// untouched so callers can fall through.
func sendStoredObject(c fiber.Ctx, key, contentType, cacheControl string) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeStreamWindow)
	rc, info, err := storage.Default().Get(ctx, key)
	if err != nil {
		cancel()
		return err
	}
	if contentType == "" {
		contentType = storage.ContentTypeFor(key)
	}
	c.Set(fiber.HeaderContentType, contentType)
	c.Set("X-Content-Type-Options", "nosniff")
	if cacheControl != "" {
		c.Set(fiber.HeaderCacheControl, cacheControl)
	}
	if info.ETag != "" {
		c.Set(fiber.HeaderETag, `"`+info.ETag+`"`)
	}
	if !info.ModTime.IsZero() {
		c.Set(fiber.HeaderLastModified, info.ModTime.UTC().Format(http.TimeFormat))
	}
	if c.Fresh() {
		rc.Close()
		cancel()
		return c.SendStatus(fiber.StatusNotModified)
	}
	if c.Method() == fiber.MethodHead {
		rc.Close()
		cancel()
		c.Set(fiber.HeaderContentLength, strconv.FormatInt(info.Size, 10))
		return c.SendStatus(fiber.StatusOK)
	}
	body := &cancelOnClose{ReadCloser: rc, cancel: cancel}
	if info.Size >= 0 && info.Size <= int64(^uint(0)>>1) {
		return c.SendStream(body, int(info.Size))
	}
	return c.SendStream(body)
}

// StoredFileGet serves public objects for request paths that mirror their key
// (for example /static/uploads/games/7_0.png). Unknown, private or missing
// keys fall through to the next handler so render-on-miss handlers still run.
func StoredFileGet(c fiber.Ctx) error {
	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
		return c.Next()
	}
	key, err := storage.Key(strings.TrimPrefix(c.Path(), "/"))
	if err != nil || !IsPublicStorageKey(key) {
		return c.Next()
	}
	store := storage.Default()
	if _, ok := store.PublicURL(key); ok {
		ctx, cancel := storeCtx(c)
		_, statErr := store.Stat(ctx, key)
		cancel()
		if errors.Is(statErr, storage.ErrNotFound) {
			return c.Next()
		}
		if statErr == nil {
			u, _ := store.PublicURL(key)
			c.Set(fiber.HeaderCacheControl, "public, max-age=300")
			return c.Redirect().Status(fiber.StatusFound).To(u)
		}
		log.Printf("storage: stat %s: %v", key, statErr)
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	err = sendStoredObject(c, key, "", "public, max-age=86400")
	if errors.Is(err, storage.ErrNotFound) {
		return c.Next()
	}
	if err != nil {
		log.Printf("storage: get %s: %v", key, err)
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return nil
}
