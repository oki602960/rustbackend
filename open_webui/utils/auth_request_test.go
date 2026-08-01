package utils

import (
	"net/http/httptest"
	"testing"
)

func TestExtractTokenFromRequest(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer token-123")
	if token := ExtractTokenFromRequest(request); token != "token-123" {
		t.Fatalf("unexpected token: %s", token)
	}
}
