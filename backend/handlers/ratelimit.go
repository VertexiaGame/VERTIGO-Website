package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"vertexia-frontend/backend/kv"
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
	clanCreateMax       = 5
	clanCreateTTL       = time.Hour
	clanUpdateMax       = 30
	clanUpdateTTL       = time.Hour
	clanActionMax       = 120
	clanActionTTL       = time.Minute
	authAttemptMax      = 10
	authAttemptTTL      = time.Minute
	altchaMax           = 30
	altchaTTL           = time.Minute
	externalAPIMax      = 20
	externalAPITTL      = time.Minute
	studioPollMax       = 120
	studioPollTTL       = time.Minute
	usersCardsMax       = 300
	usersCardsTTL       = time.Minute
	tokenExchangeMax    = 20
	tokenExchangeTTL    = time.Minute
	gameJoinMax         = 30
	gameJoinTTL         = time.Minute
)

// limiterStorage keeps every rate-limit counter in Redis so limits hold
// across platform instances and restarts.
var limiterStorage = kv.NewStorage("ratelimit:")

// LimiterStorage is the shared counter store for limiters built outside this
// package (the global limiter in main).
func LimiterStorage() *kv.Storage { return limiterStorage }

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

func newUploadLimiter(prefix string, max int, expiration time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   expiration,
		KeyGenerator: func(c fiber.Ctx) string { return prefix + uploadLimitKey(c) },
		LimitReached: RateLimitReached,
		Storage:      limiterStorage,
	})
}

// newPrefixedIPLimiter keeps a separate counter namespace so endpoints that
// share an IP budget size do not drain each other.
func newPrefixedIPLimiter(prefix string, max int, expiration time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          max,
		Expiration:   expiration,
		KeyGenerator: func(c fiber.Ctx) string { return prefix + c.IP() },
		LimitReached: RateLimitReached,
		Storage:      limiterStorage,
	})
}

var (
	gameCreateLimiter    = newUploadLimiter("gamecreate:", gameCreateUploadMax, gameCreateUploadTTL)
	gameUpdateLimiter    = newUploadLimiter("gameupdate:", gameUpdateUploadMax, gameUpdateUploadTTL)
	shopCreateLimiter    = newUploadLimiter("shopcreate:", shopCreateUploadMax, shopCreateUploadTTL)
	assetUploadLimiter   = newUploadLimiter("assetupload:", assetUploadMax, assetUploadTTL)
	createPreviewLimiter = newUploadLimiter("createpreview:", createPreviewMax, createPreviewTTL)
	clanCreateLimiter    = newUploadLimiter("clancreate:", clanCreateMax, clanCreateTTL)
	clanUpdateLimiter    = newUploadLimiter("clanupdate:", clanUpdateMax, clanUpdateTTL)
	clanActionLimiter    = newUploadLimiter("clanaction:", clanActionMax, clanActionTTL)
	authAttemptLimiter   = newPrefixedIPLimiter("auth:", authAttemptMax, authAttemptTTL)
	altchaLimiter        = newPrefixedIPLimiter("altcha:", altchaMax, altchaTTL)
	externalAPILimiter   = newPrefixedIPLimiter("external:", externalAPIMax, externalAPITTL)
	studioPollLimiter    = newPrefixedIPLimiter("studiopoll:", studioPollMax, studioPollTTL)
	usersCardsLimiter    = newPrefixedIPLimiter("userscards:", usersCardsMax, usersCardsTTL)
	tokenExchangeLimiter = newPrefixedIPLimiter("tokenexchange:", tokenExchangeMax, tokenExchangeTTL)
	gameJoinLimiter      = newPrefixedIPLimiter("gamejoin:", gameJoinMax, gameJoinTTL)
)

func TokenExchangeLimiter() fiber.Handler {
	return tokenExchangeLimiter
}

func GameJoinLimiter() fiber.Handler {
	return gameJoinLimiter
}

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

func ClanCreateLimiter() fiber.Handler {
	return clanCreateLimiter
}

func ClanUpdateLimiter() fiber.Handler {
	return clanUpdateLimiter
}

func ClanActionLimiter() fiber.Handler {
	return clanActionLimiter
}

func AuthAttemptLimiter() fiber.Handler {
	return authAttemptLimiter
}

func AltchaLimiter() fiber.Handler {
	return altchaLimiter
}

func ExternalAPILimiter() fiber.Handler {
	return externalAPILimiter
}

func StudioPollLimiter() fiber.Handler {
	return studioPollLimiter
}

func UsersCardsLimiter() fiber.Handler {
	return usersCardsLimiter
}

const (
	smallBodyMax = 1 << 20
	largeBodyMax = 40 << 20
)

var largeBodyPathPrefixes = []string{
	"/create/",
	"/assets",
	"/asset/",
	"/api/v1/assets",
	"/api/v1/asset/",
	"/api/v1/create/preview",
	"/api/v1/games",
	"/api/v1/studio/",
	"/api/v1/shop/items",
	"/settings/banner",
}

func largeBodyAllowed(path string) bool {
	for _, prefix := range largeBodyPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func RequestBodyLimit() fiber.Handler {
	return func(c fiber.Ctx) error {
		length := c.Request().Header.ContentLength()
		if length <= 0 {
			return c.Next()
		}
		limit := int64(smallBodyMax)
		path := strings.ToLower(strings.TrimRight(c.Path(), "/"))
		if c.Method() == fiber.MethodPost && ((strings.HasPrefix(path, "/forum/s/") && strings.HasSuffix(path, "/threads")) ||
			(strings.HasPrefix(path, "/forum/t/") && strings.HasSuffix(path, "/replies")) || path == "/api/v1/feed" || path == "/feed") {
			limit = forumImageRequestLimit
		} else if largeBodyAllowed(c.Path()) {
			limit = int64(largeBodyMax)
		}
		if int64(length) > limit {
			return c.Status(fiber.StatusRequestEntityTooLarge).SendString("Request body too large")
		}
		return c.Next()
	}
}
