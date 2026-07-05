package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/fiber/v3/middleware/favicon"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
)

func render(c fiber.Ctx, view string, data fiber.Map, layouts ...string) error {
	layout := "layouts/main"
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	if c.Get("HX-Request") == "true" {
		if layout == "layouts/main" {
			layout = "layouts/htmx"
		}
	}
	return c.Render(view, data, layout)
}

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
		Views:        engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ServerHeader: "",
	})

	app.Use(recoverer.New())


	//We should not allow more than 100 requests X minute from same IP
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
	}))

	app.Use(favicon.New(favicon.Config{
	    File:	"./static/branding/favicon.ico",
	    URL:	"/favicon.ico",
	}))

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