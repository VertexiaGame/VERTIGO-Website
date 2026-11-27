package handlers

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/config"
)

// CSRFProtect rejects cross-site state-changing requests. Browsers attach the
// session and remember-me cookies automatically, so every unsafe request
// that does not carry explicit header credentials must come from our own
// origin, proven by Sec-Fetch-Site, Origin or Referer. Requests without any
// of those headers are not from a modern browser (native client, Studio,
// gameserver, curl) and cannot be forged cross-site.
func CSRFProtect() fiber.Handler {
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
			return c.Next()
		}
		if bearerToken(c) != "" || c.Get("X-Gameserver-Key") != "" {
			// Custom headers cannot be attached to a cross-site request
			// without a CORS preflight, which this app never grants.
			return c.Next()
		}
		if !sameOriginRequest(c) {
			return csrfRejected(c)
		}
		return c.Next()
	}
}

func csrfRejected(c fiber.Ctx) error {
	if rateLimitWantsJSON(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "csrf", "message": "Cross-site request rejected"})
	}
	return c.Status(fiber.StatusForbidden).SendString("Cross-site request rejected")
}

func sameOriginRequest(c fiber.Ctx) bool {
	switch strings.ToLower(c.Get("Sec-Fetch-Site")) {
	case "same-origin", "none":
		return true
	case "cross-site", "same-site":
		// same-site is rejected too: sibling subdomains of the parent domain
		// are separate applications.
		return false
	}
	if origin := c.Get(fiber.HeaderOrigin); origin != "" {
		return trustedOriginHost(c, origin)
	}
	if ref := c.Get(fiber.HeaderReferer); ref != "" {
		return trustedOriginHost(c, ref)
	}
	return true
}

func trustedOriginHost(c fiber.Ctx, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	host := strings.ToLower(u.Host)
	if strings.EqualFold(host, c.Host()) {
		return true
	}
	if config.Global != nil && config.Global.PublicBaseURL != "" {
		if pub, err := url.Parse(config.Global.PublicBaseURL); err == nil && strings.EqualFold(host, pub.Host) {
			return true
		}
	}
	return false
}

// WebSocketOriginAllowed guards the cookie-authenticated feed socket against
// cross-site WebSocket hijacking. The handshake is a GET, so CSRFProtect does
// not see it. Non-browser clients send no Origin and are allowed.
func WebSocketOriginAllowed(c fiber.Ctx) bool {
	origin := c.Get(fiber.HeaderOrigin)
	if origin == "" {
		return true
	}
	return trustedOriginHost(c, origin)
}
