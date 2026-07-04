package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error! ########## %v", err)
	}

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("DB error! ########## %v", err)
	}

	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Get("/", func(c fiber.Ctx) error {
		return c.Render("pages/home", fiber.Map{
			"Title": "Vertexia",
		}, "layouts/main")
	})

    app.Get("/login", func(c fiber.Ctx) error {
        return c.Render("pages/login", fiber.Map{
            "Title": "Log In - Vertexia",
        }, "layouts/main")
    })

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
	log.Fatal(app.Listen(":3000"))
}