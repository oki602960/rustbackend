package routers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/storage"
)

func TestFilesRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	filesTable := models.NewFilesTable(db)
	provider := storage.NewLocalProvider(filepath.Join(t.TempDir(), "uploads"))

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

	filesRouter := &FilesRouter{
		Config: FilesRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:        users,
		Files:        filesTable,
		Groups:       models.NewGroupsTable(db),
		AccessGrants: models.NewAccessGrantsTable(db),
		Storage:      provider,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	filesRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "File User",
		"email":             "file@example.com",
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

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello file")); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("metadata", `{"source":"test"}`)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/files/", &body)
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRes := httptest.NewRecorder()
	mux.ServeHTTP(uploadRes, uploadReq)
	if uploadRes.Code != http.StatusOK {
		t.Fatalf("unexpected upload file status: %d", uploadRes.Code)
	}
	var filePayload map[string]any
	if err := json.Unmarshal(uploadRes.Body.Bytes(), &filePayload); err != nil {
		t.Fatal(err)
	}
	fileID, _ := filePayload["id"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected file list status: %d", listRes.Code)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/search?query=hello", nil)
	searchReq.Header.Set("Authorization", "Bearer "+token)
	searchRes := httptest.NewRecorder()
	mux.ServeHTTP(searchRes, searchReq)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("unexpected file search status: %d", searchRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/"+fileID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get file status: %d", getRes.Code)
	}

	contentReq := httptest.NewRequest(http.MethodGet, "/api/v1/files/"+fileID+"/content", nil)
	contentReq.Header.Set("Authorization", "Bearer "+token)
	contentRes := httptest.NewRecorder()
	mux.ServeHTTP(contentRes, contentReq)
	if contentRes.Code != http.StatusOK {
		t.Fatalf("unexpected get file content status: %d", contentRes.Code)
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
	accessReq := httptest.NewRequest(http.MethodPost, "/api/v1/files/"+fileID+"/access/update", bytes.NewReader(accessBody))
	accessReq.Header.Set("Authorization", "Bearer "+token)
	accessRes := httptest.NewRecorder()
	mux.ServeHTTP(accessRes, accessReq)
	if accessRes.Code != http.StatusOK {
		t.Fatalf("unexpected file access update status: %d", accessRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete file status: %d", deleteRes.Code)
	}

	deleteAllReq := httptest.NewRequest(http.MethodDelete, "/api/v1/files/all", nil)
	deleteAllReq.Header.Set("Authorization", "Bearer "+token)
	deleteAllRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteAllRes, deleteAllReq)
	if deleteAllRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete all files status: %d", deleteAllRes.Code)
	}
}
