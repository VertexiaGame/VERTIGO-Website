package service

import (
	"strings"
	"testing"
)

func TestHashRegistrationIP(t *testing.T) {
	const salt = "test-salt"
	const ip = "203.0.113.7"
	hash := hashRegistrationIP(ip, salt)

	if len(hash) != 64 {
		t.Fatalf("expected a 64-character hash, got %d", len(hash))
	}
	if strings.Contains(hash, ip) {
		t.Fatal("hash must not contain the raw IP")
	}
	if got := hashRegistrationIP(ip, salt); got != hash {
		t.Fatal("hash must be deterministic for the same IP and salt")
	}
	if got := hashRegistrationIP(ip, "other-salt"); got == hash {
		t.Fatal("hash must depend on the salt")
	}
	if got := hashRegistrationIP("::ffff:"+ip, salt); got != hash {
		t.Fatal("IPv4-mapped IPv6 addresses must normalize to the same hash")
	}
	if got := hashRegistrationIP("  "+ip+"  ", salt); got != hash {
		t.Fatal("surrounding whitespace must be ignored")
	}
}
