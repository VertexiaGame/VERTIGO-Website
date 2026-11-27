package handlers

import (
	"errors"
	"fmt"
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
		return internalErrorPage(c, errors.New("Forum service unavailable"))
	}

	boards, err := service.Forum.GetBoards()
	if err != nil {
		return internalErrorPage(c, fmt.Errorf("VERTEXIA DB Error: Failed to fetch forum boards: %v", err))
	}
	if boards == nil {
		boards = []*models.ForumBoard{}
	}

	return Render(c, "pages/forum", fiber.Map{
		"Title":  "Forum - VERTEXIA",
		"Boards": boards,
	})
}

func ForumMyThreadsGet(c fiber.Ctx) error {
	user, err := clanViewer(c)
	if err != nil || user == nil || user.ID <= 0 {
		return c.Redirect().To("/login?next=" + url.QueryEscape("/forum/mine"))
	}
	if service.Forum == nil {
		return internalErrorPage(c, errors.New("Forum service unavailable"))
	}
	total, err := service.Forum.CountThreadsByUser(user.ID)
	if err != nil {
		return internalErrorPage(c, fmt.Errorf("Forum user thread count failed: %v", err))
	}
	page, pages := forumPage(c.Query("page"), total)
	threads, err := service.Forum.GetThreadsByUser(user.ID, forumPageSize, (page-1)*forumPageSize)
	if err != nil {
		return internalErrorPage(c, fmt.Errorf("Forum user threads failed: %v", err))
	}
	enrichForumThreadTags(threads)
	return Render(c, "pages/forum_my_threads", fiber.Map{
		"Title": "My Threads - Forum - VERTEXIA", "Threads": threads, "ThreadCount": total,
		"Page": page, "Pages": pages, "PrevPage": page - 1, "NextPage": page + 1,
	})
}

