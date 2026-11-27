package handlers

import (
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
)

// isValidStudioRedirectURI allows only loopback callbacks (with an explicit
// port) and the app's custom schemes. The URL is parsed rather than prefix
// matched so "http://localhost:1@evil.example/" is rejected.
func isValidStudioRedirectURI(uri string) bool {
	if uri == "" || len(uri) > 512 || strings.ContainsAny(uri, "\r\n\t\\") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.User != nil || u.Fragment != "" {
		return false
	}
	switch u.Scheme {
	case "vertexia-studio", "vertexia":
		return true
	case "http", "https":
		host := u.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return false
		}
		return u.Port() != ""
	default:
		return false
	}
}

func buildStudioRedirectURL(base, code, state string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	u := base + sep + "code=" + url.QueryEscape(code)
	if state != "" {
		u += "&state=" + url.QueryEscape(state)
	}
	return u
}

func StudioAuthGet(c fiber.Ctx) error {
	state := c.Query("state")
	redirectURI := c.Query("redirect_uri")

	if redirectURI != "" && !isValidStudioRedirectURI(redirectURI) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid redirect_uri")
	}
	if redirectURI == "" {
		redirectURI = "vertexia-studio://auth"
	}

	username := GetActiveUser(c)

	if username == "" {
		next := "/studio/auth"
		q := url.Values{}
		if state != "" {
			q.Set("state", state)
		}
		if redirectURI != "" {
			q.Set("redirect_uri", redirectURI)
		}
		if len(q) > 0 {
			next += "?" + q.Encode()
		}
		return c.Redirect().To("/login?next=" + url.QueryEscape(next))
	}

	if service.StudioAuth != nil && state != "" {
		service.StudioAuth.RegisterPendingState(state, redirectURI)
	}

	var displayName string
	var userID int
	if service.User != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			displayName = u.DisplayName
			if displayName == "" {
				displayName = u.Username
			}
			userID = u.ID
		}
	}

	return Render(c, "pages/studio_auth", fiber.Map{
		"Title":       "Authorize VERTEXIA Studio - VERTEXIA",
		"State":       state,
		"RedirectURI": redirectURI,
		"Username":    username,
		"DisplayName": displayName,
		"UserID":      userID,
	}, "layouts/main")
}

func StudioAuthAllow(c fiber.Ctx) error {
	state := c.FormValue("state")
	redirectURI := c.FormValue("redirect_uri")

	if redirectURI == "" {
		redirectURI = c.Query("redirect_uri")
	}
	if state == "" {
		state = c.Query("state")
	}
	if redirectURI != "" && !isValidStudioRedirectURI(redirectURI) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid redirect_uri")
	}
	if redirectURI == "" {
		redirectURI = "vertexia-studio://auth"
	}

	username := GetActiveUser(c)
	if username == "" {
		return c.Redirect().To("/login")
	}
	if service.User == nil || service.StudioAuth == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Service unavailable")
	}
	user, err := service.User.GetUserByUsername(username)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	var code string
	if state != "" {
		code, err = service.StudioAuth.GenerateCodeForState(state, user.ID)
	} else {
		code, err = service.StudioAuth.GenerateCode(user.ID)
	}
	if err != nil {
		recordInternalError(c, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate code")
	}

	target := buildStudioRedirectURL(redirectURI, code, state)
	return c.Redirect().To(target)
}

func StudioAuthDeny(c fiber.Ctx) error {
	state := c.FormValue("state")
	if state == "" {
		state = c.Query("state")
	}
	redirectURI := c.FormValue("redirect_uri")
	if redirectURI == "" {
		redirectURI = c.Query("redirect_uri")
	}
	if redirectURI == "" {
		redirectURI = "vertexia-studio://auth"
	}
	if !isValidStudioRedirectURI(redirectURI) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid redirect_uri")
	}
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	target := redirectURI + sep + "error=access_denied"
	if state != "" {
		target += "&state=" + url.QueryEscape(state)
	}
	return c.Redirect().To(target)
}

// studioExchangeHandler redeems a one-time browser authorization code for a
// short-lived opaque access token. defaultKind selects the token lifetime when
// the body does not name one ("client" or "studio"). The long-lived Unikey is
// never returned.
func studioExchangeHandler(defaultKind string) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Code   string `json:"code" form:"code"`
			Client string `json:"client" form:"client"`
		}
		_ = c.Bind().Body(&body)
		if body.Code == "" {
			body.Code = c.Query("code")
		}
		code := strings.TrimSpace(body.Code)
		if code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing code"})
		}
		if service.StudioAuth == nil || service.Tokens == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Service unavailable"})
		}
		user, err := service.StudioAuth.ConsumeCode(code)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or expired code"})
		}
		kind := defaultKind
		if strings.TrimSpace(body.Client) != "" {
			kind = service.NormalizeTokenKind(body.Client)
		}
		raw, tok, err := service.Tokens.Issue(user.ID, kind)
		if err != nil {
			recordInternalError(c, err)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "unavailable"})
		}
		display := user.DisplayName
		if display == "" {
			display = user.Username
		}
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.JSON(fiber.Map{
			"uid":          user.ID,
			"username":     user.Username,
			"displayname":  display,
			"display_name": display,
			"access_token": raw,
			"token_type":   "Bearer",
			"token_kind":   tok.Kind,
			"expires_at":   tok.ExpiresAt.Unix(),
			"expires_in":   int64(time.Until(tok.ExpiresAt).Seconds()),
		})
	}
}

// StudioAuthExchange is POST /api/v1/studio/exchange (Studio token by default).
var StudioAuthExchange = studioExchangeHandler(service.TokenKindStudio)

// ClientAuthExchange is POST /api/v1/auth/exchange (client token by default).
var ClientAuthExchange = studioExchangeHandler(service.TokenKindClient)

func StudioAuthPoll(c fiber.Ctx) error {
	state := c.Query("state")
	if state == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing state"})
	}
	if service.StudioAuth == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Service unavailable"})
	}
	code, ok := service.StudioAuth.GetPendingCode(state)
	if !ok {
		return c.JSON(fiber.Map{"status": "pending"})
	}
	return c.JSON(fiber.Map{"status": "authorized", "code": code})
}
