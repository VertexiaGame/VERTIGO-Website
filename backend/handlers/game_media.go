package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"mime/multipart"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/storage"
)

// gameMediaPendingPrefix holds submissions awaiting moderation as
// <prefix>/<token>/<n>.png. It is private: only the admin handlers below read
// it.
var gameMediaPendingPrefix = keyGameMediaQueue
var gameMediaMu sync.Mutex

// pendingGameMediaTokens lists submission tokens and how many consecutive
// images (1.png, 2.png, ...) each holds.
func pendingGameMediaTokens() (map[string]int, error) {
	ctx, cancel := bgStoreCtx()
	defer cancel()
	objects, err := storage.List(ctx, storage.Default(), gameMediaPendingPrefix)
	if err != nil {
		return nil, err
	}
	files := map[string]map[int]bool{}
	for _, obj := range objects {
		rest, ok := strings.CutPrefix(obj.Key, gameMediaPendingPrefix+"/")
		if !ok {
			continue
		}
		token, name, ok := strings.Cut(rest, "/")
		if !ok || strings.Contains(name, "/") {
			continue
		}
		if _, _, valid := parseGameMediaToken(token); !valid {
			continue
		}
		index, ok := parseStrictID(name, ".png")
		if !ok || !strings.HasSuffix(name, ".png") || index > gameMaxThumbnails {
			continue
		}
		if files[token] == nil {
			files[token] = map[int]bool{}
		}
		files[token][index] = true
	}
	out := make(map[string]int, len(files))
	for token, idx := range files {
		n := 0
		for idx[n+1] {
			n++
		}
		out[token] = n
	}
	return out, nil
}

func gameMediaPendingKey(token string, index int) (string, error) {
	if _, _, ok := parseGameMediaToken(token); !ok || index < 1 || index > gameMaxThumbnails {
		return "", storage.ErrInvalidKey
	}
	return storage.Key(gameMediaPendingPrefix, token, strconv.Itoa(index)+".png")
}

