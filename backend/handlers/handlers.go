package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/kv"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/service"
)

type FeedPost struct {
	ID         int
	Username   string
	UserID     int
	Content    string
	FeedType   string
	TimeAgo    string
	FullDate   string
	Reactions  int
	HasReacted bool
	Images     []int           `json:"images,omitempty"`
	Tag        *models.ClanTag `json:"tag,omitempty"`
}

type wsMessage struct {
	Type      string          `json:"type"`
	ID        int             `json:"id"`
	Username  string          `json:"username"`
	UserID    int             `json:"user_id"`
	Content   string          `json:"content"`
	TimeAgo   string          `json:"time_ago"`
	FullDate  string          `json:"full_date"`
	Reactions int             `json:"reactions"`
	FeedType  string          `json:"feed_type"`
	Images    []int           `json:"images,omitempty"`
	Tag       *models.ClanTag `json:"tag,omitempty"`
}

type WSClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type clientInfo struct {
	client   *WSClient
	username string
}

var (
	feedClients = make(map[*WSClient]string)
	feedHubMu   sync.Mutex
)

func getFeedClientsSnapshot() []clientInfo {
	feedHubMu.Lock()
	defer feedHubMu.Unlock()
	clients := make([]clientInfo, 0, len(feedClients))
	for client, uName := range feedClients {
		clients = append(clients, clientInfo{client: client, username: uName})
	}
	return clients
}

func wornClanTag(userID int) *models.ClanTag {
	if service.Clan == nil || userID <= 0 {
		return nil
	}
	clan, err := service.Clan.WornClan(userID)
	if err != nil || clan == nil {
		return nil
	}
	return clan.TagInfo()
}

func enrichCommentTags(comments []*models.FeedComment) {
	for _, c := range comments {
		if c == nil {
			continue
		}
		c.Tag = wornClanTag(c.UserID)
		if len(c.Replies) > 0 {
			enrichCommentTags(c.Replies)
		}
	}
}

func toFeedPosts(dbPosts []*models.FeedPost, feedType string) []FeedPost {
	posts := make([]FeedPost, 0, len(dbPosts))
	for _, dbP := range dbPosts {
		posts = append(posts, FeedPost{
			ID:         dbP.ID,
			Username:   dbP.Username,
			UserID:     dbP.UserID,
			Content:    dbP.Content,
			FeedType:   feedType,
			Images:     dbP.Images,
			TimeAgo:    service.FormatTimeAgo(dbP.CreationDate),
			FullDate:   dbP.CreationDate.Format("January 02, 2006 at 03:04 PM"),
			Reactions:  dbP.Reactions,
			HasReacted: dbP.HasReacted,
			Tag:        wornClanTag(dbP.UserID),
		})
	}
	return posts
}

func sendFeedError(client *WSClient, err error) {
	errPayload, _ := json.Marshal(wsMessage{
		Type:    "error",
		Content: err.Error(),
	})
	_ = client.Write(websocket.TextMessage, errPayload)
}

// feedEvent is one feed WebSocket message fanned out to every platform
// instance over Redis pub/sub (channel feedEventsChannel).
type feedEvent struct {
	Data     json.RawMessage `json:"data"`
	Friends  bool            `json:"friends,omitempty"`
	AuthorID int             `json:"author_id,omitempty"`
	Author   string          `json:"author,omitempty"`
}

const feedEventsChannel = "feed:events"

