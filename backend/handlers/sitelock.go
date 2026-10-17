package handlers

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"vertexia-frontend/backend/config"
)

func IsSiteLockUnlocked(c fiber.Ctx) bool {
	sess := session.FromContext(c)
	if sess == nil {
		return false
	}
	unlocked, ok := sess.Get("sitelock_unlocked").(bool)
	return ok && unlocked
}

func SiteLockGet(c fiber.Ctx) error {
	if !config.Global.IsSiteLockActive() {
		return c.Redirect().To("/")
	}
	if IsSiteLockUnlocked(c) {
		return c.Redirect().To("/")
	}
	return c.Render("pages/sitelock", fiber.Map{
		"Title":  "VERTEXIA",
		"AssetV": AssetVersion(),
	}, "")
}

func SiteLockPost(c fiber.Ctx) error {
	if !config.Global.IsSiteLockActive() {
		return c.Redirect().To("/")
	}
	password := c.FormValue("password")
	expected := config.Global.SiteLockPassword
	match := len(password) > 0 && len(expected) > 0 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1
	if !match {
		return c.Render("pages/sitelock", fiber.Map{
			"Title":  "VERTEXIA",
			"AssetV": AssetVersion(),
			"Error":  "Incorrect password",
		}, "")
	}
	if sess := session.FromContext(c); sess != nil {
		sess.Set("sitelock_unlocked", true)
	}
	return c.Redirect().To("/")
}
