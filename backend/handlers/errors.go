package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type internalErrorCauseKey struct{}

func recordInternalError(c fiber.Ctx, err error) {
	c.Locals(internalErrorCauseKey{}, err)
}

func LogInternalServerErrors(c fiber.Ctx) error {
	err := c.Next()
	if err == nil && c.Response().StatusCode() == fiber.StatusInternalServerError {
		reason := "no cause reported by handler"
		if cause, ok := c.Locals(internalErrorCauseKey{}).(error); ok {
			reason = cause.Error()
		} else if detail := internalErrorResponseDetail(c); detail != "" {
			reason = detail
		}
		logServerError(c, fiber.StatusInternalServerError, reason)
	}
	return err
}

func internalErrorResponseDetail(c fiber.Ctx) string {
	if c.Response().IsBodyStream() {
		return ""
	}
	contentType, _, _ := strings.Cut(c.GetRespHeader(fiber.HeaderContentType), ";")
	if contentType != fiber.MIMETextPlain && contentType != fiber.MIMEApplicationJSON {
		return ""
	}
	body := c.Response().Body()
	if len(body) > 4096 {
		return ""
	}
	if contentType == fiber.MIMETextPlain {
		return strings.TrimSpace(string(body))
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return ""
	}
	for _, field := range []string{"error", "message", "reason"} {
		var detail string
		if json.Unmarshal(fields[field], &detail) == nil && detail != "" {
			return detail
		}
	}
	return ""
}

func logServerError(c fiber.Ctx, code int, reason string) {
	log.Printf("ERROR [%d] %s %q: %q", code, c.Method(), c.Path(), reason)
}

func internalErrorPage(c fiber.Ctx, cause error) error {
	recordInternalError(c, cause)
	return serveErrorPage(c, fiber.StatusInternalServerError)
}

var errorPages = map[int]struct {
	Template string
	Title    string
}{
	fiber.StatusNotFound:            {"pages/errors/404", "404 - Page Not Found - VERTEXIA"},
	fiber.StatusForbidden:           {"pages/errors/403", "403 - Access Denied - VERTEXIA"},
	fiber.StatusTooManyRequests:     {"pages/errors/429", "429 - Too Many Requests - VERTEXIA"},
	fiber.StatusInternalServerError: {"pages/errors/500", "500 - Internal Server Error - VERTEXIA"},
}

func serveErrorPage(c fiber.Ctx, code int) error {
	page, ok := errorPages[code]
	if !ok {
		page = errorPages[fiber.StatusInternalServerError]
	}
	c.Status(code)
	data := fiber.Map{"Title": page.Title}
	if code == fiber.StatusTooManyRequests {
		retryAfter, err := strconv.Atoi(c.GetRespHeader(fiber.HeaderRetryAfter))
		if err != nil || retryAfter < 1 {
			retryAfter = 60
		}
		data["RetryAfter"] = retryAfter
	}
	if err := Render(c, page.Template, data); err != nil {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		return c.Status(code).SendString(page.Title)
	}
	return nil
}

func ErrorHandler(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	code := fiber.StatusInternalServerError
	var e *fiber.Error
	if errors.As(err, &e) && e != nil && e.Code > 0 {
		code = e.Code
	}

	if code >= fiber.StatusInternalServerError {
		logServerError(c, code, err.Error())
	}

	return serveErrorPage(c, code)
}

func NotFoundHandler(c fiber.Ctx) error {
	return serveErrorPage(c, fiber.StatusNotFound)
}
