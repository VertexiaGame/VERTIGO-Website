package handlers

import (
	"crypto/subtle"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

func gameserverKeyValid(c fiber.Ctx) bool {
	if config.Global == nil {
		return false
	}
	expected := strings.Trim(strings.TrimSpace(config.Global.GameserverAPIKey), "\"")
	if expected == "" {
		return false
	}
	provided := c.Get("X-Gameserver-Key")
	if provided == "" {
		provided = c.Get("Gameserver-Key")
	}
	if provided == "" {
		provided = c.Get("X-API-Key")
	}
	if provided == "" {
		provided = c.Query("gameserver_key")
	}
	if provided == "" {
		provided = c.Query("api_key")
	}
	if provided == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			provided = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	provided = strings.Trim(strings.TrimSpace(provided), "\"")
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func gameserverUkeyFromRequest(c fiber.Ctx) string {
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
	if idParam := strings.TrimSpace(c.Params("ukey")); idParam != "" {
		return strings.Trim(idParam, "\"")
	}
	return ""
}

func GameserverAvatarBundleGet(c fiber.Ctx) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized gameserver request"})
	}
	ukey := gameserverUkeyFromRequest(c)
	if ukey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing ukey parameter"})
	}
	if service.User == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "User service unavailable"})
	}
	user, err := service.User.GetUserByUnikey(ukey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor := models.DefaultHeadColor, models.DefaultArmColor, models.DefaultArmColor, models.DefaultTorsoColor, models.DefaultLegColor, models.DefaultLegColor
	face, shirt, pants, tshirt, tool, hat1, hat2, hat3, hat4, hat5 := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	if service.Avatar != nil {
		if avatarData, _ := service.Avatar.GetAvatar(user.ID); avatarData != nil {
			headColor = avatarData.HeadColor
			larmColor = avatarData.LArmColor
			rarmColor = avatarData.RArmColor
			torsoColor = avatarData.TorsoColor
			llegColor = avatarData.LLegColor
			rlegColor = avatarData.RLegColor
			face = avatarData.Face
			shirt = avatarData.Shirt
			pants = avatarData.Pants
			tshirt = avatarData.TShirt
			tool = avatarData.Tool
			hat1 = avatarData.Hat1
			hat2 = avatarData.Hat2
			hat3 = avatarData.Hat3
			hat4 = avatarData.Hat4
			hat5 = avatarData.Hat5
		}
	}

	formatColor := func(hex string) string {
		hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
		if hex == "" {
			return models.DefaultHeadColor
		}
		return hex
	}

	return c.JSON(fiber.Map{
		"uid":      user.ID,
		"username": user.Username,
		"avatar": fiber.Map{
			"id":          user.ID,
			"head_color":  formatColor(headColor),
			"torso_color": formatColor(torsoColor),
			"larm_color":  formatColor(larmColor),
			"rarm_color":  formatColor(rarmColor),
			"lleg_color":  formatColor(llegColor),
			"rleg_color":  formatColor(rlegColor),
			"face":        face,
			"face_id":     face,
			"shirt":       shirt,
			"pants":       pants,
			"tshirt":      tshirt,
			"tool":        tool,
			"hat1":        hat1,
			"hat2":        hat2,
			"hat3":        hat3,
			"hat4":        hat4,
			"hat5":        hat5,
		},
	})
}

func serveGameserverShopFile(c fiber.Ctx, ext string) error {
	if !gameserverKeyValid(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized gameserver request")
	}
	rawType := c.Params("type")
	normType, _ := models.NormalizeShopCategory(rawType)
	if normType == "all" {
		normType = strings.ToLower(filepath.Base(strings.TrimSpace(rawType)))
	}
	idParam := strings.TrimSpace(c.Params("id"))
	if dotIdx := strings.Index(idParam, "."); dotIdx != -1 {
		idParam = idParam[:dotIdx]
	}
	itemID, err := strconv.Atoi(idParam)
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid item ID")
	}

	var fileName string
	var contentType string
	switch ext {
	case "png":
		fileName = strconv.Itoa(itemID) + ".png"
		contentType = "image/png"
	case "obj":
		fileName = strconv.Itoa(itemID) + ".obj"
		contentType = "text/plain"
	default:
		return c.Status(fiber.StatusBadRequest).SendString("Invalid asset extension")
	}

	candidates := []string{
		filepath.Join("assets", "uploads", "shop", normType, fileName),
		filepath.Join("static", "uploads", "shop", normType, fileName),
		filepath.Join("uploads", "shop", normType, fileName),
	}
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if fi, err := os.Stat(clean); err == nil && !fi.IsDir() && fi.Size() > 0 {
			c.Set("Content-Type", contentType)
			c.Set("Cache-Control", "public, max-age=86400")
			c.Set("X-Avatar-Asset-Type", normType)
			c.Set("X-Avatar-Asset-ID", strconv.Itoa(itemID))
			return c.SendFile(clean)
		}
	}
	return c.Status(fiber.StatusNotFound).SendString("Avatar asset not found")
}

func GameserverShopTextureGet(c fiber.Ctx) error {
	return serveGameserverShopFile(c, "png")
}

func GameserverShopMeshGet(c fiber.Ctx) error {
	return serveGameserverShopFile(c, "obj")
}
