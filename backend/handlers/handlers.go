package handlers

import (
	"github.com/gofiber/fiber/v3"
)

func Home(c fiber.Ctx) error {
	return Render(c, "pages/home", fiber.Map{
		"Title": "VERTEXIA",
	}, "layouts/main")
}