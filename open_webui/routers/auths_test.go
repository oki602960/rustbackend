package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func openRouterTestDB(t *testing.T) *dbinternal.Handle {
	t.Helper()

	tempDir := t.TempDir()
	db, err := dbinternal.Open(context.Background(), dbinternal.Options{
		DatabaseURL:     "sqlite:///" + filepath.Join(tempDir, "router.db"),
		EnableSQLiteWAL: true,
		OpenTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAuthsRouterSignupAndSignin(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	router := &AuthsRouter{
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

	mux := http.NewServeMux()
	router.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Tester",
		"email":             "tester@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	signupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(signupBody))
	signupRes := httptest.NewRecorder()
	mux.ServeHTTP(signupRes, signupReq)
	if signupRes.Code != http.StatusOK {
		t.Fatalf("unexpected signup status: %d", signupRes.Code)
	}

	signinBody, _ := json.Marshal(map[string]any{
		"email":    "tester@example.com",
		"password": "password-123",
	})
	signinReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signin", bytes.NewReader(signinBody))
	signinRes := httptest.NewRecorder()
	mux.ServeHTTP(signinRes, signinReq)
	if signinRes.Code != http.StatusOK {
		t.Fatalf("unexpected signin status: %d", signinRes.Code)
	}

	var signinPayload map[string]any
	if err := json.Unmarshal(signinRes.Body.Bytes(), &signinPayload); err != nil {
		t.Fatal(err)
	}
	token, _ := signinPayload["token"].(string)
	if token == "" {
		t.Fatal("expected signin token")
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/", nil)
	sessionReq.Header.Set("Authorization", "Bearer "+token)
	sessionRes := httptest.NewRecorder()
	mux.ServeHTTP(sessionRes, sessionReq)
	if sessionRes.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionRes.Code)
	}

	signoutReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/signout", nil)
	signoutRes := httptest.NewRecorder()
	mux.ServeHTTP(signoutRes, signoutReq)
	if signoutRes.Code != http.StatusOK {
		t.Fatalf("unexpected signout status: %d", signoutRes.Code)
	}

	profileBody, _ := json.Marshal(map[string]any{
		"name":              "Tester Updated",
		"profile_image_url": "/user.png",
		"bio":               "hello",
	})
	profileReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/update/profile", bytes.NewReader(profileBody))
	profileReq.Header.Set("Authorization", "Bearer "+token)
	profileRes := httptest.NewRecorder()
	mux.ServeHTTP(profileRes, profileReq)
	if profileRes.Code != http.StatusOK {
		t.Fatalf("unexpected profile update status: %d", profileRes.Code)
	}

	timezoneBody, _ := json.Marshal(map[string]any{
		"timezone": "Asia/Shanghai",
	})
	timezoneReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/update/timezone", bytes.NewReader(timezoneBody))
	timezoneReq.Header.Set("Authorization", "Bearer "+token)
	timezoneRes := httptest.NewRecorder()
	mux.ServeHTTP(timezoneRes, timezoneReq)
	if timezoneRes.Code != http.StatusOK {
		t.Fatalf("unexpected timezone update status: %d", timezoneRes.Code)
	}

	passwordBody, _ := json.Marshal(map[string]any{
		"password":     "password-123",
		"new_password": "password-456",
	})
	passwordReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/update/password", bytes.NewReader(passwordBody))
	passwordReq.Header.Set("Authorization", "Bearer "+token)
	passwordRes := httptest.NewRecorder()
	mux.ServeHTTP(passwordRes, passwordReq)
	if passwordRes.Code != http.StatusOK {
		t.Fatalf("unexpected password update status: %d", passwordRes.Code)
	}

	signinNewBody, _ := json.Marshal(map[string]any{
		"email":    "tester@example.com",
		"password": "password-456",
	})
	signinNewReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signin", bytes.NewReader(signinNewBody))
	signinNewRes := httptest.NewRecorder()
	mux.ServeHTTP(signinNewRes, signinNewReq)
	if signinNewRes.Code != http.StatusOK {
		t.Fatalf("unexpected signin with new password status: %d", signinNewRes.Code)
	}

	apiKeyCreateReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/api_key", nil)
	apiKeyCreateReq.Header.Set("Authorization", "Bearer "+token)
	apiKeyCreateRes := httptest.NewRecorder()
	mux.ServeHTTP(apiKeyCreateRes, apiKeyCreateReq)
	if apiKeyCreateRes.Code != http.StatusOK {
		t.Fatalf("unexpected api key create status: %d", apiKeyCreateRes.Code)
	}

	apiKeyGetReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/api_key", nil)
	apiKeyGetReq.Header.Set("Authorization", "Bearer "+token)
	apiKeyGetRes := httptest.NewRecorder()
	mux.ServeHTTP(apiKeyGetRes, apiKeyGetReq)
	if apiKeyGetRes.Code != http.StatusOK {
		t.Fatalf("unexpected api key get status: %d", apiKeyGetRes.Code)
	}
	var apiKeyPayload map[string]any
	if err := json.Unmarshal(apiKeyGetRes.Body.Bytes(), &apiKeyPayload); err != nil {
		t.Fatal(err)
	}
	apiKey, _ := apiKeyPayload["api_key"].(string)
	if apiKey == "" {
		t.Fatal("expected api key")
	}

	sessionByAPIKeyReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/", nil)
	sessionByAPIKeyReq.Header.Set("Authorization", "Bearer "+apiKey)
	sessionByAPIKeyRes := httptest.NewRecorder()
	mux.ServeHTTP(sessionByAPIKeyRes, sessionByAPIKeyReq)
	if sessionByAPIKeyRes.Code != http.StatusOK {
		t.Fatalf("unexpected api key session status: %d", sessionByAPIKeyRes.Code)
	}

	apiKeyDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/auths/api_key", nil)
	apiKeyDeleteReq.Header.Set("Authorization", "Bearer "+token)
	apiKeyDeleteRes := httptest.NewRecorder()
	mux.ServeHTTP(apiKeyDeleteRes, apiKeyDeleteReq)
	if apiKeyDeleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected api key delete status: %d", apiKeyDeleteRes.Code)
	}

	addUserBody, _ := json.Marshal(map[string]any{
		"name":              "Added User",
		"email":             "added@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
		"role":              "user",
	})
	addUserReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/add", bytes.NewReader(addUserBody))
	addUserReq.Header.Set("Authorization", "Bearer "+token)
	addUserRes := httptest.NewRecorder()
	mux.ServeHTTP(addUserRes, addUserReq)
	if addUserRes.Code != http.StatusOK {
		t.Fatalf("unexpected add user status: %d", addUserRes.Code)
	}

	adminDetailsReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/admin/details", nil)
	adminDetailsReq.Header.Set("Authorization", "Bearer "+token)
	adminDetailsRes := httptest.NewRecorder()
	mux.ServeHTTP(adminDetailsRes, adminDetailsReq)
	if adminDetailsRes.Code != http.StatusOK {
		t.Fatalf("unexpected admin details status: %d", adminDetailsRes.Code)
	}

	adminConfigReq := httptest.NewRequest(http.MethodGet, "/api/v1/auths/admin/config", nil)
	adminConfigReq.Header.Set("Authorization", "Bearer "+token)
	adminConfigRes := httptest.NewRecorder()
	mux.ServeHTTP(adminConfigRes, adminConfigReq)
	if adminConfigRes.Code != http.StatusOK {
		t.Fatalf("unexpected admin config status: %d", adminConfigRes.Code)
	}
}
