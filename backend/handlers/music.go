package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
)

func MusicSearchGet(c fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON(fiber.Map{
			"data": []any{},
		})
	}

	if service.Music == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Music service uninitialized",
		})
	}

	tracks, err := service.Music.SearchTracks(q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}