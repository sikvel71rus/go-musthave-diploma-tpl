package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Manager struct {
	sessionTTL time.Duration
}

func NewManager(sessionTTL time.Duration) *Manager {
	return &Manager{sessionTTL: sessionTTL}
}

func (m *Manager) HashPassword(password string) (string, error) {
	salt, err := randomBytes(16)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash[:]), nil
}

func (m *Manager) CheckPassword(encodedHash, password string) bool {
	parts := strings.Split(encodedHash, ":")
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	actual := sha256.Sum256(append(salt, []byte(password)...))
	return subtle.ConstantTimeCompare(expected, actual[:]) == 1
}

func (m *Manager) NewToken() (string, error) {
	token, err := randomBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func (m *Manager) Expiry(now time.Time) time.Time {
	return now.Add(m.sessionTTL)
}

func randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, errors.New("failed to generate random bytes")
	}
	return buf, nil
}
