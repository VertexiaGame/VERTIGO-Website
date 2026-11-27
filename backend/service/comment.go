package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

var (
	ErrCommentNotFound   = errors.New("comment not found")
	ErrCommentPermission = errors.New("you cannot remove this comment")
	ErrCommentLength     = fmt.Errorf("comment needs to be between %d and %d characters", models.CommentMinLength, models.CommentMaxLength)
	ErrCommentTarget     = errors.New("comments are not available here")
)

const commentCooldown = 15 * time.Second

type CommentTarget struct {
	Kind    string
	ID      int
	OwnerID int
}

type commentStore interface {
	Create(kind string, targetID, userID, parentID int, body string) (int, error)
	Get(id int) (*models.Comment, error)
	ListTop(kind string, targetID, beforeID, limit int) ([]*models.Comment, error)
	ListReplies(parentIDs []int) ([]*models.Comment, error)
	Remove(id int) error
}

type CommentService struct {
	repo     commentStore
	cooldown *CooldownService
}

func NewCommentService(repo *repository.CommentRepository, cooldown *CooldownService) *CommentService {
	return &CommentService{repo: repo, cooldown: cooldown}
}

func newCommentServiceWithStore(repo commentStore, cooldown *CooldownService) *CommentService {
	return &CommentService{repo: repo, cooldown: cooldown}
}

func CanRemoveComment(viewer *models.User, c *models.Comment, target CommentTarget) bool {
	if viewer == nil || c == nil {
		return false
	}
	return viewer.ID == c.UserID || viewer.HasPower(models.PowerModerator) ||
		(target.OwnerID > 0 && viewer.ID == target.OwnerID)
}

func NormalizeCommentBody(body string) (string, error) {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	n := utf8.RuneCountInString(body)
	if n < models.CommentMinLength || n > models.CommentMaxLength {
		return "", ErrCommentLength
	}
	return body, nil
}

func (s *CommentService) decorate(c *models.Comment, viewer *models.User, target CommentTarget) {
	c.TimeAgo = FormatTimeAgo(c.CreationDate)
	c.FullDate = c.CreationDate.Format("January 02, 2006 at 03:04 PM")
	c.CanDelete = CanRemoveComment(viewer, c, target)
}

func (s *CommentService) Post(viewer *models.User, target CommentTarget, parentID int, body string) (*models.Comment, error) {
	if viewer == nil {
		return nil, errors.New("Unauthorized")
	}
	if !models.ValidCommentTarget(target.Kind) || target.ID <= 0 {
		return nil, ErrCommentTarget
	}
	body, err := NormalizeCommentBody(body)
	if err != nil {
		return nil, err
	}
	if parentID > 0 {
		parent, err := s.repo.Get(parentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.Removed || parent.TargetType != target.Kind || parent.TargetID != target.ID {
			return nil, ErrCommentNotFound
		}
		if parent.ParentID > 0 {
			parentID = parent.ParentID
		}
	} else {
		parentID = 0
	}
	if s.cooldown != nil {
		key := fmt.Sprintf("comment:%d", viewer.ID)
		if ok, remaining := s.cooldown.Allow(key, commentCooldown); !ok {
			return nil, fmt.Errorf("whoa, slow down! wait %s before posting again", s.cooldown.FormatRemaining(remaining))
		}
	}
	id, err := s.repo.Create(target.Kind, target.ID, viewer.ID, parentID, body)
	if err != nil {
		return nil, err
	}
	c := &models.Comment{
		ID: id, TargetType: target.Kind, TargetID: target.ID, UserID: viewer.ID, ParentID: parentID,
		Username: viewer.Username, Body: body, CreationDate: time.Now(),
	}
	s.decorate(c, viewer, target)
	c.TimeAgo = "just now"
	return c, nil
}

func (s *CommentService) List(viewer *models.User, target CommentTarget, beforeID int) ([]*models.Comment, bool, error) {
	if !models.ValidCommentTarget(target.Kind) {
		return nil, false, ErrCommentTarget
	}
	top, err := s.repo.ListTop(target.Kind, target.ID, beforeID, models.CommentPageSize+1)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(top) > models.CommentPageSize
	if hasMore {
		top = top[:models.CommentPageSize]
	}
	ids := make([]int, 0, len(top))
	byID := make(map[int]*models.Comment, len(top))
	for _, c := range top {
		s.decorate(c, viewer, target)
		ids = append(ids, c.ID)
		byID[c.ID] = c
	}
	replies, err := s.repo.ListReplies(ids)
	if err != nil {
		return nil, false, err
	}
	for _, r := range replies {
		if parent := byID[r.ParentID]; parent != nil {
			s.decorate(r, viewer, target)
			parent.Replies = append(parent.Replies, r)
		}
	}
	return top, hasMore, nil
}

func (s *CommentService) Get(id int) (*models.Comment, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if c == nil || c.Removed {
		return nil, ErrCommentNotFound
	}
	return c, nil
}

func (s *CommentService) Remove(viewer *models.User, c *models.Comment, target CommentTarget) error {
	if c == nil || c.Removed {
		return ErrCommentNotFound
	}
	if c.TargetType != target.Kind || c.TargetID != target.ID {
		return ErrCommentNotFound
	}
	if !CanRemoveComment(viewer, c, target) {
		return ErrCommentPermission
	}
	return s.repo.Remove(c.ID)
}
