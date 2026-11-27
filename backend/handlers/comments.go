package handlers

import (
	"errors"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

var errCommentTargetMissing = errors.New("not found")

func commentTarget(kind string, id, viewerID int) (service.CommentTarget, error) {
	target := service.CommentTarget{Kind: kind, ID: id}
	if id <= 0 {
		return target, errCommentTargetMissing
	}
	switch kind {
	case models.CommentTargetShop:
		if service.Shop == nil {
			return target, errCommentTargetMissing
		}
		item, err := service.Shop.GetShopItemByID(id)
		if err != nil || item == nil {
			return target, errCommentTargetMissing
		}
		if item.CreatorType == "user" {
			target.OwnerID = item.CID
		}
		if item.ApprovalState != models.ShopApprovalApproved && (viewerID == 0 || viewerID != target.OwnerID) {
			return target, errCommentTargetMissing
		}
	case models.CommentTargetGame:
		if service.Game == nil {
			return target, errCommentTargetMissing
		}
		game, err := service.Game.GetGameByID(id)
		if err != nil || game == nil || !game.VisibleTo(viewerID) {
			return target, errCommentTargetMissing
		}
		if game.CreatorID.Valid {
			target.OwnerID = int(game.CreatorID.Int64)
		}
	default:
		return target, errCommentTargetMissing
	}
	return target, nil
}

func commentKindParam(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "shop", "item", "items":
		return models.CommentTargetShop
	case "game", "games":
		return models.CommentTargetGame
	}
	return ""
}

func commentPagePath(target service.CommentTarget) string {
	if target.Kind == models.CommentTargetGame {
		return "/play/" + strconv.Itoa(target.ID) + "?tab=comments"
	}
	return "/shop/item/" + strconv.Itoa(target.ID) + "?tab=comments"
}

func commentViewer(c fiber.Ctx) *models.User {
	username := GetActiveUser(c)
	if username == "" || service.User == nil {
		return nil
	}
	u, err := service.User.GetUserByUsername(username)
	if err != nil {
		return nil
	}
	return u
}

func commentErrorStatus(err error) int {
	switch {
	case errors.Is(err, errClanSession):
		return fiber.StatusUnauthorized
	case errors.Is(err, errCommentTargetMissing), errors.Is(err, service.ErrCommentNotFound):
		return fiber.StatusNotFound
	case errors.Is(err, service.ErrCommentPermission):
		return fiber.StatusForbidden
	case errors.Is(err, service.ErrCommentLength), errors.Is(err, service.ErrCommentTarget):
		return fiber.StatusBadRequest
	case strings.HasPrefix(err.Error(), "whoa, slow down"):
		return fiber.StatusTooManyRequests
	case strings.Contains(err.Error(), "session expired"):
		return fiber.StatusForbidden
	}
	return fiber.StatusInternalServerError
}

