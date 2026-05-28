package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	manager := NewManager(time.Hour)
	hash, err := manager.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	hashAgain, err := manager.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password again: %v", err)
	}
	if hash == hashAgain {
		t.Fatal("expected salted password hashes to differ")
	}
	if !manager.CheckPassword(hash, "secret") {
		t.Fatal("expected password to match")
	}
	if manager.CheckPassword(hash, "wrong") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestNewToken(t *testing.T) {
	manager := NewManager(time.Hour)
	tokenA, err := manager.NewToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	tokenB, err := manager.NewToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if tokenA == tokenB {
		t.Fatal("expected unique tokens")
	}
}
