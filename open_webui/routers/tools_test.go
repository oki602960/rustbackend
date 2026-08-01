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

func TestToolsRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	toolsTable := models.NewToolsTable(db)

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

	toolsRouter := &ToolsRouter{
		Config: ToolsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:        users,
		Tools:        toolsTable,
		Groups:       models.NewGroupsTable(db),
		AccessGrants: models.NewAccessGrantsTable(db),
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	toolsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Tool User",
		"email":             "tool@example.com",
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
		"id":      "tool_one",
		"name":    "Tool One",
		"content": "content",
		"meta":    map[string]any{"description": "desc"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/tools/create", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create tool status: %d", createRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/tools/", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get tools status: %d", getRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/tools/list", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool list status: %d", listRes.Code)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/tools/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export tools status: %d", exportRes.Code)
	}

	getByIDReq := httptest.NewRequest(http.MethodGet, "/api/v1/tools/id/tool_one", nil)
	getByIDReq.Header.Set("Authorization", "Bearer "+token)
	getByIDRes := httptest.NewRecorder()
	mux.ServeHTTP(getByIDRes, getByIDReq)
	if getByIDRes.Code != http.StatusOK {
		t.Fatalf("unexpected get tool by id status: %d", getByIDRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"name":    "Tool Updated",
		"content": "content2",
		"meta":    map[string]any{"description": "updated"},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/tools/id/tool_one/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update tool status: %d", updateRes.Code)
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
	accessReq := httptest.NewRequest(http.MethodPost, "/api/v1/tools/id/tool_one/access/update", bytes.NewReader(accessBody))
	accessReq.Header.Set("Authorization", "Bearer "+token)
	accessRes := httptest.NewRecorder()
	mux.ServeHTTP(accessRes, accessReq)
	if accessRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool access update status: %d", accessRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/tools/id/tool_one/delete", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete tool status: %d", deleteRes.Code)
	}
}
