package handlers

import (
	"database/sql"
	"errors"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

func serveFeedImage(c fiber.Ctx, pending bool) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 || service.Feed == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	var image models.ForumImageUpload
	if pending {
		image, err = service.Feed.GetPendingImage(id)
	} else {
		image, err = service.Feed.GetImage(id, activeUserID(c))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		log.Printf("Feed image %d failed: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, image.ContentType)
	return c.Send(image.Content)
}

func FeedImageGet(c fiber.Ctx) error {
	return serveFeedImage(c, false)
}

func AdminFeedImageGet(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	return serveFeedImage(c, true)
}

func adminFeedImagesPath(after int) string {
	path := "/admin?tab=feedimages"
	if after > 0 {
		path += "&after=" + strconv.Itoa(after)
	}
	return path
}

func loadAdminFeedImages(c fiber.Ctx) (fiber.Map, error) {
	after, _ := strconv.Atoi(c.Query("after"))
	after = max(0, after)
	const pageSize = 24
	images, err := service.Feed.PendingImages(after, pageSize+1)
	if err != nil {
		return nil, err
	}
	next := 0
	if len(images) > pageSize {
		images = images[:pageSize]
		next = images[pageSize-1].ID
	}
	return fiber.Map{"Images": images, "After": after, "Next": next, "CSRF": getCSRFToken(c)}, nil
}

func AdminFeedImageReviewPost(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	if !csrfValid(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}
	action := c.FormValue("action")
	if action != "approve" && action != "reject" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if service.Feed == nil {
		return serveErrorPage(c, fiber.StatusServiceUnavailable)
	}
	reviewed, err := service.Feed.ReviewImage(id, action == "approve")
	if err != nil {
		log.Printf("Feed image %d review failed: %v", id, err)
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}
	if !reviewed {
		return c.Status(fiber.StatusConflict).SendString("This image has already been reviewed.")
	}
	if c.Get("X-Admin-Image-Queue") == "true" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	after, _ := strconv.Atoi(c.FormValue("after"))
	return c.Redirect().To(adminFeedImagesPath(after))
}
