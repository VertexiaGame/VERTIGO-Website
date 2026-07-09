package handlers

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/models"
)

func Home(c fiber.Ctx) error {
	count, _ := models.GetUserCount(database.DB)
	games, err := models.GetPopularGames(database.DB, 12)
	if err != nil {
		log.Printf("VERTEXIA DB Error: Failed to fetch popular games: %v", err)
	}
	return Render(c, "pages/home", fiber.Map{
		"Title":     "VERTEXIA",
		"UserCount": count,
		"Games":     games,
	}, "layouts/main")
}