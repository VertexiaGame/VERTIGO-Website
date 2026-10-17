package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/service"
)

func setAvatarNoCache(c fiber.Ctx) {
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
}

func refreshAvatarRendersAsync(userID int) {
	if service.Avatar != nil {
		service.Avatar.InvalidateRenderCache(userID)
	}
	go func() {
		fullPath := service.AvatarFullPath(userID)
		headPath := service.AvatarHeadPath(userID)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if imgBytes, err := renderer.RenderUser(database.DB, userID); err == nil && len(imgBytes) > 0 {
			_ = os.WriteFile(fullPath, imgBytes, 0644)
		}
		if imgBytes, err := renderer.RenderUserHeadshot(database.DB, userID); err == nil && len(imgBytes) > 0 {
			_ = os.WriteFile(headPath, imgBytes, 0644)
		}
	}()
}

func serveAvatar(c fiber.Ctx, isHeadshot bool) error {
	idParam := strings.TrimSuffix(c.Params("id"), ".png")
	userID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid user ID")
	}

	if service.User != nil {
		exists, err := service.User.UserExists(userID)
		if err != nil || !exists {
			return c.Status(fiber.StatusNotFound).SendString("User not found")
		}
	} else {
		return c.Status(fiber.StatusInternalServerError).SendString("Database offline")
	}

	cachePathFull := service.AvatarFullPath(userID)
	cachePathHead := service.AvatarHeadPath(userID)

	targetCache := cachePathFull
	if isHeadshot {
		targetCache = cachePathHead
	}

	if fi, err := os.Stat(targetCache); err == nil && fi.Size() > 0 {
		if imgBytes, err := os.ReadFile(targetCache); err == nil && len(imgBytes) > 0 {
			setAvatarNoCache(c)
			return c.Send(imgBytes)
		}
	}

	imgBytesFull, err := renderer.RenderUser(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	imgBytesHead, err := renderer.RenderUserHeadshot(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	_ = os.MkdirAll(filepath.Dir(cachePathFull), 0755)
	_ = os.MkdirAll(filepath.Dir(cachePathHead), 0755)
	_ = os.WriteFile(cachePathFull, imgBytesFull, 0644)
	_ = os.WriteFile(cachePathHead, imgBytesHead, 0644)

	setAvatarNoCache(c)
	if isHeadshot {
		return c.Send(imgBytesHead)
	}
	return c.Send(imgBytesFull)
}

func AvatarGet(c fiber.Ctx) error {
	return serveAvatar(c, false)
}

func AvatarHeadshotGet(c fiber.Ctx) error {
	return serveAvatar(c, true)
}

func ShopRenderGet(c fiber.Ctx) error {
	rawType := c.Params("type")
	normType, _ := models.NormalizeShopCategory(rawType)
	if normType == "all" {
		normType = strings.ToLower(filepath.Clean(rawType))
	}

	idParam := strings.TrimSuffix(c.Params("id"), ".png")
	itemID, err := strconv.Atoi(idParam)
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid item ID")
	}

	cachePath := filepath.Join("static", "renders", "shop", normType, idParam+".png")
	if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
		if imgBytes, err := os.ReadFile(cachePath); err == nil && len(imgBytes) > 0 {
			c.Set("Content-Type", "image/png")
			c.Set("Cache-Control", "public, max-age=86400")
			return c.Send(imgBytes)
		}
	}

	imgBytes, err := renderer.RenderShopItem(normType, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if len(imgBytes) > 0 {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
		tmpPath := cachePath + ".tmp"
		if err := os.WriteFile(tmpPath, imgBytes, 0644); err == nil {
			_ = os.Rename(tmpPath, cachePath)
		} else {
			_ = os.Remove(tmpPath)
		}
	}

	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "public, max-age=86400")
	return c.Send(imgBytes)
}

func AvatarOutfitGet(c fiber.Ctx) error {
	idParam := strings.TrimSuffix(c.Params("id"), ".png")
	outfitID, err := strconv.Atoi(idParam)
	if err != nil || outfitID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid outfit ID")
	}

	cachePath := filepath.Join("static", "renders", "outfits", idParam+".png")
	if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
		if imgBytes, err := os.ReadFile(cachePath); err == nil && len(imgBytes) > 0 {
			c.Set("Content-Type", "image/png")
			c.Set("Cache-Control", "public, max-age=86400")
			return c.Send(imgBytes)
		}
	}

	imgBytes, err := renderer.RenderOutfit(database.DB, outfitID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if len(imgBytes) > 0 {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
		tmpPath := cachePath + ".tmp"
		if err := os.WriteFile(tmpPath, imgBytes, 0644); err == nil {
			_ = os.Rename(tmpPath, cachePath)
		} else {
			_ = os.Remove(tmpPath)
		}
	}

	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "public, max-age=86400")
	return c.Send(imgBytes)
}

