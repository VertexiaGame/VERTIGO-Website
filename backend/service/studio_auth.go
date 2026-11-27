package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"vertexia-frontend/backend/kv"
	"vertexia-frontend/backend/models"
	"vertexia-frontend/backend/repository"
)

// Studio/client browser authorization. Codes and pending states live in
// Redis (via kv) so any platform instance can finish a flow another started
// and nothing is lost on restart.

const (
	studioCodeTTL     = 5 * time.Minute
	studioStateTTL    = 10 * time.Minute
	studioCodePrefix  = "studio:code:"
	studioStatePrefix = "studio:state:"
	maxStudioStateLen = 256
)

type pendingStateEntry struct {
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	Code        string    `json:"code,omitempty"`
	Authorized  bool      `json:"authorized"`
	UserID      int       `json:"uid,omitempty"`
}

type StudioAuthService struct {
	userRepo *repository.UserRepository
}

func NewStudioAuthService(userRepo *repository.UserRepository) *StudioAuthService {
	return &StudioAuthService{userRepo: userRepo}
}

func generateRandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func studioHash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func (s *StudioAuthService) loadState(state string) (*pendingStateEntry, error) {
	ctx, cancel := kv.Ctx()
	defer cancel()
	raw, err := kv.Get(ctx, studioStatePrefix+studioHash(state))
	if err != nil || raw == nil {
		return nil, err
	}
	var entry pendingStateEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, nil
	}
	return &entry, nil
}

func (s *StudioAuthService) saveState(state string, entry *pendingStateEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	ttl := studioStateTTL - time.Since(entry.CreatedAt)
	if ttl <= 0 {
		return nil
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	return kv.Set(ctx, studioStatePrefix+studioHash(state), raw, ttl)
}

func (s *StudioAuthService) RegisterPendingState(state, redirectURI string) {
	if state == "" || len(state) > maxStudioStateLen {
		return
	}
	raw, err := json.Marshal(&pendingStateEntry{RedirectURI: redirectURI, CreatedAt: time.Now()})
	if err != nil {
		return
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	// NX: never let a second visit reset an already authorized state.
	_, _ = kv.SetNX(ctx, studioStatePrefix+studioHash(state), raw, studioStateTTL)
}

func (s *StudioAuthService) GenerateCode(userID int) (string, error) {
	code, err := generateRandomHex(32)
	if err != nil {
		return "", err
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	if err := kv.Set(ctx, studioCodePrefix+studioHash(code), []byte(strconv.Itoa(userID)), studioCodeTTL); err != nil {
		return "", err
	}
	return code, nil
}

func (s *StudioAuthService) GenerateCodeForState(state string, userID int) (string, error) {
	if len(state) > maxStudioStateLen {
		return "", errors.New("state too long")
	}
	code, err := s.GenerateCode(userID)
	if err != nil {
		return "", err
	}
	entry, err := s.loadState(state)
	if err != nil {
		return "", err
	}
	if entry == nil {
		entry = &pendingStateEntry{CreatedAt: time.Now()}
	}
	entry.Code = code
	entry.Authorized = true
	entry.UserID = userID
	if err := s.saveState(state, entry); err != nil {
		return "", err
	}
	return code, nil
}

func (s *StudioAuthService) GetPendingCode(state string) (string, bool) {
	if state == "" || len(state) > maxStudioStateLen {
		return "", false
	}
	entry, err := s.loadState(state)
	if err != nil || entry == nil || !entry.Authorized || entry.Code == "" {
		return "", false
	}
	if time.Since(entry.CreatedAt) > studioStateTTL {
		return "", false
	}
	return entry.Code, true
}

// ConsumeCode redeems a one-time code. GETDEL makes redemption atomic across
// instances, so a code can never be exchanged twice.
func (s *StudioAuthService) ConsumeCode(code string) (*models.User, error) {
	if len(code) != 64 {
		return nil, errors.New("invalid or expired code")
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	raw, err := kv.GetDel(ctx, studioCodePrefix+studioHash(code))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("invalid or expired code")
	}
	userID, err := strconv.Atoi(string(raw))
	if err != nil || userID <= 0 {
		return nil, errors.New("invalid or expired code")
	}
	if s.userRepo == nil {
		return nil, errors.New("user repository not initialized")
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}
