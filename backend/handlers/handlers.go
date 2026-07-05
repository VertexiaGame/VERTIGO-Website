package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func Home(c fiber.Ctx) error {
	return Render(c, "pages/home", fiber.Map{
		"Title": "VERTEXIA",
	}, "layouts/main")
}

func LoginGet(c fiber.Ctx) error {
	return Render(c, "pages/login", fiber.Map{
		"Title": "Log In - VERTEXIA",
	}, "layouts/main")
}

func Logout(c fiber.Ctx) error {
	sess := session.FromContext(c)
	_ = sess.Destroy()
	return c.Redirect("/")
}