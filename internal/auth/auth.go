// Package auth handles password hashing, session-token generation, and input
// validation shared by the server.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash suitable for storage.
func HashPassword(plaintext string) (string, error) {
	if len(plaintext) < 6 {
		return "", errors.New("password must be at least 6 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plaintext matches the stored hash.
func VerifyPassword(hash, plaintext string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

// NewToken returns a 32-byte random value as 64 hex chars (session IDs).
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateEmail does a minimal structural check.
func ValidateEmail(s string) error {
	s = strings.TrimSpace(s)
	if strings.Count(s, "@") != 1 {
		return errors.New("invalid email")
	}
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return errors.New("invalid email")
	}
	if !strings.Contains(s[at+1:], ".") {
		return errors.New("invalid email")
	}
	return nil
}
