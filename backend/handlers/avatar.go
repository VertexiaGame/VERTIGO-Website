package handlers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/service"
)

func AvatarGet(c fiber.Ctx) error {
	idParam := c.Params("id")
	idParam = strings.TrimSuffix(idParam, ".png")
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

	cachePathFull := filepath.Join("static", "renders", "avatars", "full", idParam+".png")
	cachePathHead := filepath.Join("static", "renders", "avatars", "headshots", idParam+".png")

	_, errFull := os.Stat(cachePathFull)
	_, errHead := os.Stat(cachePathHead)
	if errFull == nil && errHead == nil {
		imgBytes, err := os.ReadFile(cachePathFull)
		if err == nil {
			c.Set("Content-Type", "image/png")
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

	c.Set("Content-Type", "image/png")
	return c.Send(imgBytesFull)
}

func AvatarHeadshotGet(c fiber.Ctx) error {
	idParam := c.Params("id")
	idParam = strings.TrimSuffix(idParam, ".png")
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

	cachePathFull := filepath.Join("static", "renders", "avatars", "full", idParam+".png")
	cachePathHead := filepath.Join("static", "renders", "avatars", "headshots", idParam+".png")

	_, errFull := os.Stat(cachePathFull)
	_, errHead := os.Stat(cachePathHead)
	if errFull == nil && errHead == nil {
		imgBytes, err := os.ReadFile(cachePathHead)
		if err == nil {
			c.Set("Content-Type", "image/png")
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

	c.Set("Content-Type", "image/png")
	return c.Send(imgBytesHead)
}

func ShopRenderGet(c fiber.Ctx) error {
	itemType := c.Params("type")
	idParam := c.Params("id")
	itemID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid item ID")
	}

	imgBytes, err := renderer.RenderShopItem(itemType, itemID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Type", "image/png")
	return c.Send(imgBytes)
}

func AvatarDataGet(c fiber.Ctx) error {
	idParam := c.Params("id")
	idParam = strings.TrimSuffix(idParam, ".png")
	userID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor string
	var hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face int

	if database.DB != nil {
		query := "SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color, hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face FROM avatar WHERE id = ?"
		_ = database.DB.QueryRow(query, userID).Scan(
			&headColor, &larmColor, &rarmColor, &torsoColor, &llegColor, &rlegColor,
			&hat1, &hat2, &hat3, &hat4, &hat5, &tool, &shirt, &tshirt, &pants, &face,
		)
	}

	if headColor == "" {
		headColor = "f3b700"
	}
	if larmColor == "" {
		larmColor = "f3b700"
	}
	if rarmColor == "" {
		rarmColor = "f3b700"
	}
	if torsoColor == "" {
		torsoColor = "c60000"
	}
	if llegColor == "" {
		llegColor = "650013"
	}
	if rlegColor == "" {
		rlegColor = "650013"
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
	})
}