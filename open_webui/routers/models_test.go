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

func TestModelsRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	modelsTable := models.NewModelsTable(db)

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

	modelsRouter := &ModelsRouter{
		Config: ModelsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			StaticDir:      "/home/xxnuo/projects/open-coreui/open-webui/backend/open_webui/static",
		},
		Users:        users,
		Models:       modelsTable,
		Groups:       models.NewGroupsTable(db),
		AccessGrants: models.NewAccessGrantsTable(db),
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	modelsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Model User",
		"email":             "model@example.com",
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
		"id":            "model-1",
		"base_model_id": "gpt-4o",
		"name":          "Model One",
		"meta":          map[string]any{"profile_image_url": "/static/favicon.png", "tags": []string{"tag1"}},
		"params":        map[string]any{"temperature": 0.7},
		"is_active":     true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/create", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create model status: %d", createRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/list", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected models list status: %d", listRes.Code)
	}

	baseReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/base", nil)
	baseReq.Header.Set("Authorization", "Bearer "+token)
	baseRes := httptest.NewRecorder()
	mux.ServeHTTP(baseRes, baseReq)
	if baseRes.Code != http.StatusOK {
		t.Fatalf("unexpected base models status: %d", baseRes.Code)
	}

	tagsReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/tags", nil)
	tagsReq.Header.Set("Authorization", "Bearer "+token)
	tagsRes := httptest.NewRecorder()
	mux.ServeHTTP(tagsRes, tagsReq)
	if tagsRes.Code != http.StatusOK {
		t.Fatalf("unexpected model tags status: %d", tagsRes.Code)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export models status: %d", exportRes.Code)
	}

	importBody, _ := json.Marshal(map[string]any{
		"models": []map[string]any{
			{
				"id":            "model-2",
				"base_model_id": "gpt-4o-mini",
				"name":          "Model Two",
				"meta":          map[string]any{},
				"params":        map[string]any{},
				"is_active":     true,
			},
		},
	})
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/import", bytes.NewReader(importBody))
	importReq.Header.Set("Authorization", "Bearer "+token)
	importRes := httptest.NewRecorder()
	mux.ServeHTTP(importRes, importReq)
	if importRes.Code != http.StatusOK {
		t.Fatalf("unexpected import models status: %d", importRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/model?id=model-1", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get model status: %d", getRes.Code)
	}

	imageReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/model/profile/image?id=model-1", nil)
	imageReq.Header.Set("Authorization", "Bearer "+token)
	imageRes := httptest.NewRecorder()
	mux.ServeHTTP(imageRes, imageReq)
	if imageRes.Code != http.StatusOK {
		t.Fatalf("unexpected model profile image status: %d", imageRes.Code)
	}

	toggleReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/toggle?id=model-1", nil)
	toggleReq.Header.Set("Authorization", "Bearer "+token)
	toggleRes := httptest.NewRecorder()
	mux.ServeHTTP(toggleRes, toggleReq)
	if toggleRes.Code != http.StatusOK {
		t.Fatalf("unexpected toggle model status: %d", toggleRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"id":            "model-1",
		"base_model_id": "gpt-4o",
		"name":          "Model Updated",
		"meta":          map[string]any{"profile_image_url": "/static/favicon.png"},
		"params":        map[string]any{"temperature": 0.2},
		"is_active":     true,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update model status: %d", updateRes.Code)
	}

	accessBody, _ := json.Marshal(map[string]any{
		"id": "model-1",
		"access_grants": []map[string]any{
			{
				"principal_type": "user",
				"principal_id":   "*",
				"permission":     "read",
			},
		},
	})
	accessReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/access/update", bytes.NewReader(accessBody))
	accessReq.Header.Set("Authorization", "Bearer "+token)
	accessRes := httptest.NewRecorder()
	mux.ServeHTTP(accessRes, accessReq)
	if accessRes.Code != http.StatusOK {
		t.Fatalf("unexpected model access update status: %d", accessRes.Code)
	}

	deleteBody, _ := json.Marshal(map[string]any{"id": "model-1"})
	deleteReq := httptest.NewRequest(http.MethodPost, "/api/v1/models/model/delete", bytes.NewReader(deleteBody))
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete model status: %d", deleteRes.Code)
	}

	deleteAllReq := httptest.NewRequest(http.MethodDelete, "/api/v1/models/delete/all", nil)
	deleteAllReq.Header.Set("Authorization", "Bearer "+token)
	deleteAllRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteAllRes, deleteAllReq)
	if deleteAllRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete all models status: %d", deleteAllRes.Code)
	}
}
