package handlers

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
	filtersvc "vertexia-frontend/backend/service/filter"
)

const filterTestMaxContent = 4000

func FilterTestAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	if service.Filter == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Filter API is not configured. Set TYPESAFE_API_KEY and restart the server.",
		})
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Content is required"})
	}
	if utf8.RuneCountInString(content) > filterTestMaxContent {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Content is too long"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := service.Filter.Check(ctx, content)
	if err != nil {
		if errors.Is(err, filtersvc.ErrEmptyContent) || errors.Is(err, filtersvc.ErrContentTooLong) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		log.Printf("Filter test request failed: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Filter API request failed"})
	}

	return c.JSON(result)
}
