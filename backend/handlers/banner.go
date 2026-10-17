package handlers

import (
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

func bannerRemoveImageFile(id int) {
	_ = os.Remove(models.BannerImagePath(id))
}

func SettingsBannerPost(c fiber.Ctx) error {
	user, err := getSettingsUser(c)
	if err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}

	bannerType := models.NormalizeBannerType(c.FormValue("type"))
	colorA := models.NormalizeBannerHex(c.FormValue("color_a"), "7423CB")
	colorB := models.NormalizeBannerHex(c.FormValue("color_b"), "912ADF")
	angle := models.NormalizeBannerAngle(c.FormValue("angle"), 135)

	if bannerType == models.BannerTypeImage {
		header, ferr := c.FormFile("image")
		if ferr != nil || header == nil || header.Size <= 0 {
			return handleSettingsResponse(c, fiber.NewError(fiber.StatusBadRequest, "Please choose an image to upload"), "", nil)
		}
		if verr := checkGameUploadImage(header, "Banner"); verr != nil {
			return handleSettingsResponse(c, verr, "", nil)
		}
	}

	if service.Banner == nil {
		return handleSettingsResponse(c, fiber.NewError(fiber.StatusInternalServerError, "Banner service unavailable"), "", nil)
	}

	if bannerType != models.BannerTypeImage {
		var replacedImageIDs []int
		if live, _ := service.Banner.GetApprovedBanner(user.ID); live != nil && live.Type == models.BannerTypeImage {
			replacedImageIDs = append(replacedImageIDs, live.ID)
		}
		if pending, _ := service.Banner.GetPendingBanner(user.ID); pending != nil && pending.Type == models.BannerTypeImage {
			replacedImageIDs = append(replacedImageIDs, pending.ID)
		}
		bannerID, err := service.Banner.ApplyBanner(user.ID, bannerType, colorA, colorB, angle)
		if err != nil {
			return handleSettingsResponse(c, err, "", nil)
		}
		for _, id := range replacedImageIDs {
			bannerRemoveImageFile(id)
		}
		applied, _ := service.Banner.GetApprovedBanner(user.ID)
		style := ""
		if applied != nil {
			style = string(applied.Style())
		}
		return handleSettingsResponse(c, nil, "Banner updated", fiber.Map{
			"banner_id":     bannerID,
			"banner_type":   bannerType,
			"banner_style":  style,
			"banner_status": models.BannerStatusApproved,
		})
	}

	bannerID, err := service.Banner.SubmitBanner(user.ID, bannerType, colorA, colorB, angle)
	if err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}

	if bannerType == models.BannerTypeImage {
		header, ferr := c.FormFile("image")
		if ferr != nil || header == nil {
			_ = service.Banner.DeleteBanner(bannerID)
			return handleSettingsResponse(c, fiber.NewError(fiber.StatusBadRequest, "Please choose an image to upload"), "", nil)
		}
		src, ferr := header.Open()
		if ferr != nil {
			_ = service.Banner.DeleteBanner(bannerID)
			return handleSettingsResponse(c, fiber.NewError(fiber.StatusBadRequest, "Could not read banner"), "", nil)
		}
		defer src.Close()
		if merr := os.MkdirAll(models.BannerImageUploadDir, 0755); merr != nil {
			_ = service.Banner.DeleteBanner(bannerID)
			return handleSettingsResponse(c, fiber.NewError(fiber.StatusInternalServerError, "Could not save upload"), "", nil)
		}
		if serr := saveShopItemImage(src, models.BannerImagePath(bannerID)); serr != nil {
			_ = service.Banner.DeleteBanner(bannerID)
			bannerRemoveImageFile(bannerID)
			if serr.Error() == "invalid image file" {
				return handleSettingsResponse(c, fiber.NewError(fiber.StatusBadRequest, "Banner must be a PNG or JPEG image"), "", nil)
			}
			return handleSettingsResponse(c, fiber.NewError(fiber.StatusBadRequest, "Could not save upload"), "", nil)
		}
	}

	pending, _ := service.Banner.GetPendingBanner(user.ID)
	style := ""
	if pending != nil {
		style = string(pending.Style())
	}
	return handleSettingsResponse(c, nil, "Banner submitted and awaiting admin approval", fiber.Map{
		"banner_id":     bannerID,
		"banner_type":   bannerType,
		"banner_style":  style,
		"banner_status": models.BannerStatusPending,
	})
}

