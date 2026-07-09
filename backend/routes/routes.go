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
	app.Get("/logout", handlers.Logout)
	app.Get("/altcha", handlers.AltchaGet)
	app.Get("/avatar/:id", handlers.AvatarGet)
	app.Get("/avatar/shop/:type/:id", handlers.ShopRenderGet)

	api := app.Group("/api/v1")
	api.Get("/altcha", handlers.AltchaGet)
	api.Get("/avatar/:id", handlers.AvatarGet)
	api.Get("/avatar/headshots/:id", handlers.AvatarHeadshotGet)
	api.Get("/avatar/headshot/:id", handlers.AvatarHeadshotGet)
	api.Get("/avatar/shop/:type/:id", handlers.ShopRenderGet)

	app.Get("/static*", static.New("./static", static.Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.Next()
		},
	}))

	app.Get("/static/renders/avatars/full/:id", handlers.AvatarGet)
	app.Get("/static/renders/avatars/headshots/:id", handlers.AvatarHeadshotGet)

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