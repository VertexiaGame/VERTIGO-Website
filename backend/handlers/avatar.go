package handlers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/renderer"
)

func AvatarGet(c fiber.Ctx) error {
	idParam := c.Params("id")
	idParam = strings.TrimSuffix(idParam, ".png")
	userID, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid user ID")
	}

	var exists bool
	if database.DB != nil {
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
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

	var exists bool
	if database.DB != nil {
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
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