// publishFeedEvent sends ev to every instance. Without Redis (development,
// single instance) it delivers locally; when Redis is required, a publish
// failure is logged rather than silently delivered to this instance only.
func publishFeedEvent(ev feedEvent) {
	if kv.Client() == nil && !kv.Required() {
		deliverFeedEvent(ev)
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	if err := kv.Publish(ctx, feedEventsChannel, payload); err != nil {
		log.Printf("feed publish failed: %v", err)
	}
}

func deliverFeedEvent(ev feedEvent) {
	for _, item := range getFeedClientsSnapshot() {
		if ev.Friends && item.username != ev.Author {
			if service.User == nil || service.Friend == nil {
				continue
			}
			recipient, err := service.User.GetUserByUsername(item.username)
			if err != nil || recipient == nil {
				continue
			}
			status, _ := service.Friend.GetFriendStatus(recipient.ID, ev.AuthorID)
			if status != "friends" {
				continue
			}
		}
		_ = item.client.Write(websocket.TextMessage, ev.Data)
	}
}

// StartFeedFanout subscribes this instance to feed events published by any
// instance. It reports false when Redis is not connected.
func StartFeedFanout(ctx context.Context) bool {
	return kv.Subscribe(ctx, feedEventsChannel, func(payload []byte) {
		var ev feedEvent
		if err := json.Unmarshal(payload, &ev); err != nil || len(ev.Data) == 0 {
			return
		}
		deliverFeedEvent(ev)
	})
}

func Home(c fiber.Ctx) error {
	var count int
	var games []*models.Game
	if service.User != nil {
		count, _ = service.User.GetUserCount()
	}
	if service.Game != nil {
		var err error
		games, err = service.Game.GetPopularGames(12)
		if err != nil {
			log.Printf("VERTEXIA DB Error: Failed to fetch popular games: %v", err)
		}
	}

	username := GetActiveUser(c)
	var greeting string
	var friends []*models.User

	if username != "" {
		greetings := []string{
			"Hello, %s!",
			"G'day, %s!",
			"Welcome back, %s!",
			"Howdy, %s!",
			"What's cracking, %s!",
			"Salutations, %s!",
			"Yo, %s!",
			"Greetings, %s!",
			"Hey there, %s!",
			"Good to see you, %s!",
			"Hi, %s!",
			"Sup, %s!",
			"Ahoy, %s!",
			"What's up, %s!",
			"Aloha, %s!",
			"Nice to see you, %s!",
			"What's new, %s!",
			"Long time no see, %s!",
			"Look who it is, %s!",
			"Welcome, %s!",
		}
		idx := time.Now().UnixNano() % int64(len(greetings))
		greeting = fmt.Sprintf(greetings[idx], username)

		if service.User != nil {
			friends, _ = service.User.GetRecentUsers(username, 10)
		}
	}

	var currentUserID int
	if username != "" && service.User != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			currentUserID = u.ID
		}
	}

	var currentFeed []FeedPost
	if service.Feed != nil {
		dbPosts, err := service.Feed.GetRecentFeed(10, currentUserID, "worldwide")
		if err == nil {
			currentFeed = toFeedPosts(dbPosts, "worldwide")
		}
	}

	return Render(c, "pages/home", fiber.Map{
		"Title":           "VERTEXIA",
		"UserCount":       count,
		"Games":           games,
		"Greeting":        greeting,
		"Friends":         friends,
		"Feed":            currentFeed,
		"FocusedFeedID":   0,
		"FocusedFeedType": "worldwide",
	}, "layouts/main")
}

func GetFeedPaginated(c fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	offsetStr := c.Query("offset", "0")
	feedType := c.Query("type", "worldwide")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	username := GetActiveUser(c)
	var currentUserID int
	if username != "" && service.User != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			currentUserID = u.ID
		}
	}

	if service.Feed == nil {
		return c.JSON([]FeedPost{})
	}

	dbPosts, err := service.Feed.GetRecentFeedPaginated(limit, offset, currentUserID, feedType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(toFeedPosts(dbPosts, feedTypeOrDefault(feedType)))
}

func GetFeedSingle(c fiber.Ctx) error {
	feedID, err := strconv.Atoi(c.Params("id"))
	if err != nil || feedID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid feed ID"})
	}
	feedType := c.Query("type", "")
	username := GetActiveUser(c)
	var currentUserID int
	if username != "" && service.User != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			currentUserID = u.ID
		}
	}
	if service.Feed == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Post not found"})
	}
	post, resolvedType := service.Feed.GetSinglePost(feedID, currentUserID, feedType)
	if post == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Post not found"})
	}
	single := toFeedPosts([]*models.FeedPost{post}, resolvedType)
	if len(single) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Post not found"})
	}
	return c.JSON(fiber.Map{
		"id":          single[0].ID,
		"username":    single[0].Username,
		"user_id":     single[0].UserID,
		"content":     single[0].Content,
		"images":      single[0].Images,
		"time_ago":    single[0].TimeAgo,
		"full_date":   single[0].FullDate,
		"reactions":   single[0].Reactions,
		"has_reacted": single[0].HasReacted,
		"tag":         single[0].Tag,
		"feed_type":   resolvedType,
	})
}

