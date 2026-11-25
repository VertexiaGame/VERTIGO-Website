package service

import (
	"errors"
	"strings"
	"testing"
)

const phpArgon2Hash = "$argon2id$v=19$m=65536,t=4,p=1$TjEvS0FERXlHa3VnU05YaQ$BOZSJ+TqrWmd4A+AMoSLtxnFYGqElb9pUXmJ+u8hGdU"

func TestVerifyPasswordAcceptsLegacyPHPHash(t *testing.T) {
	if err := VerifyPassword(phpArgon2Hash, "hunter2-legacy"); err != nil {
		t.Fatalf("legacy PHP hash should verify, got %v", err)
	}
	if err := VerifyPassword(phpArgon2Hash, "wrong"); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("wrong password: want ErrPasswordMismatch, got %v", err)
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=4,p=1$") {
		t.Fatalf("unexpected hash format: %s", h)
	}
	if err := VerifyPassword(h, "correct horse battery staple"); err != nil {
		t.Fatalf("round trip failed: %v", err)
	}
	if err := VerifyPassword(h, "Correct horse battery staple"); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("want ErrPasswordMismatch, got %v", err)
	}
	h2, _ := HashPassword("correct horse battery staple")
	if h == h2 {
		t.Fatal("two hashes of the same password should use different salts")
	}
}

func TestVerifyPasswordRejectsOtherFormats(t *testing.T) {
	for _, h := range []string{
		"$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		"$argon2i$v=19$m=65536,t=4,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=16$m=65536,t=4,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=99999999,t=4,p=1$c2FsdHNhbHQ$aGFzaGhhc2g",
		"$argon2id$v=19$m=65536,t=4,p=1$$aGFzaGhhc2g",
		"plaintext",
		"",
	} {
		if err := VerifyPassword(h, "x"); !errors.Is(err, ErrUnsupportedPassword) {
			t.Errorf("%q: want ErrUnsupportedPassword, got %v", h, err)
		}
	}
}
