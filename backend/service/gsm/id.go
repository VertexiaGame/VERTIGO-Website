package gsm

import (
	"crypto/rand"
	"errors"
)

const idAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateID() (string, error) {
	return GenerateIDSized(8)
}

func GenerateIDSized(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("invalid id size")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range buf {
		out[i] = idAlphabet[int(buf[i])%len(idAlphabet)]
	}
	return string(out), nil
}

func ValidID(id string) bool {
	if len(id) != 8 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
