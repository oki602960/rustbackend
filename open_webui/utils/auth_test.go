package utils

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T) {
	t.Parallel()

	hashed, err := GetPasswordHash("password-123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("password-123", hashed) {
		t.Fatal("expected password verification")
	}
}

func TestCreateAndDecodeToken(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	token, expiresAt, err := CreateToken("secret", "user-1", "1h", now)
	if err != nil {
		t.Fatal(err)
	}
	if expiresAt == nil {
		t.Fatal("expected expires at")
	}

	claims, err := DecodeToken("secret", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ID != "user-1" {
		t.Fatalf("unexpected user id: %s", claims.ID)
	}
}

func TestCreateAPIKey(t *testing.T) {
	t.Parallel()

	apiKey, err := CreateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(apiKey, "sk-") {
		t.Fatalf("unexpected api key: %s", apiKey)
	}
}
