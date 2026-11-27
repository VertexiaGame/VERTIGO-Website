package handlers

import (
	"bytes"
	"errors"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func TestInternalServerErrorsAreLogged(t *testing.T) {
	tests := []struct {
		name   string
		handle fiber.Handler
		status int
		reason string
	}{
		{"returned", func(c fiber.Ctx) error { return errors.New("connection failed") }, 500, "connection failed"},
		{"panic", func(c fiber.Ctx) error { panic("unexpected state") }, 500, "unexpected state"},
		{"page", func(c fiber.Ctx) error { return internalErrorPage(c, errors.New("database offline")) }, 500, "database offline"},
		{"text", func(c fiber.Ctx) error { return c.Status(500).SendString("service unavailable") }, 500, "service unavailable"},
		{"json", func(c fiber.Ctx) error { return c.Status(500).JSON(fiber.Map{"error": "could not save"}) }, 500, "could not save"},
		{"bad request", func(c fiber.Ctx) error { return c.SendStatus(400) }, 400, ""},
	}

	var output bytes.Buffer
	writer := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(writer) })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Reset()
			app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
			app.Use(recover.New())
			app.Use(LogInternalServerErrors)
			app.Get("/failure", tt.handle)
			res, err := app.Test(httptest.NewRequest("GET", "/failure?secret=hidden", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.status)
			}
			got := output.String()
			if tt.reason == "" {
				if got != "" {
					t.Fatalf("unexpected log: %s", got)
				}
				return
			}
			want := `ERROR [500] GET "/failure": "` + tt.reason + `"`
			if strings.Count(got, "ERROR [500]") != 1 || !strings.Contains(got, want) || strings.Contains(got, "secret=") {
				t.Fatalf("log = %q, want one entry containing %q", got, want)
			}
		})
	}
}
