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

func normalizeForumSort(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "replies", "top":
		return "replies"
	case "oldest":
		return "oldest"
	default:
		return "newest"
	}
}

func enrichForumThreadTags(threads []*models.ForumThread) {
	if len(threads) == 0 || service.Clan == nil {
		return
	}
	cache := make(map[int]*models.ClanTag)
	for _, t := range threads {
		if t == nil || t.UserID <= 0 {
			continue
		}
		if tag, seen := cache[t.UserID]; seen {
			t.Tag = tag
			continue
		}
		tag := wornClanTag(t.UserID)
		cache[t.UserID] = tag
		t.Tag = tag
	}
}

func ForumIndexGet(c fiber.Ctx) error {
	if service.Forum == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	boards, err := service.Forum.GetBoards()
	if err != nil {
		log.Printf("VERTEXIA DB Error: Failed to fetch forum boards: %v", err)
		boards = []*models.ForumBoard{}
	}
	if boards == nil {
		boards = []*models.ForumBoard{}
	}

	return Render(c, "pages/forum", fiber.Map{
		"Title":  "Forum - VERTEXIA",
		"Boards": boards,
	})
}

func ForumBoardGet(c fiber.Ctx) error {
	if service.Forum == nil {
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}

	boardID, err := strconv.Atoi(strings.TrimSpace(c.Params("id")))
	if err != nil || boardID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	board, err := service.Forum.GetBoardByID(boardID)
	if err != nil {
		if errors.Is(err, service.ErrForumBoardNotFound) {
			return serveErrorPage(c, fiber.StatusNotFound)
		}
		log.Printf("VERTEXIA DB Error: Failed to fetch forum board %d: %v", boardID, err)
		return serveErrorPage(c, fiber.StatusInternalServerError)
	}
	if board == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	sortBy := normalizeForumSort(c.Query("sort"))
	threads, err := service.Forum.GetThreadsByBoard(board.ID, sortBy, 50, 0)
	if err != nil {
		log.Printf("VERTEXIA DB Error: Failed to fetch threads for board %d: %v", board.ID, err)
		threads = []*models.ForumThread{}
	}
	enrichForumThreadTags(threads)

	flashSuccess := ""
	if c.Query("created") == "1" {
		flashSuccess = "Your thread is live."
	}
	flashError := strings.TrimSpace(c.Query("error"))

	return Render(c, "pages/forum_board", fiber.Map{
		"Title":        board.Name + " - Forum - VERTEXIA",
		"Board":        board,
		"Threads":      threads,
		"ThreadCount":  len(threads),
		"Sort":         sortBy,
		"FlashSuccess": flashSuccess,
		"FlashError":   flashError,
	})
}

func ForumThreadCreatePost(c fiber.Ctx) error {
	boardID, _ := strconv.Atoi(strings.TrimSpace(c.Params("id")))
	if boardID <= 0 {
		return clanActionError(c, service.ErrForumBoardNotFound, "/forum")
	}

	boardPath := "/forum/s/" + strconv.Itoa(boardID)

	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, boardPath)
	}

	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, boardPath)
	}

	if service.Forum == nil {
		return clanActionError(c, errors.New("forum service unavailable"), boardPath)
	}

	title := c.FormValue("title")
	body := c.FormValue("body")

	created, err := service.Forum.CreateThread(user, boardID, title, body)
	if err != nil {
		if errors.Is(err, service.ErrForumBoardNotFound) {
			return clanActionError(c, err, "/forum")
		}
		if wantsAssetJSON(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To(boardPath + "?error=" + url.QueryEscape(err.Error()))
	}

	if wantsAssetJSON(c) {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      created.ID,
			"path":    boardPath + "?created=1",
		})
	}

	return c.Redirect().To(boardPath + "?created=1")
}
