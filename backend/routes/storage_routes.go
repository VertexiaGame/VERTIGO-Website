package routes

import (
	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/handlers"
)

// setupStorageRoutes serves runtime-written public files from the object
// store at the URLs the static middleware used to serve them from disk, for
// example /static/uploads/games/7_0.png or /assets/uploads/shop/hat/3.png.
// Only keys under handlers.PublicStoragePrefixes are served; private keys,
// unknown paths and missing objects fall through to the next handler so the
// render-on-miss handlers and the bundled static files still work. It must be
// registered before the static middleware.
func setupStorageRoutes(app *fiber.App) {
	for _, mount := range []string{"/static/uploads/*", "/static/renders/*", "/assets/uploads/*"} {
		app.Get(mount, handlers.StoredFileGet)
	}
}
