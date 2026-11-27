package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"vertexia-frontend/backend/kv"
	"vertexia-frontend/backend/renderer"
	"vertexia-frontend/backend/routes"
	"vertexia-frontend/backend/service"
	"vertexia-frontend/backend/storage"
)

// connectRedis connects the shared Redis client. When Redis is required
// (production) a failure is fatal; in development the app continues with
// process-local fallbacks and says so loudly.
func connectRedis(cfg *config.Config) {
	kv.SetRequired(cfg.RedisRequired)
	client, err := kv.Connect(kv.Options{
		URL:      cfg.RedisURL,
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		TLS:      cfg.RedisTLS,
	})
	if err != nil {
		if cfg.RedisRequired {
			log.Fatalf("Redis error! ########## %v (REDIS_REQUIRED is set; refusing to start without shared state)", err)
		}
		log.Printf("WARNING: Redis unavailable (%v); using process-local state. This is only acceptable in development.", err)
		return
	}
	kv.SetClient(client)
	log.Printf("Redis connected")
}

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
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config error! ########## %v", err)
	}

	connectRedis(cfg)

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("DB error! ########## %v", err)
	}

	if err := storage.Init(cfg); err != nil {
		log.Fatalf("Storage error! ########## %v", err)
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
		ProxyHeader:  cfg.ProxyHeader,
		TrustProxy:   cfg.TrustProxy,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:  cfg.TrustedProxies,
			Loopback: cfg.TrustProxy,
			Private:  cfg.TrustProxy,
		},
		EnableIPValidation: true,
	})

	app.Use(recoverer.New())

	// Probes are registered before every other middleware so site lock,
	// rate limits and sessions never interfere with them.
	app.Get("/healthz", handlers.Healthz)
	app.Get("/readyz", handlers.Readyz)

	app.Use(handlers.LogInternalServerErrors)

	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Frame-Options", "SAMEORIGIN")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval' https: http: data: blob: ws: wss:; worker-src 'self' blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' https: http:; style-src 'self' 'unsafe-inline' https: http:; img-src 'self' https: http: data: blob:; font-src 'self' https: http: data:; connect-src 'self' https: http: ws: wss: data: blob:; frame-src 'self' vertexia-client:; frame-ancestors 'self';")
		return c.Next()
	})

	app.Use(handlers.RequestBodyLimit())

	limiterMax := cfg.LimiterMax
	if limiterMax <= 0 {
		limiterMax = 1000
	}
	limiterExpiration := cfg.LimiterExpiration
	if limiterExpiration <= 0 {
		limiterExpiration = time.Minute
	}
	app.Use(limiter.New(limiter.Config{
		Max:          limiterMax,
		Expiration:   limiterExpiration,
		LimitReached: handlers.RateLimitReached,
		KeyGenerator: func(c fiber.Ctx) string { return "global:" + c.IP() },
		Storage:      handlers.LimiterStorage(),
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/ws/")
		},
	}))

	app.Use(favicon.New(favicon.Config{
		File: "./static/branding/favicon.ico",
		URL:  "/favicon.ico",
	}))

	sessionConfig := session.Config{
		Storage:        kv.NewStorage("session:"),
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

	app.Use(handlers.CSRFProtect())

	routes.Setup(app)

	fanoutCtx, stopFanout := context.WithCancel(context.Background())
	defer stopFanout()
	if handlers.StartFeedFanout(fanoutCtx) {
		log.Printf("Feed WebSocket fan-out via Redis pub/sub enabled")
	}

	go func() {
		defer close(shutdownDone)
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		log.Printf("Received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Printf("HTTP shutdown: %v", err)
		}
		stopFanout()
		if service.GSM != nil {
			if err := service.GSM.Shutdown(ctx); err != nil {
				log.Printf("GSM shutdown: %v", err)
			}
		}
		if database.DB != nil {
			_ = database.DB.Close()
		}
		if c := kv.Client(); c != nil {
			_ = c.Close()
		}
	}()

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
				serve(app.Listen(addr, fiber.ListenConfig{
					CertFile:    certFile,
					CertKeyFile: keyFile,
				}))
				return
			}
		}
	}
	log.Printf("Fiber listening on %s (Keystone PORT env)", addr)
	serve(app.Listen(addr))
}

// shutdownDone is closed once the signal handler has released resources.
var shutdownDone = make(chan struct{})

// serve reports the listener result. A clean shutdown returns nil from
// Listen; then wait for the signal handler to finish closing resources.
func serve(err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
	select {
	case <-shutdownDone:
	case <-time.After(30 * time.Second):
	}
	log.Printf("Server stopped")
}
