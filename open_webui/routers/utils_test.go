package routers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func TestUtilsRouterLightEndpoints(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)

	authRouter := &AuthsRouter{
		Config: AuthRuntimeConfig{
			WebUIAuth:                true,
			EnableInitialAdminSignup: true,
			EnablePasswordAuth:       true,
			EnableAPIKeys:            true,
			EnableSignup:             true,
			DefaultUserRole:          "pending",
			ShowAdminDetails:         true,
			WebUISecretKey:           "secret",
			JWTExpiresIn:             "1h",
			AuthCookieSameSite:       "Lax",
		},
		Users: users,
		Auths: auths,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}

	utilsRouter := &UtilsRouter{
		Config: UtilsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			DatabaseURL:    "sqlite:///" + t.TempDir() + "/webui.db",
		},
		Users: users,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	utilsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Utils User",
		"email":             "utils@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	signupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(signupBody))
	signupRes := httptest.NewRecorder()
	mux.ServeHTTP(signupRes, signupReq)
	if signupRes.Code != http.StatusOK {
		t.Fatalf("unexpected signup status: %d", signupRes.Code)
	}
	var signupPayload map[string]any
	if err := json.Unmarshal(signupRes.Body.Bytes(), &signupPayload); err != nil {
		t.Fatal(err)
	}
	token, _ := signupPayload["token"].(string)

	gravatarReq := httptest.NewRequest(http.MethodGet, "/api/v1/utils/gravatar?email=test@example.com", nil)
	gravatarReq.Header.Set("Authorization", "Bearer "+token)
	gravatarRes := httptest.NewRecorder()
	mux.ServeHTTP(gravatarRes, gravatarReq)
	if gravatarRes.Code != http.StatusOK {
		t.Fatalf("unexpected gravatar status: %d", gravatarRes.Code)
	}

	markdownBody, _ := json.Marshal(map[string]any{"md": "hello"})
	markdownReq := httptest.NewRequest(http.MethodPost, "/api/v1/utils/markdown", bytes.NewReader(markdownBody))
	markdownReq.Header.Set("Authorization", "Bearer "+token)
	markdownRes := httptest.NewRecorder()
	mux.ServeHTTP(markdownRes, markdownReq)
	if markdownRes.Code != http.StatusOK {
		t.Fatalf("unexpected markdown status: %d", markdownRes.Code)
	}

	formatBody, _ := json.Marshal(map[string]any{"code": "x=1"})
	formatReq := httptest.NewRequest(http.MethodPost, "/api/v1/utils/code/format", bytes.NewReader(formatBody))
	formatReq.Header.Set("Authorization", "Bearer "+token)
	formatRes := httptest.NewRecorder()
	mux.ServeHTTP(formatRes, formatReq)
	if formatRes.Code != http.StatusOK {
		t.Fatalf("unexpected code format status: %d", formatRes.Code)
	}
}
