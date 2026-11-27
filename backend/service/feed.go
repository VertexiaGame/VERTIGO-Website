package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type FeedService struct {
	feedRepo *repository.FeedRepository
	userRepo *repository.UserRepository
	cooldown *CooldownService
}

func NewFeedService(feedRepo *repository.FeedRepository, userRepo *repository.UserRepository, cooldown *CooldownService) *FeedService {
	return &FeedService{
		feedRepo: feedRepo,
		userRepo: userRepo,
		cooldown: cooldown,
	}
}

func FormatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return "Just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins <= 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hrs := int(d.Hours())
		if hrs <= 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hrs)
	}
	if d < 30*24*time.Hour {
		days := int(d.Hours() / 24)
		if days <= 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("Jan 02, 2006")
}

func (s *FeedService) GetRecentFeed(limit, currentUserID int, feedType string) ([]*models.FeedPost, error) {
	if feedType == "friends" {
		return s.feedRepo.GetRecentFriendsFeedPosts(limit, currentUserID)
	}
	return s.feedRepo.GetRecentFeedPosts(limit, currentUserID)
}

func (s *FeedService) GetRecentFeedPaginated(limit, offset, currentUserID int, feedType string) ([]*models.FeedPost, error) {
	if feedType == "friends" {
		return s.feedRepo.GetRecentFriendsFeedPostsPaginated(limit, offset, currentUserID)
	}
	return s.feedRepo.GetRecentFeedPostsPaginated(limit, offset, currentUserID)
}

func (s *FeedService) GetSinglePost(id, currentUserID int, feedType string) (*models.FeedPost, string) {
	if id <= 0 {
		return nil, ""
	}
	if s.feedRepo == nil {
		return nil, ""
	}
	if feedType != "friends" {
		if post, _ := s.feedRepo.GetFeedPostByID(id, currentUserID); post != nil {
			return post, "worldwide"
		}
	}
	if feedType != "worldwide" {
		if post, _ := s.feedRepo.GetFriendsFeedPostByID(id, currentUserID); post != nil {
			return post, "friends"
		}
	}
	return nil, ""
}

var ErrFeedPostNotFound = errors.New("Post not found")

// visiblePost returns the live post when viewerID may see it: anyone for
// worldwide posts, the author and accepted friends for friends posts.
func (s *FeedService) visiblePost(feedID, viewerID int, feedType string) (*models.FeedPost, error) {
	if feedType != "friends" && feedType != "worldwide" {
		return nil, ErrFeedPostNotFound
	}
	post, _ := s.GetSinglePost(feedID, viewerID, feedType)
	if post == nil {
		return nil, ErrFeedPostNotFound
	}
	return post, nil
}

// VisiblePost resolves a post for the named viewer (empty for anonymous).
func (s *FeedService) VisiblePost(username string, feedID int, feedType string) (*models.FeedPost, error) {
	viewerID := 0
	if username != "" {
		if u, _ := s.userRepo.GetByUsername(username); u != nil {
			viewerID = u.ID
		}
	}
	return s.visiblePost(feedID, viewerID, feedType)
}

// CommentPost resolves the post a comment belongs to, if the viewer can see it.
func (s *FeedService) CommentPost(username string, commentID int) (*models.FeedPost, string, error) {
	feedID, feedType, err := s.feedRepo.GetCommentTarget(commentID)
	if err != nil {
		return nil, "", ErrFeedPostNotFound
	}
	post, err := s.VisiblePost(username, feedID, feedType)
	return post, feedType, err
}

