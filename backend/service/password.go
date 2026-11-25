package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory  uint32 = 64 * 1024
	argon2Time    uint32 = 4
	argon2Threads uint8  = 1
	argon2SaltLen        = 16
	argon2KeyLen  uint32 = 32

	argon2MaxMemory  uint32 = 1024 * 1024
	argon2MaxTime    uint32 = 16
	argon2MaxThreads uint8  = 16
)

var (
	ErrPasswordMismatch    = errors.New("password does not match")
	ErrUnsupportedPassword = errors.New("password hash is not argon2id")
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return ErrUnsupportedPassword
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return ErrUnsupportedPassword
	}

	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return ErrUnsupportedPassword
	}
	if memory == 0 || memory > argon2MaxMemory || time == 0 || time > argon2MaxTime ||
		threads == 0 || threads > argon2MaxThreads {
		return ErrUnsupportedPassword
	}

	salt, err := decodeArgon2B64(parts[4])
	if err != nil || len(salt) == 0 {
		return ErrUnsupportedPassword
	}
	want, err := decodeArgon2B64(parts[5])
	if err != nil || len(want) == 0 || len(want) > 1024 {
		return ErrUnsupportedPassword
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

func decodeArgon2B64(s string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
}