func commentActionError(c fiber.Ctx, err error, redirect string) error {
	status := commentErrorStatus(err)
	msg := err.Error()
	jsonResponse := wantsAssetJSON(c)
	if status == fiber.StatusInternalServerError {
		msg = "something went wrong, try again"
		if jsonResponse {
			recordInternalError(c, err)
		} else {
			log.Printf("comment action failed: %v", err)
		}
	}
	if jsonResponse {
		return c.Status(status).JSON(fiber.Map{"error": msg})
	}
	if status == fiber.StatusUnauthorized {
		return c.Redirect().To("/login")
	}
	if status == fiber.StatusNotFound && redirect == "" {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	return c.Redirect().To(redirect + "&comment_error=" + url.QueryEscape(msg))
}

func loadCommentSection(c fiber.Ctx, target service.CommentTarget, viewer *models.User) *models.CommentSection {
	section := &models.CommentSection{
		TargetType: target.Kind,
		TargetID:   target.ID,
		IsLoggedIn: viewer != nil,
		CanPost:    viewer != nil,
		CSRF:       getCSRFToken(c),
		Error:      strings.TrimSpace(c.Query("comment_error")),
	}
	if service.Comment == nil {
		section.CanPost = false
		return section
	}
	comments, hasMore, err := service.Comment.List(viewer, target, 0)
	if err != nil {
		log.Printf("load %s %d comments: %v", target.Kind, target.ID, err)
		section.Error = "Comments could not be loaded."
		return section
	}
	section.Comments, section.HasMore = comments, hasMore
	return section
}

func pageCommentSection(c fiber.Ctx, kind string, id int) *models.CommentSection {
	viewer := commentViewer(c)
	viewerID := 0
	if viewer != nil {
		viewerID = viewer.ID
	}
	target, err := commentTarget(kind, id, viewerID)
	if err != nil {
		return &models.CommentSection{TargetType: kind, TargetID: id, IsLoggedIn: viewer != nil}
	}
	return loadCommentSection(c, target, viewer)
}

func ShopItemCommentPost(c fiber.Ctx) error {
	return commentPost(c, models.CommentTargetShop)
}

func GameCommentPost(c fiber.Ctx) error {
	return commentPost(c, models.CommentTargetGame)
}

func commentPost(c fiber.Ctx, kind string) error {
	id, _ := strconv.Atoi(c.Params("id"))
	viewer := commentViewer(c)
	viewerID := 0
	if viewer != nil {
		viewerID = viewer.ID
	}
	target, err := commentTarget(kind, id, viewerID)
	if err != nil {
		return commentActionError(c, err, "")
	}
	path := commentPagePath(target)
	if viewer == nil {
		return commentActionError(c, errClanSession, path)
	}
	if err := clanCheckCSRF(c); err != nil {
		return commentActionError(c, err, path)
	}
	if service.Comment == nil {
		return commentActionError(c, errors.New("comments are unavailable"), path)
	}
	parentID, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("parent_id")))
	comment, err := service.Comment.Post(viewer, target, max(parentID, 0), c.FormValue("body"))
	if err != nil {
		return commentActionError(c, err, path)
	}
	if wantsAssetJSON(c) {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "comment": comment})
	}
	return c.Redirect().To(path + "#comment-" + strconv.Itoa(comment.ID))
}

func CommentRemovePost(c fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	viewer := commentViewer(c)
	if viewer == nil {
		return commentActionError(c, errClanSession, "/")
	}
	if service.Comment == nil || id <= 0 {
		return commentActionError(c, service.ErrCommentNotFound, "")
	}
	comment, err := service.Comment.Get(id)
	if err != nil {
		return commentActionError(c, err, "")
	}
	target, err := commentTarget(comment.TargetType, comment.TargetID, viewer.ID)
	if err != nil {
		return commentActionError(c, err, "")
	}
	path := commentPagePath(target)
	if err := clanCheckCSRF(c); err != nil {
		return commentActionError(c, err, path)
	}
	if err := service.Comment.Remove(viewer, comment, target); err != nil {
		return commentActionError(c, err, path)
	}
	if wantsAssetJSON(c) {
		return c.JSON(fiber.Map{"success": true, "id": comment.ID})
	}
	return c.Redirect().To(path)
}

func CommentsAPI(c fiber.Ctx) error {
	kind := commentKindParam(c.Params("kind"))
	id, _ := strconv.Atoi(c.Params("id"))
	viewer := commentViewer(c)
	viewerID := 0
	if viewer != nil {
		viewerID = viewer.ID
	}
	target, err := commentTarget(kind, id, viewerID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
	if service.Comment == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "comments are unavailable"})
	}
	before, _ := strconv.Atoi(c.Query("before"))
	comments, hasMore, err := service.Comment.List(viewer, target, max(before, 0))
	if err != nil {
		recordInternalError(c, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "comments could not be loaded"})
	}
	if comments == nil {
		comments = []*models.Comment{}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"comments": comments, "has_more": hasMore})
}
