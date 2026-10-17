package handlers

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

const (
	gameVrtxUploadDir = "./uploads/games/vrtx"
	gameVrtxMaxBytes  = 40 << 20
)

func gameVrtxDiskPath(gameID int) string {
	return filepath.Join(gameVrtxUploadDir, strconv.Itoa(gameID)+".vrtx")
}

func studioKeyFromRequest(c fiber.Ctx) string {
	for _, key := range []string{"ukey", "unikey", "user_key"} {
		if v := strings.TrimSpace(c.Query(key)); v != "" {
			return strings.Trim(strings.TrimSpace(v), "\"")
		}
	}
	for _, key := range []string{"X-User-Key", "X-Unikey", "X-Ukey"} {
		if v := strings.TrimSpace(c.Get(key)); v != "" {
			return strings.Trim(strings.TrimSpace(v), "\"")
		}
	}
	for _, key := range []string{"ukey", "unikey"} {
		if v := strings.TrimSpace(c.FormValue(key)); v != "" {
			return strings.Trim(strings.TrimSpace(v), "\"")
		}
	}
	return ""
}

func resolveStudioUser(c fiber.Ctx, jsonUkey string) (*models.User, error) {
	if service.User == nil {
		return nil, errors.New("service unavailable")
	}
	if username := GetActiveUser(c); username != "" {
		if u, _ := service.User.GetUserByUsername(username); u != nil {
			return u, nil
		}
	}
	ukey := studioKeyFromRequest(c)
	if ukey == "" {
		ukey = strings.Trim(strings.TrimSpace(jsonUkey), "\"")
	}
	if ukey == "" {
		return nil, errors.New("unauthorized")
	}
	u, err := service.User.GetUserByUnikey(ukey)
	if err != nil || u == nil {
		return nil, errors.New("unauthorized")
	}
	return u, nil
}

func StudioGameCreatePost(c fiber.Ctx) error {
	var body struct {
		Name        string `json:"name"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Ukey        string `json:"ukey"`
		Unikey      string `json:"unikey"`
	}
	_ = c.Bind().JSON(&body)
	user, err := resolveStudioUser(c, firstNonEmpty(body.Ukey, body.Unikey))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	name := firstNonEmpty(strings.TrimSpace(body.Name), strings.TrimSpace(body.Title), strings.TrimSpace(c.FormValue("name")), strings.TrimSpace(c.FormValue("title")))
	description := firstNonEmpty(strings.TrimSpace(body.Description), strings.TrimSpace(c.FormValue("description")))
	if gameCreationAgeBlocked(time.Since(user.CreationDate), user.HasAdminAccess(), false) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Your account must be at least 1 day old to create public games. You can still create a private game."})
	}
	if count, err := service.Game.CountUserGames(user.ID); err == nil && gameLimitReached(count, user.HasAdminAccess()) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "You have reached the maximum number of games you can publish."})
	}
	gameID, err := service.Game.CreateGameWithWarnings(user.ID, name, description, "", "", false, 100, "")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "id": gameID})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func readStudioVrtxBytes(c fiber.Ctx) ([]byte, error) {
	for _, field := range []string{"file", "vrtx"} {
		if header, err := c.FormFile(field); err == nil && header != nil && header.Size > 0 {
			if header.Size > gameVrtxMaxBytes {
				return nil, errors.New("VRTX file is too large")
			}
			src, err := header.Open()
			if err != nil {
				return nil, errors.New("Could not read upload")
			}
			data, err := io.ReadAll(io.LimitReader(src, gameVrtxMaxBytes+1))
			src.Close()
			if err != nil {
				return nil, errors.New("Could not read upload")
			}
			if int64(len(data)) > gameVrtxMaxBytes {
				return nil, errors.New("VRTX file is too large")
			}
			return data, nil
		}
	}
	data := c.Body()
	if len(data) == 0 {
		return nil, errors.New("Missing VRTX file")
	}
	if int64(len(data)) > gameVrtxMaxBytes {
		return nil, errors.New("VRTX file is too large")
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func validateVrtxBytes(data []byte) error {
	if len(data) < 8 {
		return errors.New("Invalid VRTX file")
	}
	if data[0] != 'V' || data[1] != 'R' || data[2] != 'T' || data[3] != 'X' {
		return errors.New("Invalid VRTX file")
	}
	return nil
}

func StudioGameVrtxUploadPost(c fiber.Ctx) error {
	user, err := resolveStudioUser(c, "")
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this game"})
	}
	data, err := readStudioVrtxBytes(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := validateVrtxBytes(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := os.MkdirAll(gameVrtxUploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save upload"})
	}
	dst := gameVrtxDiskPath(gameID)
	tmp, err := os.CreateTemp(gameVrtxUploadDir, "vrtxupload*.tmp")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save upload"})
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save upload"})
	}
	tmp.Close()
	os.Remove(dst)
	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save upload"})
	}
	return c.JSON(fiber.Map{"success": true, "id": gameID})
}

func GameserverGameVrtxGet(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	diskPath := gameVrtxDiskPath(gameID)
	fi, err := os.Stat(diskPath)
	if err != nil || fi.IsDir() || fi.Size() <= 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game file not found"})
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", "attachment; filename=\"game-"+strconv.Itoa(gameID)+".vrtx\"")
	return c.SendFile(diskPath)
}

func StudioGamesListGet(c fiber.Ctx) error {
	user, err := resolveStudioUser(c, "")
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	games, err := service.Game.GetUserGames(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not load games"})
	}
	if games == nil {
		games = []*models.Game{}
	}
	out := make([]fiber.Map, 0, len(games))
	for _, game := range games {
		if game == nil {
			continue
		}
		out = append(out, fiber.Map{"id": game.ID, "name": game.Name, "description": game.DescriptionText})
	}
	return c.JSON(fiber.Map{"games": out})
}

func StudioGameGet(c fiber.Ctx) error {
	user, err := resolveStudioUser(c, "")
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this game"})
	}
	return c.JSON(fiber.Map{"id": game.ID, "name": game.Name, "description": game.DescriptionText})
}

func StudioGameVrtxGet(c fiber.Ctx) error {
	user, err := resolveStudioUser(c, "")
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: please log in"})
	}
	gameID, err := strconv.Atoi(c.Params("id"))
	if err != nil || gameID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid game ID"})
	}
	if service.Game == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Game service unavailable"})
	}
	game, err := service.Game.GetGameByID(gameID)
	if err != nil || game == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game not found"})
	}
	ownerID := 0
	if game.CreatorID.Valid {
		ownerID = int(game.CreatorID.Int64)
	}
	if ownerID != user.ID && !user.HasAdminAccess() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not own this game"})
	}
	diskPath := gameVrtxDiskPath(gameID)
	fi, err := os.Stat(diskPath)
	if err != nil || fi.IsDir() || fi.Size() <= 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Game file not found"})
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", "attachment; filename=\"game-"+strconv.Itoa(gameID)+".vrtx\"")
	return c.SendFile(diskPath)
}