func FeedSinglePage(c fiber.Ctx) error {
	feedID, err := strconv.Atoi(c.Params("id"))
	if err != nil || feedID <= 0 {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	var count int
	var games []*models.Game
	if service.User != nil {
		count, _ = service.User.GetUserCount()
	}
	if service.Game != nil {
		var gerr error
		games, gerr = service.Game.GetPopularGames(12)
		if gerr != nil {
			log.Printf("VERTEXIA DB Error: Failed to fetch popular games: %v", gerr)
		}
	}

	username := GetActiveUser(c)
	var greeting string
	var friends []*models.User

	if username != "" {
		greetings := []string{
			"Hello, %s!",
			"G'day, %s!",
			"Welcome back, %s!",
			"Howdy, %s!",
			"What's cracking, %s!",
			"Salutations, %s!",
			"Yo, %s!",
			"Greetings, %s!",
			"Hey there, %s!",
			"Good to see you, %s!",
			"Hi, %s!",
			"Sup, %s!",
			"Ahoy, %s!",
			"What's up, %s!",
			"Aloha, %s!",
			"Nice to see you, %s!",
			"What's new, %s!",
			"Long time no see, %s!",
			"Look who it is, %s!",
			"Welcome, %s!",
		}
		idx := time.Now().UnixNano() % int64(len(greetings))
		greeting = fmt.Sprintf(greetings[idx], username)

		if service.User != nil {
			friends, _ = service.User.GetRecentUsers(username, 10)
		}
	}

	var currentUserID int
	if username != "" && service.User != nil {
		u, _ := service.User.GetUserByUsername(username)
		if u != nil {
			currentUserID = u.ID
		}
	}

	if service.Feed == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	post, resolvedType := service.Feed.GetSinglePost(feedID, currentUserID, c.Query("type"))
	if post == nil {
		return serveErrorPage(c, fiber.StatusNotFound)
	}

	var currentFeed []FeedPost
	if dbPosts, err := service.Feed.GetRecentFeed(10, currentUserID, "worldwide"); err == nil {
		currentFeed = toFeedPosts(dbPosts, "worldwide")
	}
	focused := toFeedPosts([]*models.FeedPost{post}, resolvedType)
	found := false
	for _, p := range currentFeed {
		if p.ID == feedID && p.FeedType == resolvedType {
			found = true
			break
		}
	}
	if !found && len(focused) > 0 {
		currentFeed = append(focused, currentFeed...)
	}

	return Render(c, "pages/home", fiber.Map{
		"Title":           fmt.Sprintf("@%s - VERTEXIA", post.Username),
		"UserCount":       count,
		"Games":           games,
		"Greeting":        greeting,
		"Friends":         friends,
		"Feed":            currentFeed,
		"FocusedFeedID":   feedID,
		"FocusedFeedType": resolvedType,
	}, "layouts/main")
}

func GetFeedCommentsHandler(c fiber.Ctx) error {
	feedID, err := strconv.Atoi(c.Query("feed_id"))
	if err != nil || feedID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid feed_id"})
	}
	feedType := c.Query("feed_type", "worldwide")
	username := GetActiveUser(c)

	if service.Feed == nil {
		return c.JSON([]*models.FeedComment{})
	}

	comments, err := service.Feed.GetFeedComments(feedID, feedType, username)
	if errors.Is(err, service.ErrFeedPostNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Post not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	enrichCommentTags(comments)

	return c.JSON(comments)
}

func PostFeed(c fiber.Ctx) error {
	username := GetActiveUser(c)
	if username == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	jsonResponse := c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON
	fail := func(status int, message string) error {
		if jsonResponse {
			return c.Status(status).JSON(fiber.Map{"error": message})
		}
		return c.Status(status).SendString(message)
	}

	mediaType, _, _ := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	var images []models.ForumImageUpload
	if mediaType == "multipart/form-data" {
		if !csrfValid(c) {
			return fail(fiber.StatusForbidden, "Invalid CSRF token")
		}
		var err error
		images, err = forumImageUploads(c)
		if err != nil {
			return fail(fiber.StatusBadRequest, err.Error())
		}
	}
	if service.Feed == nil {
		return fail(fiber.StatusServiceUnavailable, "Feed is unavailable")
	}

	feedType := feedTypeOrDefault(c.FormValue("feed_type"))
	created, err := service.Feed.PostToFeed(username, c.FormValue("content"), feedType, images...)
	if err != nil {
		if !jsonResponse && mediaType != "multipart/form-data" {
			return c.Redirect().To("/")
		}
		return fail(fiber.StatusBadRequest, err.Error())
	}

	post := BroadcastFeedPost(created, feedType)
	if jsonResponse {
		return c.Status(fiber.StatusCreated).JSON(post)
	}
	return c.Redirect().To("/")
}

const (
	feedWSMaxMessageBytes = 32 << 10
	feedWSPongWait        = 90 * time.Second
	feedWSWriteWait       = 10 * time.Second
	feedWSMaxPerSecond    = 8
	feedWSMaxViolations   = 10
)

func feedTypeOrDefault(raw string) string {
	if raw == "friends" {
		return "friends"
	}
	return "worldwide"
}

func FeedWS(c *websocket.Conn) {
	usernameVal := c.Locals("username")
	username, ok := usernameVal.(string)
	if !ok || username == "" {
		_ = c.Close()
		return
	}

	c.SetReadLimit(feedWSMaxMessageBytes)
	_ = c.SetReadDeadline(time.Now().Add(feedWSPongWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(feedWSPongWait))
	})

	client := &WSClient{conn: c}

	feedHubMu.Lock()
	feedClients[client] = username
	feedHubMu.Unlock()

	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.mu.Lock()
				_ = client.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
				client.mu.Unlock()
			case <-stopPing:
				return
			}
		}
	}()

	defer func() {
		feedHubMu.Lock()
		delete(feedClients, client)
		feedHubMu.Unlock()
		_ = c.Close()
	}()

	type WSClientMessage struct {
		Type     string `json:"type"`
		Content  string `json:"content"`
		FID      int    `json:"fid"`
		CID      int    `json:"cid"`
		FeedID   int    `json:"feed_id"`
		ParentID *int   `json:"parent_id"`
		FeedType string `json:"feed_type"`
	}

	windowStart := time.Now()
	messagesInWindow := 0
	violations := 0

	for {
		mt, msg, err := c.ReadMessage()
		if err != nil {
			break
		}
		if mt == websocket.TextMessage {
			if time.Since(windowStart) >= time.Second {
				windowStart = time.Now()
				messagesInWindow = 0
			}
			messagesInWindow++
			if messagesInWindow > feedWSMaxPerSecond {
				violations++
				sendFeedError(client, errors.New("You're sending messages too fast. Please slow down."))
				if violations >= feedWSMaxViolations {
					break
				}
				continue
			}

			var clientMsg WSClientMessage
			if err := json.Unmarshal(msg, &clientMsg); err == nil {
				feedType := feedTypeOrDefault(clientMsg.FeedType)

				switch clientMsg.Type {
				case "react":
					if clientMsg.FID > 0 && service.Feed != nil {
						totalReactions, hasReacted, err := service.Feed.ToggleReaction(username, clientMsg.FID, feedType)
						if err == nil {
							post, _ := service.Feed.VisiblePost(username, clientMsg.FID, feedType)
							if audience, ok := feedAudience(post, feedType); ok {
								BroadcastReactionUpdate(clientMsg.FID, totalReactions, feedType, username, hasReacted, audience)
							}
						} else {
							sendFeedError(client, err)
						}
					}

				case "creact":
					if clientMsg.CID > 0 && service.Feed != nil {
						totalReactions, hasReacted, err := service.Feed.ToggleCommentReaction(username, clientMsg.CID)
						if err == nil {
							post, realType, _ := service.Feed.CommentPost(username, clientMsg.CID)
							if audience, ok := feedAudience(post, realType); ok {
								BroadcastCommentReactionUpdate(clientMsg.CID, totalReactions, realType, username, hasReacted, audience)
							}
						} else {
							sendFeedError(client, err)
						}
					}

				case "comment":
					if clientMsg.FeedID > 0 && service.Feed != nil {
						comment, err := service.Feed.PostComment(username, clientMsg.FeedID, clientMsg.ParentID, feedType, clientMsg.Content)
						if err != nil {
							sendFeedError(client, err)
							continue
						}
						comment.Tag = wornClanTag(comment.UserID)
						post, _ := service.Feed.VisiblePost(username, clientMsg.FeedID, feedType)
						if audience, ok := feedAudience(post, feedType); ok {
							BroadcastCommentPost(comment, audience)
						}
					}

				default:
					if service.Feed != nil {
						post, err := service.Feed.PostToFeed(username, clientMsg.Content, feedType)
						if err != nil {
							sendFeedError(client, err)
							continue
						}

						if post.ID > 0 {
							BroadcastFeedPost(post, feedType)
						}
					}
				}
			}
		}
	}
}

