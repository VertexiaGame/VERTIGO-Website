package service

import (
	"strings"
	"testing"
	"time"
)

func TestTokenIssueValidateRevoke(t *testing.T) {
	s := NewTokenService(time.Hour, 2*time.Hour)
	raw, tok, err := s.Issue(42, "studio")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, accessTokenPrefix) || len(raw) < 40 {
		t.Fatalf("unexpected token format %q", raw)
	}
	if tok.Kind != TokenKindStudio || tok.ExpiresAt.Sub(tok.CreatedAt) != 2*time.Hour {
		t.Fatalf("studio token metadata wrong: %+v", tok)
	}
	got, err := s.Validate(raw)
	if err != nil || got.UserID != 42 {
		t.Fatalf("validate = %+v, %v", got, err)
	}
	if err := s.Revoke(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Validate(raw); err != ErrInvalidToken {
		t.Fatalf("revoked token still valid: %v", err)
	}
}

func TestTokenRejectsGarbageAndExpired(t *testing.T) {
	s := NewTokenService(time.Minute, time.Minute)
	for _, raw := range []string{"", "abc", "vxa_nope", strings.Repeat("vxa_", 50)} {
		if _, err := s.Validate(raw); err == nil {
			t.Fatalf("garbage token %q accepted", raw)
		}
	}
	raw, _, err := s.Issue(7, "client")
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := s.Validate(raw); err != ErrInvalidToken {
		t.Fatalf("expired token accepted: %v", err)
	}
}

func TestTokenRevokeAll(t *testing.T) {
	s := NewTokenService(time.Hour, time.Hour)
	a, _, _ := s.Issue(9, "client")
	b, _, _ := s.Issue(9, "studio")
	other, _, _ := s.Issue(10, "client")
	if err := s.RevokeAll(9); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{a, b} {
		if _, err := s.Validate(raw); err == nil {
			t.Fatal("RevokeAll left a token valid")
		}
	}
	if _, err := s.Validate(other); err != nil {
		t.Fatalf("RevokeAll revoked another user's token: %v", err)
	}
}

func TestStudioCodeIsSingleUse(t *testing.T) {
	s := NewStudioAuthService(nil)
	code, err := s.GenerateCodeForState("state-1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.GetPendingCode("state-1"); !ok || got != code {
		t.Fatalf("pending code = %q, %v", got, ok)
	}
	// The repository is nil, so a valid code fails after it is consumed;
	// the second attempt must fail earlier as an unknown code.
	if _, err := s.ConsumeCode(code); err == nil || !strings.Contains(err.Error(), "repository") {
		t.Fatalf("first redemption should reach the user lookup, got %v", err)
	}
	if _, err := s.ConsumeCode(code); err == nil || !strings.Contains(err.Error(), "invalid or expired") {
		t.Fatalf("code redeemed twice: %v", err)
	}
}

func TestCooldownSharedState(t *testing.T) {
	c := NewCooldownService(time.Minute)
	if ok, _ := c.Allow("cooldown-test:user"); !ok {
		t.Fatal("first action must be allowed")
	}
	ok, remaining := c.Allow("cooldown-test:user")
	if ok || remaining <= 0 || remaining > time.Minute {
		t.Fatalf("second action allowed=%v remaining=%v", ok, remaining)
	}
}
