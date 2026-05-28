package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Manager struct {
	sessionTTL time.Duration
}

func NewManager(sessionTTL time.Duration) *Manager {
	return &Manager{sessionTTL: sessionTTL}
}

func (m *Manager) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func (m *Manager) CheckPassword(encodedHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password)) == nil
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
