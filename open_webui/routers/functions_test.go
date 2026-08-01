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

func TestFunctionsRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	functions := models.NewFunctionsTable(db)

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

	functionsRouter := &FunctionsRouter{
		Config: FunctionsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:     users,
		Functions: functions,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	functionsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Function User",
		"email":             "function@example.com",
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

	createBody, _ := json.Marshal(map[string]any{
		"id":      "hello_fn",
		"name":    "Hello Function",
		"content": "print('hello')",
		"meta":    map[string]any{"description": "desc"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/functions/create", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create function status: %d", createRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/functions/", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get functions status: %d", getRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/functions/list", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected function list status: %d", listRes.Code)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/functions/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export functions status: %d", exportRes.Code)
	}

	getByIDReq := httptest.NewRequest(http.MethodGet, "/api/v1/functions/id/hello_fn", nil)
	getByIDReq.Header.Set("Authorization", "Bearer "+token)
	getByIDRes := httptest.NewRecorder()
	mux.ServeHTTP(getByIDRes, getByIDReq)
	if getByIDRes.Code != http.StatusOK {
		t.Fatalf("unexpected get function by id status: %d", getByIDRes.Code)
	}

	toggleReq := httptest.NewRequest(http.MethodPost, "/api/v1/functions/id/hello_fn/toggle", nil)
	toggleReq.Header.Set("Authorization", "Bearer "+token)
	toggleRes := httptest.NewRecorder()
	mux.ServeHTTP(toggleRes, toggleReq)
	if toggleRes.Code != http.StatusOK {
		t.Fatalf("unexpected toggle function status: %d", toggleRes.Code)
	}

	toggleGlobalReq := httptest.NewRequest(http.MethodPost, "/api/v1/functions/id/hello_fn/toggle/global", nil)
	toggleGlobalReq.Header.Set("Authorization", "Bearer "+token)
	toggleGlobalRes := httptest.NewRecorder()
	mux.ServeHTTP(toggleGlobalRes, toggleGlobalReq)
	if toggleGlobalRes.Code != http.StatusOK {
		t.Fatalf("unexpected toggle global status: %d", toggleGlobalRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"name":    "Hello Function Updated",
		"content": "print('updated')",
		"meta":    map[string]any{"description": "updated"},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/functions/id/hello_fn/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update function status: %d", updateRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/functions/id/hello_fn/delete", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete function status: %d", deleteRes.Code)
	}
}
