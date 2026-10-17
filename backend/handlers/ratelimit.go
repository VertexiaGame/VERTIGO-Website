package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

const (
	gameCreateUploadMax = 10
	gameCreateUploadTTL = time.Hour
	gameUpdateUploadMax = 30
	gameUpdateUploadTTL = time.Hour
	shopCreateUploadMax = 10
	shopCreateUploadTTL = time.Hour
	assetUploadMax      = 20
	assetUploadTTL      = time.Hour
	createPreviewMax    = 60
	createPreviewTTL    = time.Minute
)

func rateLimitRetryAfter(c fiber.Ctx) int {
	if secs, err := strconv.Atoi(c.GetRespHeader(fiber.HeaderRetryAfter)); err == nil && secs >= 1 && secs <= 3600 {
		return secs
	}
	return 60
}

func rateLimitWantsJSON(c fiber.Ctx) bool {
	if c.Get("HX-Request") == "true" {
		return true
	}
	if strings.HasPrefix(c.Path(), "/api/") {
		return true
	}
	return strings.Contains(c.Get("Accept"), "json")
}

func RateLimitReached(c fiber.Ctx) error {
	retryAfter := rateLimitRetryAfter(c)
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(retryAfter))
	if rateLimitWantsJSON(c) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "rate_limited",
			"message":     "Slow down. You are being rate limited.",
			"retry_after": retryAfter,
		})
	}
	c.Status(fiber.StatusTooManyRequests)
	return Render(c, "pages/errors/429", fiber.Map{
		"Title":      "429 - Too Many Requests - VERTEXIA",
		"RetryAfter": retryAfter,
	})
}

func uploadLimitKey(c fiber.Ctx) string {
	if username := GetActiveUser(c); username != "" {
		return "upload:" + strings.ToLower(username)
	}
	return "upload:ip:" + c.IP()
}

func newUploadLimiter(max int, expiration time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   expiration,
		KeyGenerator: uploadLimitKey,
		LimitReached: RateLimitReached,
	})
}

var (
	gameCreateLimiter    = newUploadLimiter(gameCreateUploadMax, gameCreateUploadTTL)
	gameUpdateLimiter    = newUploadLimiter(gameUpdateUploadMax, gameUpdateUploadTTL)
	shopCreateLimiter    = newUploadLimiter(shopCreateUploadMax, shopCreateUploadTTL)
	assetUploadLimiter   = newUploadLimiter(assetUploadMax, assetUploadTTL)
	createPreviewLimiter = newUploadLimiter(createPreviewMax, createPreviewTTL)
)

func GameCreateLimiter() fiber.Handler {
	return gameCreateLimiter
}

func GameUpdateLimiter() fiber.Handler {
	return gameUpdateLimiter
}

func ShopCreateLimiter() fiber.Handler {
	return shopCreateLimiter
}

func AssetUploadLimiter() fiber.Handler {
	return assetUploadLimiter
}

func CreatePreviewLimiter() fiber.Handler {
	return createPreviewLimiter
}
