package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v3"
)

func main() {
	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
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

	app.Get("/", func(c fiber.Ctx) error {
		return c.Render("pages/home", fiber.Map{
			"Title": "Vertexia",
		}, "layouts/main")
	})

	log.Fatal(app.Listen(":3000"))
}