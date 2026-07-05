package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/favicon"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/routes"
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
		Views:        engine,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerIdleTimeout,
		ServerHeader: "",
	})

	app.Use(recoverer.New())

	//We should not allow more than 100 requests X minute from same IP
	app.Use(limiter.New(limiter.Config{
		Max:        cfg.LimiterMax,
		Expiration: cfg.LimiterExpiration,
	}))

	app.Use(favicon.New(favicon.Config{
		File: "./static/branding/favicon.ico",
		URL:  "/favicon.ico",
	}))

	sessionConfig := session.Config{
		CookieHTTPOnly: true,
		CookieSecure:   cfg.SessionSecure,
		CookieSameSite: cfg.SessionSameSite,
	}
	if cfg.SessionIdleTimeout > 0 {
		sessionConfig.IdleTimeout = cfg.SessionIdleTimeout
	}
	if cfg.SessionAbsoluteTimeout > 0 && cfg.SessionAbsoluteTimeout >= cfg.SessionIdleTimeout {
		sessionConfig.AbsoluteTimeout = cfg.SessionAbsoluteTimeout
	}
	app.Use(session.New(sessionConfig))

	routes.Setup(app)

	log.Fatal(app.Listen(":3000"))
}