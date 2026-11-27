package handlers

import (
	"crypto/subtle"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/storage"
)

// gameserverKeyValid checks the shared gameserver API key. It is accepted
// from headers only: query strings end up in proxy and edge logs.
func gameserverKeyValid(c fiber.Ctx) bool {
	if config.Global == nil {
		return false
	}
	expected := strings.Trim(strings.TrimSpace(config.Global.GameserverAPIKey), "\"")
	if expected == "" {
		return false
	}
	provided := ""
	for _, h := range []string{"X-Gameserver-Key", "Gameserver-Key", "X-API-Key"} {
		if provided = c.Get(h); provided != "" {
			break
		}
	}
	if provided == "" && !hasAccessToken(c) {
		provided = bearerToken(c)
	}
	provided = strings.Trim(strings.TrimSpace(provided), "\"")
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func gameserverUkeyFromRequest(c fiber.Ctx) string {
	if v := legacyUnikey(c); v != "" {
		return v
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
	normType, ok := shopRenderType(c.Params("type"))
	if !ok {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid item type")
	}
	itemID, ok := parseStrictID(strings.TrimSpace(c.Params("id")), ".png", ".obj")
	if !ok {
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

	// assets/uploads/shop is where uploads are written; the other prefixes
	// are legacy locations kept for files that predate it.
	for _, prefix := range []string{keyShopUploads, "static/uploads/shop", "uploads/shop"} {
		key, err := storage.Key(prefix, normType, fileName)
		if err != nil {
			continue
		}
		c.Set("X-Avatar-Asset-Type", normType)
		c.Set("X-Avatar-Asset-ID", strconv.Itoa(itemID))
		err = sendStoredObject(c, key, contentType, "public, max-age=86400")
		if err == nil {
			return nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			recordInternalError(c, err)
			return c.Status(fiber.StatusServiceUnavailable).SendString("Avatar asset storage unavailable")
		}
	}
	c.Response().Header.Del("X-Avatar-Asset-Type")
	c.Response().Header.Del("X-Avatar-Asset-ID")
	return c.Status(fiber.StatusNotFound).SendString("Avatar asset not found")
}

func GameserverShopTextureGet(c fiber.Ctx) error {
	return serveGameserverShopFile(c, "png")
}

func GameserverShopMeshGet(c fiber.Ctx) error {
	return serveGameserverShopFile(c, "obj")
}