func deletePendingGameMediaToken(token string) error {
	ctx, cancel := bgStoreCtx()
	defer cancel()
	var errs []error
	for i := 1; i <= gameMaxThumbnails; i++ {
		key, err := gameMediaPendingKey(token, i)
		if err != nil {
			return err
		}
		if err := storage.Default().Delete(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func removePendingGameMediaKind(id int, kind string) {
	tokens, _ := pendingGameMediaTokens()
	prefix := fmt.Sprintf("%d_%s_", id, kind)
	for token := range tokens {
		if strings.HasPrefix(token, prefix) {
			_ = deletePendingGameMediaToken(token)
		}
	}
}

func removePendingGameMedia(id int) {
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	removePendingGameMediaKind(id, "icon")
	removePendingGameMediaKind(id, "thumbs")
}

// newGameMediaToken returns "<id>_<kind>_<digits>" with an unguessable
// numeric suffix.
func newGameMediaToken(id int, kind string) (string, error) {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d_%s_%018d", id, kind, n.Uint64()), nil
}

func submitGameMedia(id int, kind string, files []*multipart.FileHeader) error {
	if id <= 0 || (kind != "icon" && kind != "thumbs") || len(files) == 0 || len(files) > gameMaxThumbnails {
		return errors.New("invalid game media submission")
	}
	label := "Thumbnail"
	if kind == "icon" {
		label = "Icon"
	}
	// Validate and re-encode everything before anything becomes visible in
	// the moderation queue.
	encoded := make([][]byte, 0, len(files))
	for _, file := range files {
		data, err := prepareUploadPNG(file, label)
		if err != nil {
			return err
		}
		encoded = append(encoded, data)
	}

	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	token, err := newGameMediaToken(id, kind)
	if err != nil {
		return err
	}
	ctx, cancel := bgStoreCtx()
	defer cancel()
	for i, data := range encoded {
		key, err := gameMediaPendingKey(token, i+1)
		if err == nil {
			err = storage.PutBytes(ctx, storage.Default(), key, data, "image/png")
		}
		if err != nil {
			_ = deletePendingGameMediaToken(token)
			return errors.New("Could not save upload")
		}
	}
	tokens, _ := pendingGameMediaTokens()
	prefix := fmt.Sprintf("%d_%s_", id, kind)
	for other := range tokens {
		if other != token && strings.HasPrefix(other, prefix) {
			_ = deletePendingGameMediaToken(other)
		}
	}
	return nil
}

func parseGameMediaToken(token string) (int, string, bool) {
	parts := strings.Split(token, "_")
	if len(parts) != 3 || parts[2] == "" || len(parts[2]) > 40 {
		return 0, "", false
	}
	id, ok := parseStrictID(parts[0])
	if !ok || (parts[1] != "icon" && parts[1] != "thumbs") {
		return 0, "", false
	}
	for _, c := range parts[2] {
		if c < '0' || c > '9' {
			return 0, "", false
		}
	}
	return id, parts[1], true
}

func AdminGameMediaQueueAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	tokens, err := pendingGameMediaTokens()
	if err != nil {
		recordInternalError(c, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	names := make([]string, 0, len(tokens))
	for token := range tokens {
		names = append(names, token)
	}
	sort.Strings(names)
	items := []fiber.Map{}
	for _, token := range names {
		count := tokens[token]
		id, kind, _ := parseGameMediaToken(token)
		game, err := service.Game.GetGameByID(id)
		if err != nil || game == nil {
			continue
		}
		if count > 0 {
			items = append(items, fiber.Map{"token": token, "id": id, "name": game.Name, "kind": kind, "count": count})
		}
	}
	return c.JSON(fiber.Map{"items": items})
}

func AdminGameMediaImageGet(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	token := c.Params("token")
	index, ok := parseStrictID(c.Params("index"))
	key, err := gameMediaPendingKey(token, index)
	if !ok || err != nil {
		return c.SendStatus(404)
	}
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	ctx, cancel := storeCtx(c)
	defer cancel()
	data, err := storage.GetBytes(ctx, storage.Default(), key, gameThumbnailStoredMaxBytes)
	if err != nil {
		return c.SendStatus(404)
	}
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Type("png")
	return c.Send(data)
}

func AdminGameMediaReviewPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	if !csrfValid(c) {
		return c.SendStatus(403)
	}
	token, action := c.Params("token"), c.Params("action")
	id, kind, ok := parseGameMediaToken(token)
	if !ok || (action != "approve" && action != "reject") {
		return c.SendStatus(400)
	}
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	tokens, err := pendingGameMediaTokens()
	if err != nil {
		recordInternalError(c, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	count := tokens[token]
	if count < 1 {
		return c.Status(404).JSON(fiber.Map{"error": "Submission no longer exists"})
	}
	if action == "approve" {
		game, err := service.Game.GetGameByID(id)
		if err != nil || game == nil {
			return c.SendStatus(404)
		}
		if kind != "thumbs" {
			count = 1
		}
		ctx, cancel := bgStoreCtx()
		defer cancel()
		store := storage.Default()
		staged := make([][]byte, 0, count)
		for i := 1; i <= count; i++ {
			key, err := gameMediaPendingKey(token, i)
			if err != nil {
				return c.SendStatus(400)
			}
			data, err := storage.GetBytes(ctx, store, key, gameThumbnailStoredMaxBytes)
			if err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			staged = append(staged, data)
		}
		for i, data := range staged {
			dst := gameIconPath(id)
			if kind == "thumbs" {
				dst = gameThumbPath(id, i+1)
			}
			if err := storage.PutBytes(ctx, store, dst, data, "image/png"); err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		if kind == "thumbs" {
			for i := count + 1; i <= gameMaxThumbnails; i++ {
				_ = store.Delete(ctx, gameThumbPath(id, i))
			}
		}
	}
	if err := deletePendingGameMediaToken(token); err != nil {
		recordInternalError(c, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"success": true})
}
