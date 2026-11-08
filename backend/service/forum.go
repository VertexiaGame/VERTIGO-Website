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

const (
	ForumThreadTitleMin = 3
	ForumThreadTitleMax = 100
	ForumThreadBodyMin  = 3
	ForumThreadBodyMax  = 4000
)

var ErrForumBoardNotFound = errors.New("subforum not found")

type ForumService struct {
	forumRepo *repository.ForumRepository
	cooldown  *CooldownService
}

func NewForumService(forumRepo *repository.ForumRepository, cooldown *CooldownService) *ForumService {
	return &ForumService{
		forumRepo: forumRepo,
		cooldown:  cooldown,
	}
}

func (s *ForumService) GetBoards() ([]*models.ForumBoard, error) {
	if s.forumRepo == nil {
		return []*models.ForumBoard{}, nil
	}
	return s.forumRepo.GetBoards()
}

func (s *ForumService) GetBoardByID(id int) (*models.ForumBoard, error) {
	if s.forumRepo == nil || id <= 0 {
		return nil, ErrForumBoardNotFound
	}
	board, err := s.forumRepo.GetBoardByID(id)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, ErrForumBoardNotFound
	}
	return board, nil
}

func (s *ForumService) GetThreadsByBoard(boardID int, sortBy string, limit, offset int) ([]*models.ForumThread, error) {
	if s.forumRepo == nil || boardID <= 0 {
		return []*models.ForumThread{}, nil
	}
	threads, err := s.forumRepo.GetThreadsByBoard(boardID, sortBy, limit, offset)
	if err != nil {
		return nil, err
	}
	if threads == nil {
		threads = []*models.ForumThread{}
	}
	return threads, nil
}

func (s *ForumService) CreateThread(user *models.User, boardID int, title, body string) (*models.ForumThread, error) {
	if s.forumRepo == nil {
		return nil, errors.New("forum service unavailable")
	}
	if user == nil || user.ID <= 0 {
		return nil, errors.New("you must be signed in to create a thread")
	}

	board, err := s.GetBoardByID(boardID)
	if err != nil || board == nil {
		return nil, ErrForumBoardNotFound
	}

	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	body = strings.TrimSpace(body)

	titleLen := utf8.RuneCountInString(title)
	if titleLen < ForumThreadTitleMin {
		return nil, fmt.Errorf("thread title must be at least %d characters", ForumThreadTitleMin)
	}
	if titleLen > ForumThreadTitleMax {
		return nil, fmt.Errorf("thread title can't be longer than %d characters", ForumThreadTitleMax)
	}

	bodyLen := utf8.RuneCountInString(body)
	if bodyLen < ForumThreadBodyMin {
		return nil, fmt.Errorf("thread content must be at least %d characters", ForumThreadBodyMin)
	}
	if bodyLen > ForumThreadBodyMax {
		return nil, fmt.Errorf("thread content can't be longer than %d characters", ForumThreadBodyMax)
	}

	if s.cooldown != nil {
		if ok, remaining := s.cooldown.Allow(fmt.Sprintf("forum-thread:%d", user.ID), 10*time.Second); !ok {
			return nil, fmt.Errorf("slow down for %s", s.cooldown.FormatRemaining(remaining))
		}
	}

	id, err := s.forumRepo.CreateThread(board.ID, user.ID, title, body)
	if err != nil {
		return nil, err
	}

	created, err := s.forumRepo.GetThreadByID(id)
	if err != nil || created == nil {
		now := time.Now()
		created = &models.ForumThread{
			ID:                id,
			BoardID:           board.ID,
			UserID:            user.ID,
			AuthorUsername:    user.Username,
			AuthorDisplayName: user.DisplayName,
			Title:             title,
			Body:              body,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
	}
	return created, nil
}
