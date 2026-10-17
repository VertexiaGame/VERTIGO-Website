package handlers

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/service"
)

func isValidStudioRedirectURI(uri string) bool {
	if uri == "" {
		return false
	}
	if strings.HasPrefix(uri, "http://localhost:") || strings.HasPrefix(uri, "http://127.0.0.1:") || strings.HasPrefix(uri, "http://[::1]:") {
		return true
	}
	if strings.HasPrefix(uri, "https://localhost:") || strings.HasPrefix(uri, "https://127.0.0.1:") {
		return true
	}
	if strings.HasPrefix(uri, "vertexia-studio://") || strings.HasPrefix(uri, "vertexia://") {
		return true
	}
	return false
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

func StudioAuthExchange(c fiber.Ctx) error {
	var body struct {
		Code string `json:"code" form:"code" query:"code"`
	}
	_ = c.Bind().JSON(&body)
	if body.Code == "" {
		body.Code = c.Query("code")
	}
	if body.Code == "" {
		body.Code = c.FormValue("code")
	}
	if body.Code == "" {
		var raw struct {
			Code string `json:"code"`
		}
		_ = c.Bind().Body(&raw)
		if raw.Code != "" {
			body.Code = raw.Code
		}
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing code"})
	}
	if service.StudioAuth == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Service unavailable"})
	}
	user, err := service.StudioAuth.ConsumeCode(code)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"uid":      user.ID,
		"username": user.Username,
		"displayname": user.DisplayName,
		"ukey":     user.Unikey,
	})
}

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