func SettingsBannerEffectPost(c fiber.Ctx) error {
	user, err := getSettingsUser(c)
	if err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}

	effect := models.NormalizeBannerEffect(c.FormValue("effect"))
	err = service.User.UpdateBannerEffect(user.ID, effect)
	return handleSettingsResponse(c, err, "Banner effect updated", fiber.Map{"effect": effect})
}

func SettingsBannerTextBgPost(c fiber.Ctx) error {
	user, err := getSettingsUser(c)
	if err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}

	raw := strings.ToLower(strings.TrimSpace(c.FormValue("enabled")))
	enabled := raw == "1" || raw == "true" || raw == "on"
	color := c.FormValue("color")
	if strings.TrimSpace(color) == "" {
		color = user.BannerTextBgColor
	}
	border := c.FormValue("border")
	if strings.TrimSpace(border) == "" {
		border = user.BannerTextBgBorder
	}
	text := c.FormValue("text")
	if strings.TrimSpace(text) == "" {
		text = user.BannerTextBgText
	}
	opacity := user.BannerTextBgOpacity
	if rawOpacity := strings.TrimSpace(c.FormValue("opacity")); rawOpacity != "" {
		if parsed, err := strconv.Atoi(rawOpacity); err == nil {
			opacity = parsed
		}
	}
	err = service.User.UpdateBannerTextBg(user.ID, enabled, color, border, text, opacity)
	return handleSettingsResponse(c, err, "Header updated", fiber.Map{"enabled": enabled})
}

func SettingsBannerRemovePost(c fiber.Ctx) error {
	user, err := getSettingsUser(c)
	if err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}
	if service.Banner == nil {
		return handleSettingsResponse(c, fiber.NewError(fiber.StatusInternalServerError, "Banner service unavailable"), "", nil)
	}
	var imageIDs []int
	if live, _ := service.Banner.GetApprovedBanner(user.ID); live != nil {
		imageIDs = append(imageIDs, live.ID)
	}
	if pending, _ := service.Banner.GetPendingBanner(user.ID); pending != nil {
		imageIDs = append(imageIDs, pending.ID)
	}
	if err := service.Banner.RemoveBanners(user.ID); err != nil {
		return handleSettingsResponse(c, err, "", nil)
	}
	for _, id := range imageIDs {
		bannerRemoveImageFile(id)
	}
	return handleSettingsResponse(c, nil, "Profile banner removed", nil)
}

func AdminBannerQueueAPI(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAPIError(c, err)
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "12"))

	if service.Banner == nil {
		return c.JSON(fiber.Map{
			"banners":     []*models.Banner{},
			"total":       0,
			"page":        page,
			"limit":       limit,
			"total_pages": 1,
		})
	}

	banners, total, err := service.Banner.GetQueue(page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if banners == nil {
		banners = []*models.Banner{}
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return c.JSON(fiber.Map{
		"banners":     banners,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func AdminBannerReviewPost(c fiber.Ctx) error {
	if _, err := getAdminUser(c); err != nil {
		return adminAuthError(c, err)
	}

	if !csrfValid(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	bannerID, err := strconv.Atoi(c.Params("id"))
	if err != nil || bannerID <= 0 {
		return adminModerationError(c, "Invalid banner ID")
	}

	if service.Banner == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Banner service unavailable")
	}

	action := c.Params("action")
	status, err := service.Banner.ReviewBanner(bannerID, action)
	if err != nil {
		return adminModerationError(c, err.Error())
	}
	if status == models.BannerStatusDeclined {
		bannerRemoveImageFile(bannerID)
	}

	if c.Get("Accept") == "application/json" || c.Get("X-Requested-With") == "XMLHttpRequest" || strings.HasPrefix(c.Path(), "/api/") {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      bannerID,
			"state":   status,
			"action":  action,
		})
	}

	return c.Redirect().To("/admin")
}
