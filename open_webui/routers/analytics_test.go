package routers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func TestAnalyticsRouterEndpoints(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	feedbacks := models.NewFeedbacksTable(db)

	now := time.Now().UTC().Unix()

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
			return time.Unix(now, 0).UTC()
		},
	}

	analyticsRouter := &AnalyticsRouter{
		Config: AnalyticsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
		},
		Users:        users,
		ChatMessages: models.NewChatMessagesTable(db),
		Chats:        models.NewChatsTable(db),
		Feedbacks:    feedbacks,
		Now: func() time.Time {
			return time.Unix(now, 0).UTC()
		},
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	analyticsRouter.Register(mux)

	adminToken, adminID := signupAnalyticsUser(t, mux, "Admin User", "admin-analytics@example.com")
	_, user1ID := signupAnalyticsUser(t, mux, "User One", "user1-analytics@example.com")
	_, user2ID := signupAnalyticsUser(t, mux, "User Two", "user2-analytics@example.com")

	if adminID == "" {
		t.Fatal("expected signup tokens and ids")
	}

	if err := insertAnalyticsChat(t, db.DB, "chat-1", user1ID, "Chat One", map[string]any{
		"history": map[string]any{
			"currentId": "a1",
			"messages": map[string]any{
				"u1": map[string]any{
					"id":        "u1",
					"role":      "user",
					"content":   "hello analytics",
					"timestamp": now - 300,
				},
				"a1": map[string]any{
					"id":        "a1",
					"role":      "assistant",
					"content":   "response one",
					"model":     "model-a",
					"timestamp": now - 290,
					"usage": map[string]any{
						"input_tokens":  7,
						"output_tokens": 11,
					},
				},
			},
		},
		"meta": map[string]any{
			"tags": []any{"tag-a", "tag-common"},
		},
	}, now-300, now-290); err != nil {
		t.Fatal(err)
	}
	if err := insertAnalyticsChat(t, db.DB, "chat-2", user1ID, "Chat Two", map[string]any{
		"history": map[string]any{
			"currentId": "a2",
			"messages": map[string]any{
				"u2": map[string]any{
					"id":        "u2",
					"role":      "user",
					"content":   []any{map[string]any{"text": "second question"}},
					"timestamp": now - 200,
				},
				"a2": map[string]any{
					"id":        "a2",
					"role":      "assistant",
					"content":   "response two",
					"model":     "model-a",
					"timestamp": now - 190,
					"usage": map[string]any{
						"input_tokens":  9,
						"output_tokens": 13,
					},
				},
			},
		},
		"meta": map[string]any{
			"tags": []any{"tag-b", "tag-common"},
		},
	}, now-200, now-190); err != nil {
		t.Fatal(err)
	}
	if err := insertAnalyticsChat(t, db.DB, "chat-3", user2ID, "Chat Three", map[string]any{
		"history": map[string]any{
			"currentId": "a3",
			"messages": map[string]any{
				"u3": map[string]any{
					"id":        "u3",
					"role":      "user",
					"content":   "third question",
					"timestamp": now - 100,
				},
				"a3": map[string]any{
					"id":        "a3",
					"role":      "assistant",
					"content":   "response three",
					"model":     "model-b",
					"timestamp": now - 90,
					"usage": map[string]any{
						"input_tokens":  5,
						"output_tokens": 8,
					},
				},
			},
		},
		"meta": map[string]any{
			"tags": []any{"tag-c"},
		},
	}, now-100, now-90); err != nil {
		t.Fatal(err)
	}

	if _, err := feedbacks.InsertNewFeedback(context.Background(), models.FeedbackCreateParams{
		UserID: adminID,
		Type:   "rating",
		Data:   map[string]any{"rating": 1},
		Meta:   map[string]any{"chat_id": "chat-1"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := feedbacks.InsertNewFeedback(context.Background(), models.FeedbackCreateParams{
		UserID: adminID,
		Type:   "rating",
		Data:   map[string]any{"rating": -1},
		Meta:   map[string]any{"chat_id": "chat-2"},
	}); err != nil {
		t.Fatal(err)
	}

	modelsReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/models", nil)
	modelsReq.Header.Set("Authorization", "Bearer "+adminToken)
	modelsRes := httptest.NewRecorder()
	mux.ServeHTTP(modelsRes, modelsReq)
	if modelsRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics models status: %d", modelsRes.Code)
	}

	usersReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/users?limit=10", nil)
	usersReq.Header.Set("Authorization", "Bearer "+adminToken)
	usersRes := httptest.NewRecorder()
	mux.ServeHTTP(usersRes, usersReq)
	if usersRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics users status: %d", usersRes.Code)
	}

	messagesReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/messages?model_id=model-a", nil)
	messagesReq.Header.Set("Authorization", "Bearer "+adminToken)
	messagesRes := httptest.NewRecorder()
	mux.ServeHTTP(messagesRes, messagesReq)
	if messagesRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics messages status: %d", messagesRes.Code)
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/summary", nil)
	summaryReq.Header.Set("Authorization", "Bearer "+adminToken)
	summaryRes := httptest.NewRecorder()
	mux.ServeHTTP(summaryRes, summaryReq)
	if summaryRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics summary status: %d", summaryRes.Code)
	}
	var summaryPayload map[string]any
	if err := json.Unmarshal(summaryRes.Body.Bytes(), &summaryPayload); err != nil {
		t.Fatal(err)
	}
	if int(summaryPayload["total_messages"].(float64)) != 3 {
		t.Fatalf("unexpected total_messages: %v", summaryPayload["total_messages"])
	}

	dailyReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/daily", nil)
	dailyReq.Header.Set("Authorization", "Bearer "+adminToken)
	dailyRes := httptest.NewRecorder()
	mux.ServeHTTP(dailyRes, dailyReq)
	if dailyRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics daily status: %d", dailyRes.Code)
	}

	tokensReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/tokens", nil)
	tokensReq.Header.Set("Authorization", "Bearer "+adminToken)
	tokensRes := httptest.NewRecorder()
	mux.ServeHTTP(tokensRes, tokensReq)
	if tokensRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics tokens status: %d", tokensRes.Code)
	}

	modelChatsReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/models/model-a/chats", nil)
	modelChatsReq.Header.Set("Authorization", "Bearer "+adminToken)
	modelChatsRes := httptest.NewRecorder()
	mux.ServeHTTP(modelChatsRes, modelChatsReq)
	if modelChatsRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics model chats status: %d", modelChatsRes.Code)
	}
	var modelChatsPayload map[string]any
	if err := json.Unmarshal(modelChatsRes.Body.Bytes(), &modelChatsPayload); err != nil {
		t.Fatal(err)
	}
	if int(modelChatsPayload["total"].(float64)) != 2 {
		t.Fatalf("unexpected model chats total: %v", modelChatsPayload["total"])
	}

	overviewReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/models/model-a/overview?days=7", nil)
	overviewReq.Header.Set("Authorization", "Bearer "+adminToken)
	overviewRes := httptest.NewRecorder()
	mux.ServeHTTP(overviewRes, overviewReq)
	if overviewRes.Code != http.StatusOK {
		t.Fatalf("unexpected analytics overview status: %d", overviewRes.Code)
	}
	var overviewPayload map[string]any
	if err := json.Unmarshal(overviewRes.Body.Bytes(), &overviewPayload); err != nil {
		t.Fatal(err)
	}
	tags, _ := overviewPayload["tags"].([]any)
	if len(tags) == 0 {
		t.Fatal("expected overview tags")
	}
}

func signupAnalyticsUser(t *testing.T, mux *http.ServeMux, name string, email string) (string, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name":              name,
		"email":             email,
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(body))
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("unexpected signup status: %d", res.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	token, _ := payload["token"].(string)
	id, _ := payload["id"].(string)
	return token, id
}

func insertAnalyticsChat(t *testing.T, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, id string, userID string, title string, payload map[string]any, createdAt int64, updatedAt int64) error {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(
		context.Background(),
		`INSERT INTO chat (id, user_id, title, chat, created_at, updated_at, share_id, archived, folder_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		userID,
		title,
		string(body),
		createdAt,
		updatedAt,
		nil,
		false,
		nil,
	)
	return err
}
