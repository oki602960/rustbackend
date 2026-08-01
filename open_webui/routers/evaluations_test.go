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

func TestEvaluationsRouterFeedbackCRUD(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	feedbacks := models.NewFeedbacksTable(db)

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

	config := &EvaluationsRuntimeConfig{
		WebUISecretKey:              "secret",
		EnableAPIKeys:               true,
		EnableEvaluationArenaModels: true,
		EvaluationArenaModels:       []map[string]any{{"id": "model-a"}},
	}
	evaluationsRouter := &EvaluationsRouter{
		Config:    config,
		Users:     users,
		Feedbacks: feedbacks,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	evaluationsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Eval User",
		"email":             "eval@example.com",
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

	configReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/config", nil)
	configReq.Header.Set("Authorization", "Bearer "+token)
	configRes := httptest.NewRecorder()
	mux.ServeHTTP(configRes, configReq)
	if configRes.Code != http.StatusOK {
		t.Fatalf("unexpected evaluations config status: %d", configRes.Code)
	}

	feedbackBody, _ := json.Marshal(map[string]any{
		"type": "rating",
		"data": map[string]any{"rating": 1, "model_id": "model-a"},
	})
	feedbackReq := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations/feedback", bytes.NewReader(feedbackBody))
	feedbackReq.Header.Set("Authorization", "Bearer "+token)
	feedbackRes := httptest.NewRecorder()
	mux.ServeHTTP(feedbackRes, feedbackReq)
	if feedbackRes.Code != http.StatusOK {
		t.Fatalf("unexpected create feedback status: %d", feedbackRes.Code)
	}
	var feedbackPayload map[string]any
	if err := json.Unmarshal(feedbackRes.Body.Bytes(), &feedbackPayload); err != nil {
		t.Fatal(err)
	}
	feedbackID, _ := feedbackPayload["id"].(string)

	userFeedbacksReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/feedbacks/user", nil)
	userFeedbacksReq.Header.Set("Authorization", "Bearer "+token)
	userFeedbacksRes := httptest.NewRecorder()
	mux.ServeHTTP(userFeedbacksRes, userFeedbacksReq)
	if userFeedbacksRes.Code != http.StatusOK {
		t.Fatalf("unexpected user feedbacks status: %d", userFeedbacksRes.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/feedbacks/list", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected feedback list status: %d", listRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/feedback/"+feedbackID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get feedback status: %d", getRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"type": "rating",
		"data": map[string]any{"rating": -1, "model_id": "model-a"},
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations/feedback/"+feedbackID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update feedback status: %d", updateRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/feedback/"+feedbackID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete feedback status: %d", deleteRes.Code)
	}

	deleteAllReq := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/feedbacks", nil)
	deleteAllReq.Header.Set("Authorization", "Bearer "+token)
	deleteAllRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteAllRes, deleteAllReq)
	if deleteAllRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete user feedbacks status: %d", deleteAllRes.Code)
	}
}
