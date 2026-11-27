package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

var errNoBearer = errors.New("missing bearer token")

// bearerToken returns the raw token from "Authorization: Bearer <token>".
func bearerToken(c fiber.Ctx) string {
	h := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if len(h) < 7 || !strings.EqualFold(h[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

// hasAccessToken reports whether the request carries one of our opaque
// access tokens (as opposed to, for example, a gameserver key).
func hasAccessToken(c fiber.Ctx) bool {
	return strings.HasPrefix(bearerToken(c), "vxa_")
}

// bearerUser resolves the user behind a client/Studio access token.
func bearerUser(c fiber.Ctx) (*models.User, service.AccessToken, error) {
	raw := bearerToken(c)
	if raw == "" {
		return nil, service.AccessToken{}, errNoBearer
	}
	if service.Tokens == nil || service.User == nil {
		return nil, service.AccessToken{}, errors.New("service unavailable")
	}
	tok, err := service.Tokens.Validate(raw)
	if err != nil {
		return nil, service.AccessToken{}, err
	}
	user, err := service.User.GetUserByID(tok.UserID)
	if err != nil || user == nil {
		return nil, service.AccessToken{}, service.ErrInvalidToken
	}
	return user, tok, nil
}

// requestUser accepts either a bearer access token (native client, Studio)
// or the browser session. A present but invalid bearer token is rejected
// instead of silently falling back to the cookie.
func requestUser(c fiber.Ctx) (*models.User, error) {
	if bearerToken(c) != "" {
		user, _, err := bearerUser(c)
		return user, err
	}
	username := GetActiveUser(c)
	if username == "" || service.User == nil {
		return nil, errNoBearer
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return nil, errNoBearer
	}
	return user, nil
}

// legacyUnikey returns the permanent per-user Unikey. Headers win; the query
// parameters are deprecated and kept only because the shipped native client
// and gameserver still send ?ukey=. New clients use bearer access tokens.
func legacyUnikey(c fiber.Ctx) string {
	for _, h := range []string{"X-User-Key", "X-Unikey", "X-Ukey"} {
		if v := strings.TrimSpace(c.Get(h)); v != "" {
			return strings.Trim(v, "\"")
		}
	}
	for _, q := range []string{"ukey", "unikey", "user_key"} {
		if v := strings.TrimSpace(c.Query(q)); v != "" {
			return strings.Trim(v, "\"")
		}
	}
	return ""
}

// credentialUser resolves the caller from a bearer access token, the legacy
// Unikey or the browser session, in that order. It returns nil when none is
// valid; a present but invalid bearer token never falls through.
func credentialUser(c fiber.Ctx) *models.User {
	if service.User == nil {
		return nil
	}
	if hasAccessToken(c) {
		u, _, err := bearerUser(c)
		if err != nil {
			return nil
		}
		return u
	}
	if ukey := legacyUnikey(c); ukey != "" {
		if u, err := service.User.GetUserByUnikey(ukey); err == nil && u != nil {
			return u
		}
	}
	if username := GetActiveUser(c); username != "" {
		if u, err := service.User.GetUserByUsername(username); err == nil && u != nil {
			return u
		}
	}
	return nil
}

func unauthorizedJSON(c fiber.Ctx) error {
	c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="vertexia"`)
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
}

// MeGet implements GET /api/v1/me (Bearer).
func MeGet(c fiber.Ctx) error {
	user, _, err := bearerUser(c)
	if err != nil {
		return unauthorizedJSON(c)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	display := user.DisplayName
	if display == "" {
		display = user.Username
	}
	return c.JSON(fiber.Map{"id": user.ID, "username": user.Username, "display_name": display})
}

// AuthLogoutPost implements POST /api/v1/auth/logout (Bearer): revokes the
// presented access token.
func AuthLogoutPost(c fiber.Ctx) error {
	raw := bearerToken(c)
	if raw == "" || service.Tokens == nil {
		return unauthorizedJSON(c)
	}
	if _, err := service.Tokens.Validate(raw); err != nil {
		return unauthorizedJSON(c)
	}
	if err := service.Tokens.Revoke(raw); err != nil {
		recordInternalError(c, err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "unavailable"})
	}
	return c.JSON(fiber.Map{"success": true})
}