func (s *FeedService) PostToFeed(username, content, feedType string, images ...models.ForumImageUpload) (*models.FeedPost, error) {
	if username == "" {
		return nil, errors.New("Unauthorized")
	}

	content = strings.TrimSpace(content)
	if content == "" && len(images) == 0 {
		return nil, errors.New("Content cannot be empty")
	}

	if r := []rune(content); len(r) > 150 {
		content = string(r[:150])
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return nil, errors.New("User not found")
	}

	if s.cooldown != nil {
		key := fmt.Sprintf("feed_post:%d", user.ID)
		if feedType == "friends" {
			key = fmt.Sprintf("friends_feed_post:%d", user.ID)
		}
		if allowed, remaining := s.cooldown.Allow(key, 15*time.Second); !allowed {
			return nil, fmt.Errorf("The feed has a %s cooldown per message.", s.cooldown.FormatRemaining(remaining))
		}
	}

	post, err := s.feedRepo.CreatePost(user.ID, content, feedType, images)
	if err != nil {
		if errors.Is(err, models.ErrSavedImageUnavailable) {
			return nil, err
		}
		log.Printf("Feed post creation failed: %v", err)
		return nil, errors.New("Could not post to the feed. Please try again.")
	}

	post.Username = user.Username
	return post, nil
}

func (s *FeedService) GetImage(id, currentUserID int) (models.ForumImageUpload, error) {
	return s.feedRepo.GetImage(id, currentUserID)
}

func (s *FeedService) GetPendingImage(id int) (models.ForumImageUpload, error) {
	return s.feedRepo.GetPendingImage(id)
}

func (s *FeedService) PendingImages(after, limit int) ([]models.PendingFeedImage, error) {
	return s.feedRepo.PendingImages(after, limit)
}

func (s *FeedService) ReviewImage(id int, approve bool) (bool, error) {
	return s.feedRepo.ReviewImage(id, approve)
}

func (s *FeedService) ToggleReaction(username string, feedID int, feedType string) (int, bool, error) {
	if username == "" || feedID <= 0 {
		return 0, false, errors.New("invalid request")
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return 0, false, errors.New("user not found")
	}

	if _, err := s.visiblePost(feedID, user.ID, feedType); err != nil {
		return 0, false, err
	}

	if s.cooldown != nil {
		key := fmt.Sprintf("reaction:%d:%d:%s", user.ID, feedID, feedType)
		if allowed, remaining := s.cooldown.Allow(key, 2*time.Second); !allowed {
			return 0, false, fmt.Errorf("Please wait %s before reacting again.", s.cooldown.FormatRemaining(remaining))
		}
	}

	if feedType == "friends" {
		hasReacted, err := s.feedRepo.HasUserReactedFriends(user.ID, feedID)
		if err != nil {
			return 0, false, err
		}

		if hasReacted {
			if err := s.feedRepo.RemoveReactionFriends(user.ID, feedID); err != nil {
				return 0, false, err
			}
		} else {
			if err := s.feedRepo.AddReactionFriends(user.ID, feedID); err != nil {
				return 0, false, err
			}
		}

		count, err := s.feedRepo.GetReactionCountFriends(feedID)
		if err != nil {
			return 0, false, err
		}
		return count, !hasReacted, nil
	}

	hasReacted, err := s.feedRepo.HasUserReacted(user.ID, feedID)
	if err != nil {
		return 0, false, err
	}

	if hasReacted {
		if err := s.feedRepo.RemoveReaction(user.ID, feedID); err != nil {
			return 0, false, err
		}
	} else {
		if err := s.feedRepo.AddReaction(user.ID, feedID); err != nil {
			return 0, false, err
		}
	}

	count, err := s.feedRepo.GetReactionCount(feedID)
	if err != nil {
		return 0, false, err
	}
	return count, !hasReacted, nil
}

func (s *FeedService) HasUserReacted(username string, feedID int, feedType string) bool {
	if username == "" || feedID <= 0 {
		return false
	}
	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return false
	}
	if feedType == "friends" {
		hasReacted, _ := s.feedRepo.HasUserReactedFriends(user.ID, feedID)
		return hasReacted
	}
	hasReacted, _ := s.feedRepo.HasUserReacted(user.ID, feedID)
	return hasReacted
}

