package handlers

import (
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
)

var gameMediaPendingDir = "./uploads/game_media_pending"
var gameMediaMu sync.Mutex

func removePendingGameMediaKind(id int, kind string) {
	entries, _ := os.ReadDir(gameMediaPendingDir)
	prefix := fmt.Sprintf("%d_%s_", id, kind)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.RemoveAll(filepath.Join(gameMediaPendingDir, entry.Name()))
		}
	}
}
func removePendingGameMedia(id int) {
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	removePendingGameMediaKind(id, "icon")
	removePendingGameMediaKind(id, "thumbs")
}
func submitGameMedia(id int, kind string, files []*multipart.FileHeader) error {
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	if err := os.MkdirAll(gameMediaPendingDir, 0755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(gameMediaPendingDir, ".upload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for i, file := range files {
		label := "Thumbnail"
		if kind == "icon" {
			label = "Icon"
		}
		if err := saveGameUploadImage(file, label, filepath.Join(dir, fmt.Sprintf("%d.png", i+1))); err != nil {
			return err
		}
	}
	token := fmt.Sprintf("%d_%s_%s", id, kind, strings.TrimPrefix(filepath.Base(dir), ".upload-"))
	if err := os.Rename(dir, filepath.Join(gameMediaPendingDir, token)); err != nil {
		return err
	}
	entries, _ := os.ReadDir(gameMediaPendingDir)
	prefix := fmt.Sprintf("%d_%s_", id, kind)
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != token && strings.HasPrefix(entry.Name(), prefix) {
			_ = os.RemoveAll(filepath.Join(gameMediaPendingDir, entry.Name()))
		}
	}
	return nil
}
func parseGameMediaToken(token string) (int, string, bool) {
	parts := strings.Split(token, "_")
	if len(parts) != 3 || parts[2] == "" {
		return 0, "", false
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 || (parts[1] != "icon" && parts[1] != "thumbs") {
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
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	entries, err := os.ReadDir(gameMediaPendingDir)
	if err != nil && !os.IsNotExist(err) {
		recordInternalError(c, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	items := []fiber.Map{}
	for _, entry := range entries {
		id, kind, ok := parseGameMediaToken(entry.Name())
		if !ok || !entry.IsDir() {
			continue
		}
		game, err := service.Game.GetGameByID(id)
		if err != nil || game == nil {
			continue
		}
		count := 0
		for i := 1; i <= gameMaxThumbnails; i++ {
			if _, err := os.Stat(filepath.Join(gameMediaPendingDir, entry.Name(), fmt.Sprintf("%d.png", i))); err != nil {
				break
			}
			count++
		}
		if count > 0 {
			items = append(items, fiber.Map{"token": entry.Name(), "id": id, "name": game.Name, "kind": kind, "count": count})
		}
	}
	return c.JSON(fiber.Map{"items": items})
}
func AdminGameMediaImageGet(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	token := c.Params("token")
	_, _, ok := parseGameMediaToken(token)
	index, err := strconv.Atoi(c.Params("index"))
	if !ok || err != nil || index < 1 || index > gameMaxThumbnails {
		return c.SendStatus(404)
	}
	gameMediaMu.Lock()
	defer gameMediaMu.Unlock()
	data, err := os.ReadFile(filepath.Join(gameMediaPendingDir, token, fmt.Sprintf("%d.png", index)))
	if err != nil {
		return c.SendStatus(404)
	}
	c.Set("Cache-Control", "no-store")
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
	dir := filepath.Join(gameMediaPendingDir, token)
	if _, err := os.Stat(filepath.Join(dir, "1.png")); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Submission no longer exists"})
	}
	if action == "approve" {
		game, err := service.Game.GetGameByID(id)
		if err != nil || game == nil {
			return c.SendStatus(404)
		}
		count := 1
		if kind == "thumbs" {
			for count < gameMaxThumbnails {
				if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%d.png", count+1))); err != nil {
					break
				}
				count++
			}
		}
		staged := []string{}
		defer func() {
			for _, p := range staged {
				_ = os.Remove(p)
			}
		}()
		for i := 1; i <= count; i++ {
			dst := gameIconPath(id)
			if kind == "thumbs" {
				dst = gameThumbPath(id, i)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.png", i)))
			if err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			f, err := os.CreateTemp(filepath.Dir(dst), ".game-media-")
			if err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			staged = append(staged, f.Name())
			_, err = f.Write(data)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				recordInternalError(c, errors.Join(err, closeErr))
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		for i, src := range staged {
			dst := gameIconPath(id)
			if kind == "thumbs" {
				dst = gameThumbPath(id, i+1)
			}
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			if err := os.Rename(src, dst); err != nil {
				recordInternalError(c, err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		if kind == "thumbs" {
			for i := count + 1; i <= gameMaxThumbnails; i++ {
				_ = os.Remove(gameThumbPath(id, i))
			}
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		recordInternalError(c, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"success": true})
}
