package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

const forumImageRequestLimit = (models.ForumImageMaxCount + 1) * models.ForumImageMaxBytes

func forumImageUploads(c fiber.Ctx) ([]models.ForumImageUpload, error) {
	mediaType, _, _ := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if mediaType != "multipart/form-data" {
		return nil, nil
	}
	length := c.Request().Header.ContentLength()
	if length < 0 {
		length = len(c.Request().Body())
	}
	if length > forumImageRequestLimit {
		return nil, errors.New("image upload is too large")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return nil, errors.New("could not read image attachments")
	}
	files := form.File["images"]
	saved := form.Value["saved_images"]
	if len(files)+len(saved) > models.ForumImageMaxCount {
		return nil, errors.New("attach up to 3 images")
	}

	var images []models.ForumImageUpload
	var savedIDs []int64
	for _, value := range saved {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 || slices.Contains(savedIDs, id) {
			return nil, errors.New("invalid saved image selection")
		}
		savedIDs = append(savedIDs, id)
		images = append(images, models.ForumImageUpload{SavedID: id})
	}
	for _, file := range files {
		if file.Size > models.ForumImageMaxBytes {
			return nil, errors.New("each image must be 2 MB or smaller")
		}
		reader, err := file.Open()
		if err != nil {
			return nil, errors.New("could not open image attachment")
		}
		content, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, errors.New("could not read image attachment")
		}

		config, format, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || (format != "jpeg" && format != "png") {
			return nil, errors.New("attach valid PNG or JPEG images")
		}
		const maxPixels = 12_000_000
		if config.Width <= 0 || config.Height <= 0 || config.Width > maxPixels/config.Height {
			return nil, errors.New("each image must be 12 megapixels or smaller")
		}
		if _, _, err := image.Decode(bytes.NewReader(content)); err != nil {
			return nil, errors.New("attach valid PNG or JPEG images")
		}
		name := []rune(strings.ToValidUTF8(file.Filename, "�"))
		images = append(images, models.ForumImageUpload{
			Name: string(name[:min(len(name), 255)]), ContentType: "image/" + format, Content: content,
		})
	}
	return images, nil
}

func hasNewImages(images []models.ForumImageUpload) bool {
	return slices.ContainsFunc(images, func(image models.ForumImageUpload) bool { return image.SavedID == 0 })
}

func SavedImagesGet(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	ownerID := activeUserID(c)
	if ownerID == 0 {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	if service.Forum == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	const pageSize = 12
	images, err := service.Forum.SavedImages(ownerID, before, pageSize+1)
	if err != nil {
		log.Printf("Saved image list failed: %v", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	next := ""
	if len(images) > pageSize {
		images = images[:pageSize]
		next = strconv.FormatInt(images[pageSize-1].ID, 10)
	}
	return c.JSON(fiber.Map{"images": images, "next": next})
}

func SavedImageGet(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	ownerID := activeUserID(c)
	if ownerID == 0 {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if service.Forum == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	image, err := service.Forum.SavedImage(ownerID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		log.Printf("Saved image %d failed: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, image.ContentType)
	return c.Send(image.Content)
}

func serveForumImage(c fiber.Ctx, approved bool) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if service.Forum == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	image, err := service.Forum.GetForumImage(id, approved)
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		log.Printf("Forum image %d failed: %v", id, err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderContentType, image.ContentType)
	return c.Send(image.Content)
}

func ForumImageGet(c fiber.Ctx) error {
	return serveForumImage(c, true)
}

func AdminForumImageGet(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}
	return serveForumImage(c, false)
}

func adminForumImagesPath(after int) string {
	path := "/admin?tab=forumimages"
	if after > 0 {
		path += "&after=" + strconv.Itoa(after)
	}
	return path
}

func AdminForumImagesGet(c fiber.Ctx) error {
	after, _ := strconv.Atoi(c.Query("after"))
	return c.Redirect().To(adminForumImagesPath(after))
}

func loadAdminForumImages(c fiber.Ctx) (fiber.Map, error) {
	after, _ := strconv.Atoi(c.Query("after"))
	after = max(0, after)
	const pageSize = 24
	images, err := service.Forum.PendingForumImages(after, pageSize+1)
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

func AdminForumImageReviewPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		if c.Get("X-Admin-Image-Queue") == "true" {
			return adminAPIError(c, err)
		}
		return adminAuthError(c, err)
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
	if service.Forum == nil {
		return serveErrorPage(c, fiber.StatusServiceUnavailable)
	}
	reviewed, err := service.Forum.ReviewForumImage(id, action == "approve")
	if err != nil {
		log.Printf("Forum image %d review failed: %v", id, err)
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}
	if !reviewed {
		return c.Status(fiber.StatusConflict).SendString("This image has already been reviewed.")
	}
	if c.Get("X-Admin-Image-Queue") == "true" {
		return c.SendStatus(fiber.StatusNoContent)
	}
	after, _ := strconv.Atoi(c.FormValue("after"))
	return c.Redirect().To(adminForumImagesPath(after))
}