func (ws *WSClient) Write(messageType int, data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if err := ws.conn.SetWriteDeadline(time.Now().Add(feedWSWriteWait)); err != nil {
		return err
	}
	return ws.conn.WriteMessage(messageType, data)
}

func BroadcastFeedPost(post *models.FeedPost, feedType string) wsMessage {
	msg := wsMessage{
		Type:      "new_post",
		ID:        post.ID,
		Username:  post.Username,
		UserID:    post.UserID,
		Content:   post.Content,
		TimeAgo:   "Just now",
		FullDate:  time.Now().Format("January 02, 2006 at 03:04 PM"),
		Reactions: 0,
		FeedType:  feedType,
		Images:    post.Images,
		Tag:       wornClanTag(post.UserID),
	}
	if post.Content == "" && len(post.Images) == 0 {
		return msg
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return msg
	}

	publishFeedEvent(feedEvent{Data: data, Friends: feedType == "friends", AuthorID: post.UserID, Author: post.Username})
	return msg
}

// feedAudience scopes an event about a post: friends-feed activity only goes
// to the post author and their friends. ok is false when the post cannot be
// resolved, in which case nothing is broadcast.
func feedAudience(post *models.FeedPost, feedType string) (feedEvent, bool) {
	if post == nil {
		return feedEvent{}, false
	}
	if feedType != "friends" {
		return feedEvent{}, true
	}
	return feedEvent{Friends: true, AuthorID: post.UserID, Author: post.Username}, true
}

