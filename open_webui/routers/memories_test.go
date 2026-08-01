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

func TestMemoriesRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	memories := models.NewMemoriesTable(db)

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

	memoriesRouter := &MemoriesRouter{
		Config: MemoriesRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:    users,
		Memories: memories,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	memoriesRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Memory User",
		"email":             "memory@example.com",
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

	addBody, _ := json.Marshal(map[string]any{
		"content": "memory content",
	})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/memories/add", bytes.NewReader(addBody))
	addReq.Header.Set("Authorization", "Bearer "+token)
	addRes := httptest.NewRecorder()
	mux.ServeHTTP(addRes, addReq)
	if addRes.Code != http.StatusOK {
		t.Fatalf("unexpected add memory status: %d", addRes.Code)
	}
	var memoryPayload map[string]any
	if err := json.Unmarshal(addRes.Body.Bytes(), &memoryPayload); err != nil {
		t.Fatal(err)
	}
	memoryID, _ := memoryPayload["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/memories/", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get memories status: %d", getRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"content": "updated memory",
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/memories/"+memoryID+"/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update memory status: %d", updateRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/memories/"+memoryID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete memory status: %d", deleteRes.Code)
	}

	deleteAllReq := httptest.NewRequest(http.MethodDelete, "/api/v1/memories/delete/user", nil)
	deleteAllReq.Header.Set("Authorization", "Bearer "+token)
	deleteAllRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteAllRes, deleteAllReq)
	if deleteAllRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete all memories status: %d", deleteAllRes.Code)
	}
}
