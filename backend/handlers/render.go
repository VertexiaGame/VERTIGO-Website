package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func Render(c fiber.Ctx, view string, data fiber.Map, layouts ...string) error {
	layout := "layouts/main"
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	if c.Get("HX-Request") == "true" {
		if layout == "layouts/main" {
			layout = "layouts/htmx"
		}
	}

	sess := session.FromContext(c)
	if sess != nil {
		if username, ok := sess.Get("username").(string); ok && username != "" {
			data["Username"] = username
			data["IsLoggedIn"] = true
		} else {
			data["Username"] = ""
			data["IsLoggedIn"] = false
		}
	}

	return c.Render(view, data, layout)
}