func BroadcastCommentPost(comment *models.FeedComment, audience feedEvent) {
	msg := struct {
		Type    string              `json:"type"`
		Comment *models.FeedComment `json:"comment"`
	}{
		Type:    "new_comment",
		Comment: comment,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	audience.Data = data
	publishFeedEvent(audience)
}

func BroadcastReactionUpdate(fid int, reactions int, feedType, actor string, hasReacted bool, audience feedEvent) {
	msg := struct {
		Type       string `json:"type"`
		FID        int    `json:"fid"`
		Reactions  int    `json:"reactions"`
		HasReacted bool   `json:"has_reacted"`
		ReactedBy  string `json:"reacted_by"`
		FeedType   string `json:"feed_type"`
	}{
		Type:       "reaction_update",
		FID:        fid,
		Reactions:  reactions,
		HasReacted: hasReacted,
		ReactedBy:  actor,
		FeedType:   feedType,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	audience.Data = data
	publishFeedEvent(audience)
}

func BroadcastCommentReactionUpdate(cid int, reactions int, feedType, actor string, hasReacted bool, audience feedEvent) {
	msg := struct {
		Type       string `json:"type"`
		CID        int    `json:"cid"`
		Reactions  int    `json:"reactions"`
		HasReacted bool   `json:"has_reacted"`
		ReactedBy  string `json:"reacted_by"`
		FeedType   string `json:"feed_type"`
	}{
		Type:       "comment_reaction_update",
		CID:        cid,
		Reactions:  reactions,
		HasReacted: hasReacted,
		ReactedBy:  actor,
		FeedType:   feedType,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	audience.Data = data
	publishFeedEvent(audience)
}
