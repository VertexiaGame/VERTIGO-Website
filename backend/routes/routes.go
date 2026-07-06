package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"vertexia-frontend/backend/handlers"
)

func Setup(app *fiber.App) {
	app.Get("/", handlers.Home)
	app.Get("/login", handlers.LoginGet)
	app.Post("/login", handlers.LoginPost)
	app.Get("/register", handlers.RegisterGet)
	app.Post("/register", handlers.RegisterPost)
	app.Get("/altcha", handlers.AltchaGet)
	app.Get("/logout", handlers.Logout)

	app.Get("/static*", static.New("./static", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Get("/static*", static.New("./public", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Get("/*", static.New("./public", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))
}