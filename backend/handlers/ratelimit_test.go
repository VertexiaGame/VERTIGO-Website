package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/template/html/v3"
)

func rateLimitTestApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ServerHeader: ""})
	app.Get("/api/v1/ping", RateLimitReached)
	app.Get("/page", RateLimitReached)
	app.Get("/hx", func(c fiber.Ctx) error {
		c.Request().Header.Set("HX-Request", "true")
		return RateLimitReached(c)
	})
	return app
}

func TestRateLimitReachedAPIJSON(t *testing.T) {
	app := rateLimitTestApp(t)
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	req.Header.Set("Retry-After", "")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusTooManyRequests)
	}
	if got := resp.Header.Get(fiber.HeaderRetryAfter); got == "" {
		t.Fatalf("missing Retry-After header")
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "rate_limited" {
		t.Fatalf("error = %v, want rate_limited", body["error"])
	}
	retryAfter, ok := body["retry_after"].(float64)
	if !ok || retryAfter < 1 {
		t.Fatalf("retry_after = %v, want positive number", body["retry_after"])
	}
}

func TestRateLimitReachedPreservesRetryAfter(t *testing.T) {
	app := rateLimitTestApp(t)
	req := httptest.NewRequest("GET", "/hx", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusTooManyRequests)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["retry_after"] != float64(60) {
		t.Fatalf("retry_after = %v, want 60 fallback", body["retry_after"])
	}
}

func TestRateLimitReachedPageHTML(t *testing.T) {
	engine := html.New("../../views", ".html")
	app := fiber.New(fiber.Config{ServerHeader: "", Views: engine})
	app.Get("/somepage", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderRetryAfter, "45")
		return RateLimitReached(c)
	})
	req := httptest.NewRequest("GET", "/somepage", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusTooManyRequests)
	}
	if got := resp.Header.Get(fiber.HeaderRetryAfter); got != "45" {
		t.Fatalf("Retry-After = %q, want 45", got)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "rate limited") {
		t.Fatalf("page body missing rate limit message")
	}
	if !strings.Contains(body, "retryAfterCount") {
		t.Fatalf("page body missing countdown element")
	}
}
