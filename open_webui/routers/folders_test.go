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

func TestFoldersRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	folders := models.NewFoldersTable(db)

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

	foldersRouter := &FoldersRouter{
		Config: FoldersRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:   users,
		Folders: folders,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	foldersRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Folder User",
		"email":             "folder@example.com",
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
		"name": "Folder One",
		"data": map[string]any{"kind": "root"},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/folders/", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create folder status: %d", createRes.Code)
	}
	var folderPayload map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &folderPayload); err != nil {
		t.Fatal(err)
	}
	folderID, _ := folderPayload["id"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/folders/", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected list folders status: %d", listRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/folders/"+folderID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get folder status: %d", getRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"name": "Folder Updated",
		"meta": map[string]any{"icon": "folder"},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/folders/"+folderID+"/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update folder status: %d", updateRes.Code)
	}

	expandBody, _ := json.Marshal(map[string]any{
		"is_expanded": true,
	})
	expandReq := httptest.NewRequest(http.MethodPost, "/api/v1/folders/"+folderID+"/update/expanded", bytes.NewReader(expandBody))
	expandReq.Header.Set("Authorization", "Bearer "+token)
	expandRes := httptest.NewRecorder()
	mux.ServeHTTP(expandRes, expandReq)
	if expandRes.Code != http.StatusOK {
		t.Fatalf("unexpected update expanded status: %d", expandRes.Code)
	}

	parentBody, _ := json.Marshal(map[string]any{
		"parent_id": nil,
	})
	parentReq := httptest.NewRequest(http.MethodPost, "/api/v1/folders/"+folderID+"/update/parent", bytes.NewReader(parentBody))
	parentReq.Header.Set("Authorization", "Bearer "+token)
	parentRes := httptest.NewRecorder()
	mux.ServeHTTP(parentRes, parentReq)
	if parentRes.Code != http.StatusOK {
		t.Fatalf("unexpected update parent status: %d", parentRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/folders/"+folderID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete folder status: %d", deleteRes.Code)
	}
}