func ForumBoardGet(c fiber.Ctx) error {
	if service.Forum == nil {
		return internalErrorPage(c, errors.New("Forum service unavailable"))
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
		return internalErrorPage(c, fmt.Errorf("VERTEXIA DB Error: Failed to fetch forum board %d: %v", boardID, err))
	}
	if board == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	sortBy := normalizeForumSort(c.Query("sort"))
	total, err := service.Forum.CountThreads(board.ID)
	if err != nil {
		return internalErrorPage(c, fmt.Errorf("Forum thread count failed: %v", err))
	}
	page, pages := forumPage(c.Query("page"), total)
	threads, err := service.Forum.GetThreadsByBoard(board.ID, sortBy, forumPageSize, (page-1)*forumPageSize)
	if err != nil {
		return internalErrorPage(c, fmt.Errorf("VERTEXIA DB Error: Failed to fetch threads for board %d: %v", board.ID, err))
	}
	enrichForumThreadTags(threads)

	flashSuccess := ""
	if c.Query("created") == "1" {
		flashSuccess = "Your thread is live."
	}
	flashError := strings.TrimSpace(c.Query("error"))

	return Render(c, "pages/forum_board", fiber.Map{
		"Title":       board.Name + " - Forum - VERTEXIA",
		"Board":       board,
		"Threads":     threads,
		"ThreadCount": total,
		"Page":        page, "Pages": pages, "PrevPage": page - 1, "NextPage": page + 1,
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

	images, err := forumImageUploads(c)
	if err != nil {
		return clanActionError(c, err, boardPath)
	}
	title := c.FormValue("title")
	body := c.FormValue("body")

	created, err := service.Forum.CreateThread(user, boardID, title, body, images...)
	if err != nil {
		if errors.Is(err, service.ErrForumBoardNotFound) {
			return clanActionError(c, err, "/forum")
		}
		if wantsAssetJSON(c) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect().To(boardPath + "?error=" + url.QueryEscape(err.Error()))
	}

	path := created.Path()
	if hasNewImages(images) {
		path += "?images=pending"
	}
	if wantsAssetJSON(c) {
		return c.JSON(fiber.Map{
			"success": true,
			"id":      created.ID,
			"path":    path,
		})
	}

	return c.Redirect().To(path)
}

const forumPageSize = 50

func forumPage(raw string, total int) (int, int) {
	pages := (total + forumPageSize - 1) / forumPageSize
	if pages < 1 {
		pages = 1
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	return page, pages
}
func ForumThreadGet(c fiber.Ctx) error {
	if service.Forum == nil {
		return serveErrorPage(c, fiber.StatusServiceUnavailable)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	thread, err := service.Forum.GetThread(id)
	if errors.Is(err, service.ErrForumThreadNotFound) {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	if err != nil {
		return internalErrorPage(c, err)
	}
	board, err := service.Forum.GetBoardByID(thread.BoardID)
	if err != nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	page, pages := forumPage(c.Query("page"), thread.RepliesCount)
	replies, err := service.Forum.GetReplies(id, forumPageSize, (page-1)*forumPageSize)
	if err != nil {
		return internalErrorPage(c, err)
	}
	if err := service.Forum.LoadApprovedImages(thread, replies); err != nil {
		log.Printf("Forum images failed for thread %d: %v", id, err)
	}
	attachForumQuotes(thread, replies, page)
	enrichForumThreadTags([]*models.ForumThread{thread})
	if service.Clan != nil {
		for _, reply := range replies {
			reply.Tag = wornClanTag(reply.UserID)
		}
	}
	return Render(c, "pages/forum_thread", fiber.Map{"Title": thread.Title + " - Forum - VERTEXIA", "Board": board, "Thread": thread, "Replies": replies, "Page": page, "Pages": pages, "PrevPage": page - 1, "NextPage": page + 1, "FlashError": strings.TrimSpace(c.Query("error")), "ImagesPending": c.Query("images") == "pending"})
}
func ForumReplyCreatePost(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}
	path := "/forum/t/" + strconv.Itoa(id)
	user, err := clanViewer(c)
	if err != nil {
		return clanActionError(c, err, path)
	}
	if err := clanCheckCSRF(c); err != nil {
		return clanActionError(c, err, path)
	}
	if service.Forum == nil {
		return clanActionError(c, errors.New("forum service unavailable"), path)
	}
	images, err := forumImageUploads(c)
	if err != nil {
		return clanActionError(c, err, path)
	}
	quoteID, err := forumQuoteID(c.FormValue("quote_id"))
	if err != nil {
		return clanActionError(c, err, path)
	}
	replyID, err := service.Forum.CreateQuotedReply(user, id, c.FormValue("body"), quoteID, images...)
	if err != nil {
		return clanActionError(c, err, path)
	}
	params := url.Values{}
	if hasNewImages(images) {
		params.Set("images", "pending")
	}
	thread, err := service.Forum.GetThread(id)
	if err == nil {
		_, pages := forumPage("1", thread.RepliesCount)
		params.Set("page", strconv.Itoa(pages))
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	path += "#reply-" + strconv.Itoa(replyID)
	return clanActionSucceed(c, path, fiber.Map{"id": replyID, "path": path})
}

func forumQuoteID(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id < 0 {
		return nil, service.ErrForumQuoteNotFound
	}
	return &id, nil
}

func attachForumQuotes(thread *models.ForumThread, replies []*models.ForumReply, page int) {
	for _, reply := range replies {
		q := reply.Quote
		if q == nil || q.Missing {
			continue
		}
		if q.IsOriginal() {
			q.UserID = thread.UserID
			q.AuthorUsername = thread.AuthorUsername
			q.AuthorDisplayName = thread.AuthorDisplayName
			q.Body = thread.Body
			q.Href = "#thread"
			continue
		}
		quotePage := q.Position/forumPageSize + 1
		q.Href = "#" + q.Anchor()
		if quotePage != page {
			q.Href = thread.Path() + "?page=" + strconv.Itoa(quotePage) + "#" + q.Anchor()
		}
	}
}
