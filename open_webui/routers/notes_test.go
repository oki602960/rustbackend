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

func TestNotesRouterCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	notes := models.NewNotesTable(db)

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

	notesRouter := &NotesRouter{
		Config: NotesRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:        users,
		Notes:        notes,
		Groups:       models.NewGroupsTable(db),
		AccessGrants: models.NewAccessGrantsTable(db),
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	notesRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Note User",
		"email":             "note@example.com",
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
		"title": "First Note",
		"data": map[string]any{
			"content": map[string]any{"md": "hello"},
		},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/notes/create", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("unexpected create note status: %d", createRes.Code)
	}

	var notePayload map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &notePayload); err != nil {
		t.Fatal(err)
	}
	noteID, _ := notePayload["id"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected list notes status: %d", listRes.Code)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/search?query=first", nil)
	searchReq.Header.Set("Authorization", "Bearer "+token)
	searchRes := httptest.NewRecorder()
	mux.ServeHTTP(searchRes, searchReq)
	if searchRes.Code != http.StatusOK {
		t.Fatalf("unexpected search notes status: %d", searchRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/notes/"+noteID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get note status: %d", getRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"title": "Updated Note",
		"data": map[string]any{
			"content": map[string]any{"md": "updated"},
		},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/notes/"+noteID+"/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update note status: %d", updateRes.Code)
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
	accessReq := httptest.NewRequest(http.MethodPost, "/api/v1/notes/"+noteID+"/access/update", bytes.NewReader(accessBody))
	accessReq.Header.Set("Authorization", "Bearer "+token)
	accessRes := httptest.NewRecorder()
	mux.ServeHTTP(accessRes, accessReq)
	if accessRes.Code != http.StatusOK {
		t.Fatalf("unexpected note access update status: %d", accessRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/notes/"+noteID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete note status: %d", deleteRes.Code)
	}
}
