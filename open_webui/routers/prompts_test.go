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

func TestPromptsRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	prompts := models.NewPromptsTable(db)

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

	promptsRouter := &PromptsRouter{
		Config: PromptsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:        users,
		Prompts:      prompts,
		Groups:       models.NewGroupsTable(db),
		AccessGrants: models.NewAccessGrantsTable(db),
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	promptsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Prompt User",
		"email":             "prompt@example.com",
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
		"command": "hello",
		"name":    "Hello Prompt",
		"content": "Say hello",
		"tags":    []string{"greeting"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/prompts/create", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create prompt status: %d", createRes.Code)
	}
	var promptPayload map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &promptPayload); err != nil {
		t.Fatal(err)
	}
	promptID, _ := promptPayload["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get prompts status: %d", getRes.Code)
	}

	tagsReq := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/tags", nil)
	tagsReq.Header.Set("Authorization", "Bearer "+token)
	tagsRes := httptest.NewRecorder()
	mux.ServeHTTP(tagsRes, tagsReq)
	if tagsRes.Code != http.StatusOK {
		t.Fatalf("unexpected get tags status: %d", tagsRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/list?query=hello", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected prompt list status: %d", listRes.Code)
	}

	getByCommandReq := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/command/hello", nil)
	getByCommandReq.Header.Set("Authorization", "Bearer "+token)
	getByCommandRes := httptest.NewRecorder()
	mux.ServeHTTP(getByCommandRes, getByCommandReq)
	if getByCommandRes.Code != http.StatusOK {
		t.Fatalf("unexpected get by command status: %d", getByCommandRes.Code)
	}

	getByIDReq := httptest.NewRequest(http.MethodGet, "/api/v1/prompts/id/"+promptID, nil)
	getByIDReq.Header.Set("Authorization", "Bearer "+token)
	getByIDRes := httptest.NewRecorder()
	mux.ServeHTTP(getByIDRes, getByIDReq)
	if getByIDRes.Code != http.StatusOK {
		t.Fatalf("unexpected get by id status: %d", getByIDRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"command": "hello2",
		"name":    "Hello Prompt 2",
		"content": "Say hello again",
		"tags":    []string{"greeting", "updated"},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/prompts/id/"+promptID+"/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update prompt status: %d", updateRes.Code)
	}

	accessBody, _ := json.Marshal(map[string]any{
		"access_grants": []map[string]any{
			{
				"principal_type": "user",
				"principal_id":   "*",
				"permission":     "read",
			},
		},
	})
	accessReq := httptest.NewRequest(http.MethodPost, "/api/v1/prompts/id/"+promptID+"/access/update", bytes.NewReader(accessBody))
	accessReq.Header.Set("Authorization", "Bearer "+token)
	accessRes := httptest.NewRecorder()
	mux.ServeHTTP(accessRes, accessReq)
	if accessRes.Code != http.StatusOK {
		t.Fatalf("unexpected prompt access update status: %d", accessRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/prompts/id/"+promptID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete prompt status: %d", deleteRes.Code)
	}
}
