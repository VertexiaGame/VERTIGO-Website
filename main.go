package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/favicon"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/template/html/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/handlers"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/routes"
	"vertexia-frontend/backend/service"
)

func resolveListenAddr() string {
	for _, key := range []string{"PORT", "ADDR", "LISTEN_ADDR"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if strings.HasPrefix(v, ":") {
				return v
			}
			if strings.Contains(v, ":") {
				return v
			}
			return ":" + v
		}
	}
	return ":3000"
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error! ########## %v", err)
	}

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("DB error! ########## %v", err)
	}

	service.Init(database.DB)

	renderer.Init()

	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views:        engine,
		BodyLimit:    40 * 1024 * 1024,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerIdleTimeout,
		ServerHeader: "",
		ErrorHandler: handlers.ErrorHandler,
	})

	app.Use(recoverer.New())

	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Frame-Options", "SAMEORIGIN")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval' https: http: data: blob: ws: wss:; worker-src 'self' blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' https: http:; style-src 'self' 'unsafe-inline' https: http:; img-src 'self' https: http: data: blob:; font-src 'self' https: http: data:; connect-src 'self' https: http: ws: wss: data: blob:; frame-ancestors 'self';")
		return c.Next()
	})

	limiterMax := cfg.LimiterMax
	if limiterMax <= 0 {
		limiterMax = 1000
	}
	limiterExpiration := cfg.LimiterExpiration
	if limiterExpiration <= 0 {
		limiterExpiration = time.Minute
	}
	app.Use(limiter.New(limiter.Config{
		Max:        limiterMax,
		Expiration: limiterExpiration,
		LimitReached: handlers.RateLimitReached,
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/ws/")
		},
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

	addr := resolveListenAddr()
	certFile := os.Getenv("TLS_CERT")
	if certFile == "" {
		certFile = os.Getenv("SSL_CERT")
	}
	keyFile := os.Getenv("TLS_KEY")
	if keyFile == "" {
		keyFile = os.Getenv("SSL_KEY")
	}
	tlsEnabled := os.Getenv("TLS_ENABLED") == "true" || (certFile != "" && keyFile != "")
	if tlsEnabled && certFile != "" && keyFile != "" {
		if _, err := os.Stat(certFile); err == nil {
			if _, err2 := os.Stat(keyFile); err2 == nil {
				log.Printf("Fiber TLS enabled on %s cert=%s key=%s", addr, certFile, keyFile)
				log.Fatal(app.Listen(addr, fiber.ListenConfig{
					CertFile:    certFile,
					CertKeyFile: keyFile,
				}))
			}
		}
	}
	log.Printf("Fiber listening on %s (Keystone PORT env)", addr)
	log.Fatal(app.Listen(addr))
}