func AvatarDataGet(c fiber.Ctx) error {
	idParam := strings.TrimSuffix(c.Params("id"), ".png")
	userID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var avatarData *models.Avatar
	if service.Avatar != nil {
		avatarData, _ = service.Avatar.GetAvatar(userID)
	}

	headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor := models.DefaultHeadColor, models.DefaultArmColor, models.DefaultArmColor, models.DefaultTorsoColor, models.DefaultLegColor, models.DefaultLegColor
	face, shirt, pants, tshirt, tool, hat1, hat2, hat3, hat4, hat5 := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0

	if avatarData != nil {
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

	formatColor := func(hex string) string {
		if !strings.HasPrefix(hex, "#") {
			return "#" + hex
		}
		return hex
	}

	return c.JSON(fiber.Map{
		"head_color":  formatColor(headColor),
		"torso_color": formatColor(torsoColor),
		"larm_color":  formatColor(larmColor),
		"rarm_color":  formatColor(rarmColor),
		"lleg_color":  formatColor(llegColor),
		"rleg_color":  formatColor(rlegColor),
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
	})
}

func AvatarEditorPage(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/login")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.Redirect().To("/login")
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Redirect().To("/login")
	}

	var avatarData *models.Avatar
	if service.Avatar != nil {
		avatarData, _ = service.Avatar.GetAvatar(user.ID)
	}
	if avatarData == nil {
		avatarData = models.DefaultAvatar(user.ID)
	}

	var inventory []*models.InventoryItem
	var equipped []*models.InventoryItem
	var outfits []*models.Outfit
	if service.Avatar != nil {
		inventory, _ = service.Avatar.GetInventory(user.ID, "all", "")
		equipped, _ = service.Avatar.GetEquippedItems(user.ID)
		outfits, _ = service.Avatar.GetOutfits(user.ID)
	}

	return Render(c, "pages/avatar", fiber.Map{
		"Title":      "Avatar Editor - VERTEXIA",
		"User":       user,
		"Avatar":     avatarData,
		"Inventory":  inventory,
		"Equipped":   equipped,
		"Outfits":    outfits,
		"ActiveCat":  "all",
	}, "layouts/main")
}

func AvatarUpdateColorsPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	bodyPart := c.FormValue("part")
	colorHex := c.FormValue("color")

	if bodyPart == "" || colorHex == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing part or color"})
	}

	if service.Avatar != nil {
		if err := service.Avatar.UpdateBodyColor(user.ID, bodyPart, colorHex); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	refreshAvatarRendersAsync(user.ID)

	return c.JSON(fiber.Map{
		"success":    true,
		"user_id":    user.ID,
		"part":       bodyPart,
		"color":      colorHex,
		"render_url": fmt.Sprintf("/avatar/%d.png", user.ID),
	})
}

func AvatarWearItemPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	itemType := c.FormValue("type")
	itemID, err := strconv.Atoi(c.FormValue("id"))
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid item ID"})
	}

	if service.Avatar != nil {
		if err := service.Avatar.EquipItem(user.ID, itemType, itemID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	refreshAvatarRendersAsync(user.ID)

	return c.JSON(fiber.Map{
		"success":    true,
		"user_id":    user.ID,
		"item_type":  itemType,
		"item_id":    itemID,
		"render_url": fmt.Sprintf("/avatar/%d.png", user.ID),
	})
}

func AvatarUnwearItemPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	itemType := c.FormValue("type")
	itemID, err := strconv.Atoi(c.FormValue("id"))
	if err != nil || itemID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid item ID"})
	}

	if service.Avatar != nil {
		if err := service.Avatar.UnequipItem(user.ID, itemType, itemID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	refreshAvatarRendersAsync(user.ID)

	return c.JSON(fiber.Map{
		"success":    true,
		"user_id":    user.ID,
		"item_type":  itemType,
		"item_id":    itemID,
		"render_url": fmt.Sprintf("/avatar/%d.png", user.ID),
	})
}

func AvatarInventoryAPI(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	category := c.Query("category", "all")
	search := c.Query("q", "")

	if service.Avatar == nil {
		return c.JSON([]*models.InventoryItem{})
	}

	inventory, err := service.Avatar.GetInventory(user.ID, category, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(inventory)
}

func AvatarWearingAPI(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	if service.Avatar == nil {
		return c.JSON([]*models.InventoryItem{})
	}

	equipped, err := service.Avatar.GetEquippedItems(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(equipped)
}

func AvatarSaveOutfitPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	name := c.FormValue("name")
	if service.Avatar == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Avatar service unavailable"})
	}

	outfit, err := service.Avatar.CreateOutfit(user.ID, name)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"outfit":  outfit,
	})
}

func AvatarWearOutfitPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	outfitID, err := strconv.Atoi(c.FormValue("id"))
	if err != nil || outfitID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid outfit ID"})
	}

	if service.Avatar == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Avatar service unavailable"})
	}

	if err := service.Avatar.WearOutfit(user.ID, outfitID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	refreshAvatarRendersAsync(user.ID)

	return c.JSON(fiber.Map{
		"success":    true,
		"user_id":    user.ID,
		"render_url": fmt.Sprintf("/avatar/%d.png", user.ID),
	})
}

func AvatarDeleteOutfitPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	outfitID, err := strconv.Atoi(c.FormValue("id"))
	if err != nil || outfitID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid outfit ID"})
	}

	if service.Avatar == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Avatar service unavailable"})
	}

	if err := service.Avatar.DeleteOutfit(user.ID, outfitID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"outfit_id": outfitID,
	})
}

func AvatarOutfitsAPI(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	if service.Avatar == nil {
		return c.JSON([]*models.Outfit{})
	}

	outfits, err := service.Avatar.GetOutfits(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(outfits)
}

func AvatarReRenderPost(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	if service.Avatar != nil {
		refreshAvatarRendersAsync(user.ID)
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"user_id":    user.ID,
		"render_url": fmt.Sprintf("/avatar/%d.png", user.ID),
	})
}