package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
	"vertexia-frontend/backend/database"
	"vertexia-frontend/backend/kv"
)

// Healthz is the liveness probe: the process is up and serving HTTP.
func Healthz(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(fiber.Map{"status": "ok"})
}

// Readyz is the readiness probe: MySQL and Redis must answer. Errors are
// reported as booleans only so the endpoint leaks no infrastructure detail.
func Readyz(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	dbOK := database.Ping(ctx) == nil
	redisOK := true
	if kv.Client() != nil || (config.Global != nil && config.Global.RedisRequired) {
		redisOK = kv.Ping(ctx) == nil
	}
	status := fiber.StatusOK
	if !dbOK || !redisOK {
		status = fiber.StatusServiceUnavailable
	}
	return c.Status(status).JSON(fiber.Map{"ready": status == fiber.StatusOK, "database": dbOK, "redis": redisOK})
}
