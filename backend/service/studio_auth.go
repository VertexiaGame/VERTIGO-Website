package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

type studioCodeEntry struct {
	UserID    int
	ExpiresAt time.Time
}

type pendingStateEntry struct {
	RedirectURI string
	CreatedAt   time.Time
	Code        string
	Authorized  bool
	UserID      int
}

type StudioAuthService struct {
	mu            sync.Mutex
	codes         map[string]*studioCodeEntry
	pendingStates map[string]*pendingStateEntry
	userRepo      *repository.UserRepository
}

func NewStudioAuthService(userRepo *repository.UserRepository) *StudioAuthService {
	s := &StudioAuthService{
		codes:         make(map[string]*studioCodeEntry),
		pendingStates: make(map[string]*pendingStateEntry),
		userRepo:      userRepo,
	}
	go s.cleanupLoop()
	return s
}

func (s *StudioAuthService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.codes {
			if now.After(v.ExpiresAt) {
				delete(s.codes, k)
			}
		}
		for k, v := range s.pendingStates {
			if now.Sub(v.CreatedAt) > 10*time.Minute {
				delete(s.pendingStates, k)
			}
		}
		s.mu.Unlock()
	}
}

func generateRandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *StudioAuthService) RegisterPendingState(state, redirectURI string) {
	if state == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pendingStates[state]; exists {
		return
	}
	s.pendingStates[state] = &pendingStateEntry{
		RedirectURI: redirectURI,
		CreatedAt:   time.Now(),
	}
	if len(s.pendingStates) > 5000 {
		for k, v := range s.pendingStates {
			if time.Since(v.CreatedAt) > 5*time.Minute {
				delete(s.pendingStates, k)
			}
		}
	}
}

func (s *StudioAuthService) GenerateCode(userID int) (string, error) {
	code, err := generateRandomHex(32)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code] = &studioCodeEntry{
		UserID:    userID,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	return code, nil
}

func (s *StudioAuthService) GenerateCodeForState(state string, userID int) (string, error) {
	code, err := s.GenerateCode(userID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.pendingStates[state]; ok {
		entry.Code = code
		entry.Authorized = true
		entry.UserID = userID
	} else {
		s.pendingStates[state] = &pendingStateEntry{
			RedirectURI: "",
			CreatedAt:   time.Now(),
			Code:        code,
			Authorized:  true,
			UserID:      userID,
		}
	}
	return code, nil
}

func (s *StudioAuthService) GetPendingCode(state string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.pendingStates[state]
	if !ok || !entry.Authorized {
		return "", false
	}
	if time.Since(entry.CreatedAt) > 10*time.Minute {
		return "", false
	}
	return entry.Code, true
}

func (s *StudioAuthService) ConsumeCode(code string) (*models.User, error) {
	s.mu.Lock()
	entry, ok := s.codes[code]
	if !ok {
		s.mu.Unlock()
		return nil, errors.New("invalid or expired code")
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.codes, code)
		s.mu.Unlock()
		return nil, errors.New("code expired")
	}
	delete(s.codes, code)
	for _, pending := range s.pendingStates {
		if pending.Code == code {
			pending.Code = ""
			pending.Authorized = false
		}
	}
	s.mu.Unlock()

	if s.userRepo == nil {
		return nil, errors.New("user repository not initialized")
	}
	user, err := s.userRepo.GetByID(entry.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}


