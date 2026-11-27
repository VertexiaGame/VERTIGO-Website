package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"vertexia-frontend/backend/kv"
)

// Access tokens are opaque bearer tokens issued to the native client and
// Studio after the browser authorization flow. Only the SHA-256 of a token is
// stored (in Redis), so a leaked Redis snapshot cannot be replayed.

const (
	TokenKindClient = "client"
	TokenKindStudio = "studio"

	accessTokenPrefix = "vxa_"
	tokenKeyPrefix    = "auth:token:"
	userTokensPrefix  = "auth:user_tokens:"
)

var ErrInvalidToken = errors.New("invalid or expired access token")

type AccessToken struct {
	UserID    int       `json:"uid"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TokenService struct {
	clientTTL time.Duration
	studioTTL time.Duration
	now       func() time.Time
}

func NewTokenService(clientTTL, studioTTL time.Duration) *TokenService {
	if clientTTL <= 0 {
		clientTTL = 24 * time.Hour
	}
	if studioTTL <= 0 {
		studioTTL = 7 * 24 * time.Hour
	}
	return &TokenService{clientTTL: clientTTL, studioTTL: studioTTL, now: time.Now}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NormalizeTokenKind maps a requested client type to a known token kind.
func NormalizeTokenKind(kind string) string {
	if strings.EqualFold(strings.TrimSpace(kind), TokenKindStudio) {
		return TokenKindStudio
	}
	return TokenKindClient
}

func (s *TokenService) ttl(kind string) time.Duration {
	if kind == TokenKindStudio {
		return s.studioTTL
	}
	return s.clientTTL
}

// Issue mints a new token for userID and returns the raw token (shown to the
// caller exactly once) with its metadata.
func (s *TokenService) Issue(userID int, kind string) (string, AccessToken, error) {
	if userID <= 0 {
		return "", AccessToken{}, errors.New("invalid user")
	}
	kind = NormalizeTokenKind(kind)
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", AccessToken{}, err
	}
	raw := accessTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	now := s.now()
	tok := AccessToken{UserID: userID, Kind: kind, CreatedAt: now, ExpiresAt: now.Add(s.ttl(kind))}
	payload, err := json.Marshal(tok)
	if err != nil {
		return "", AccessToken{}, err
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	h := hashToken(raw)
	if err := kv.Set(ctx, tokenKeyPrefix+h, payload, s.ttl(kind)); err != nil {
		return "", AccessToken{}, err
	}
	_ = kv.SAdd(ctx, userTokensPrefix+strconv.Itoa(userID), h)
	return raw, tok, nil
}

// Validate resolves a raw bearer token.
func (s *TokenService) Validate(raw string) (AccessToken, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, accessTokenPrefix) || len(raw) > 128 {
		return AccessToken{}, ErrInvalidToken
	}
	ctx, cancel := kv.Ctx()
	defer cancel()
	payload, err := kv.Get(ctx, tokenKeyPrefix+hashToken(raw))
	if err != nil {
		return AccessToken{}, err
	}
	if payload == nil {
		return AccessToken{}, ErrInvalidToken
	}
	var tok AccessToken
	if err := json.Unmarshal(payload, &tok); err != nil || tok.UserID <= 0 {
		return AccessToken{}, ErrInvalidToken
	}
	if !s.now().Before(tok.ExpiresAt) {
		return AccessToken{}, ErrInvalidToken
	}
	return tok, nil
}

// Revoke deletes a token. Revoking an unknown token is not an error.
func (s *TokenService) Revoke(raw string) error {
	tok, err := s.Validate(raw)
	ctx, cancel := kv.Ctx()
	defer cancel()
	h := hashToken(strings.TrimSpace(raw))
	if delErr := kv.Del(ctx, tokenKeyPrefix+h); delErr != nil {
		return delErr
	}
	if err == nil {
		_ = kv.SRem(ctx, userTokensPrefix+strconv.Itoa(tok.UserID), h)
	}
	return nil
}

// RevokeAll deletes every token issued to userID (password change, ban).
func (s *TokenService) RevokeAll(userID int) error {
	ctx, cancel := kv.Ctx()
	defer cancel()
	key := userTokensPrefix + strconv.Itoa(userID)
	hashes, err := kv.SMembers(ctx, key)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(hashes)+1)
	for _, h := range hashes {
		keys = append(keys, tokenKeyPrefix+h)
	}
	keys = append(keys, key)
	return kv.Del(ctx, keys...)
}