func (s *FeedService) PostComment(username string, feedID int, parentID *int, feedType, comment string) (*models.FeedComment, error) {
	if username == "" {
		return nil, errors.New("Unauthorized")
	}

	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, errors.New("Comment cannot be empty")
	}

	if r := []rune(comment); len(r) > 5000 {
		comment = string(r[:5000])
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return nil, errors.New("User not found")
	}

	if _, err := s.visiblePost(feedID, user.ID, feedType); err != nil {
		return nil, err
	}

	if s.cooldown != nil {
		key := fmt.Sprintf("feed_comment:%d", user.ID)
		if allowed, remaining := s.cooldown.Allow(key, 5*time.Second); !allowed {
			return nil, fmt.Errorf("Please wait %s before replying again.", s.cooldown.FormatRemaining(remaining))
		}
	}

	commentID, err := s.feedRepo.CreateComment(feedID, user.ID, parentID, feedType, comment)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	res := &models.FeedComment{
		ID:           commentID,
		FeedID:       feedID,
		UserID:       user.ID,
		FeedType:     feedType,
		Username:     user.Username,
		Comment:      comment,
		Removed:      "false",
		Edited:       "false",
		CreationDate: now,
		TimeAgo:      "Just now",
		FullDate:     now.Format("January 02, 2006 at 03:04 PM"),
		Reactions:    0,
		HasReacted:   false,
	}
	if parentID != nil && *parentID > 0 {
		res.ParentID.Int64 = int64(*parentID)
		res.ParentID.Valid = true
	}

	return res, nil
}

func (s *FeedService) GetFeedComments(feedID int, feedType, currentUsername string) ([]*models.FeedComment, error) {
	var currentUserID int
	if currentUsername != "" {
		u, _ := s.userRepo.GetByUsername(currentUsername)
		if u != nil {
			currentUserID = u.ID
		}
	}

	if _, err := s.visiblePost(feedID, currentUserID, feedType); err != nil {
		return nil, err
	}

	flatList, err := s.feedRepo.GetCommentsByFeedID(feedID, feedType, currentUserID)
	if err != nil {
		return nil, err
	}

	commentMap := make(map[int]*models.FeedComment)
	var rootComments []*models.FeedComment

	for _, c := range flatList {
		c.TimeAgo = FormatTimeAgo(c.CreationDate)
		c.FullDate = c.CreationDate.Format("January 02, 2006 at 03:04 PM")
		commentMap[c.ID] = c
	}

	for _, c := range flatList {
		if c.ParentID.Valid && c.ParentID.Int64 > 0 {
			parentID := int(c.ParentID.Int64)
			if parent, exists := commentMap[parentID]; exists {
				parent.Replies = append(parent.Replies, c)
			} else {
				rootComments = append(rootComments, c)
			}
		} else {
			rootComments = append(rootComments, c)
		}
	}

	return rootComments, nil
}

func (s *FeedService) ToggleCommentReaction(username string, commentID int) (int, bool, error) {
	if username == "" || commentID <= 0 {
		return 0, false, errors.New("invalid request")
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return 0, false, errors.New("user not found")
	}

	if _, _, err := s.CommentPost(user.Username, commentID); err != nil {
		return 0, false, err
	}

	if s.cooldown != nil {
		key := fmt.Sprintf("creaction:%d:%d", user.ID, commentID)
		if allowed, remaining := s.cooldown.Allow(key, 2*time.Second); !allowed {
			return 0, false, fmt.Errorf("Please wait %s before reacting again.", s.cooldown.FormatRemaining(remaining))
		}
	}

	hasReacted, err := s.feedRepo.HasUserReactedComment(user.ID, commentID)
	if err != nil {
		return 0, false, err
	}

	if hasReacted {
		if err := s.feedRepo.RemoveCommentReaction(user.ID, commentID); err != nil {
			return 0, false, err
		}
	} else {
		if err := s.feedRepo.AddCommentReaction(user.ID, commentID); err != nil {
			return 0, false, err
		}
	}

	count, err := s.feedRepo.GetCommentReactionCount(commentID)
	if err != nil {
		return 0, false, err
	}
	return count, !hasReacted, nil
}

func (s *FeedService) HasUserReactedComment(username string, commentID int) bool {
	if username == "" || commentID <= 0 {
		return false
	}
	user, err := s.userRepo.GetByUsername(username)
	if err != nil || user == nil {
		return false
	}
	hasReacted, _ := s.feedRepo.HasUserReactedComment(user.ID, commentID)
	return hasReacted
}
