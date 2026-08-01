package routers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func TestTasksRouterConfigAndActiveChats(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	store := &testConfigsStore{}

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

	tasksRouter := &TasksRouter{
		Config: TasksRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			State: &TasksState{
				EnableTitleGeneration:                true,
				EnableFollowUpGeneration:             true,
				EnableTagsGeneration:                 true,
				EnableSearchQueryGeneration:          true,
				EnableRetrievalQueryGeneration:       true,
				AutocompleteGenerationInputMaxLength: -1,
			},
			Store: store,
			ActiveChatsFunc: func(chatIDs []string) []string {
				if len(chatIDs) > 1 {
					return chatIDs[:1]
				}
				return chatIDs
			},
		},
		Users: users,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	tasksRouter.Register(mux)

	adminToken, _ := signupAnalyticsUser(t, mux, "Task Admin", "task-admin@example.com")
	userToken, _ := signupAnalyticsUser(t, mux, "Task User", "task-user@example.com")

	configReq := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/config", nil)
	configReq.Header.Set("Authorization", "Bearer "+userToken)
	configRes := httptest.NewRecorder()
	mux.ServeHTTP(configRes, configReq)
	if configRes.Code != http.StatusOK {
		t.Fatalf("unexpected tasks config status: %d", configRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"TASK_MODEL":                               "model-a",
		"TASK_MODEL_EXTERNAL":                      "external-a",
		"ENABLE_TITLE_GENERATION":                  true,
		"TITLE_GENERATION_PROMPT_TEMPLATE":         "title-template",
		"IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE":  "image-template",
		"ENABLE_AUTOCOMPLETE_GENERATION":           true,
		"AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH": 128,
		"TAGS_GENERATION_PROMPT_TEMPLATE":          "tags-template",
		"FOLLOW_UP_GENERATION_PROMPT_TEMPLATE":     "follow-up-template",
		"ENABLE_FOLLOW_UP_GENERATION":              true,
		"ENABLE_TAGS_GENERATION":                   true,
		"ENABLE_SEARCH_QUERY_GENERATION":           true,
		"ENABLE_RETRIEVAL_QUERY_GENERATION":        true,
		"QUERY_GENERATION_PROMPT_TEMPLATE":         "query-template",
		"TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE":   "tool-template",
		"VOICE_MODE_PROMPT_TEMPLATE":               "voice-template",
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/config/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+adminToken)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected tasks config update status: %d", updateRes.Code)
	}

	configReq = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/config", nil)
	configReq.Header.Set("Authorization", "Bearer "+userToken)
	configRes = httptest.NewRecorder()
	mux.ServeHTTP(configRes, configReq)
	if configRes.Code != http.StatusOK {
		t.Fatalf("unexpected tasks config status after update: %d", configRes.Code)
	}
	var configPayload map[string]any
	if err := json.Unmarshal(configRes.Body.Bytes(), &configPayload); err != nil {
		t.Fatal(err)
	}
	if configPayload["TASK_MODEL"] != "model-a" {
		t.Fatalf("unexpected TASK_MODEL: %v", configPayload["TASK_MODEL"])
	}
	value, ok := getConfigPathValue(store.data, "task.tools.function_calling.prompt_template")
	if !ok || value != "tool-template" {
		t.Fatalf("unexpected persisted tool prompt: %v", value)
	}

	activeBody, _ := json.Marshal(map[string]any{
		"chat_ids": []string{"chat-1", "chat-2"},
	})
	activeReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/active/chats", bytes.NewReader(activeBody))
	activeReq.Header.Set("Authorization", "Bearer "+userToken)
	activeRes := httptest.NewRecorder()
	mux.ServeHTTP(activeRes, activeReq)
	if activeRes.Code != http.StatusOK {
		t.Fatalf("unexpected active chats status: %d", activeRes.Code)
	}
	var activePayload map[string]any
	if err := json.Unmarshal(activeRes.Body.Bytes(), &activePayload); err != nil {
		t.Fatal(err)
	}
	chatIDs, _ := activePayload["active_chat_ids"].([]any)
	if len(chatIDs) != 1 {
		t.Fatalf("unexpected active chat ids: %v", activePayload["active_chat_ids"])
	}

	completionBody, _ := json.Marshal(map[string]any{
		"model": "model-a",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "How do I optimize query performance for a large table?",
			},
		},
		"prompt": "How do I optimize query performance for a large table?",
		"type":   "web_search",
	})

	titleReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/title/completions", bytes.NewReader(completionBody))
	titleReq.Header.Set("Authorization", "Bearer "+userToken)
	titleRes := httptest.NewRecorder()
	mux.ServeHTTP(titleRes, titleReq)
	if titleRes.Code != http.StatusOK {
		t.Fatalf("unexpected title completion status: %d", titleRes.Code)
	}

	followReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/follow_ups/completions", bytes.NewReader(completionBody))
	followReq.Header.Set("Authorization", "Bearer "+userToken)
	followRes := httptest.NewRecorder()
	mux.ServeHTTP(followRes, followReq)
	if followRes.Code != http.StatusOK {
		t.Fatalf("unexpected follow ups status: %d", followRes.Code)
	}

	tagsReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/tags/completions", bytes.NewReader(completionBody))
	tagsReq.Header.Set("Authorization", "Bearer "+userToken)
	tagsRes := httptest.NewRecorder()
	mux.ServeHTTP(tagsRes, tagsReq)
	if tagsRes.Code != http.StatusOK {
		t.Fatalf("unexpected tags status: %d", tagsRes.Code)
	}

	emojiReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/emoji/completions", bytes.NewReader(completionBody))
	emojiReq.Header.Set("Authorization", "Bearer "+userToken)
	emojiRes := httptest.NewRecorder()
	mux.ServeHTTP(emojiRes, emojiReq)
	if emojiRes.Code != http.StatusOK {
		t.Fatalf("unexpected emoji status: %d", emojiRes.Code)
	}

	queriesReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/queries/completions", bytes.NewReader(completionBody))
	queriesReq.Header.Set("Authorization", "Bearer "+userToken)
	queriesRes := httptest.NewRecorder()
	mux.ServeHTTP(queriesRes, queriesReq)
	if queriesRes.Code != http.StatusOK {
		t.Fatalf("unexpected queries status: %d", queriesRes.Code)
	}

	autoReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/auto/completions", bytes.NewReader(completionBody))
	autoReq.Header.Set("Authorization", "Bearer "+userToken)
	autoRes := httptest.NewRecorder()
	mux.ServeHTTP(autoRes, autoReq)
	if autoRes.Code != http.StatusOK {
		t.Fatalf("unexpected auto completion status: %d", autoRes.Code)
	}

	var titlePayload map[string]any
	if err := json.Unmarshal(titleRes.Body.Bytes(), &titlePayload); err != nil {
		t.Fatal(err)
	}
	choices, _ := titlePayload["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("unexpected title payload: %v", titlePayload)
	}

	imagePromptReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/image_prompt/completions", bytes.NewReader(completionBody))
	imagePromptReq.Header.Set("Authorization", "Bearer "+userToken)
	imagePromptRes := httptest.NewRecorder()
	mux.ServeHTTP(imagePromptRes, imagePromptReq)
	if imagePromptRes.Code != http.StatusOK {
		t.Fatalf("unexpected image prompt status: %d", imagePromptRes.Code)
	}

	moaBody, _ := json.Marshal(map[string]any{
		"model":     "model-a",
		"prompt":    "Compare these answers",
		"responses": []string{"first response", "second response"},
		"stream":    true,
	})
	moaReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/moa/completions", bytes.NewReader(moaBody))
	moaReq.Header.Set("Authorization", "Bearer "+userToken)
	moaRes := httptest.NewRecorder()
	mux.ServeHTTP(moaRes, moaReq)
	if moaRes.Code != http.StatusOK {
		t.Fatalf("unexpected moa status: %d", moaRes.Code)
	}
	if contentType := moaRes.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("unexpected moa content type: %s", contentType)
	}
	if !strings.Contains(moaRes.Body.String(), "data: [DONE]") {
		t.Fatalf("unexpected moa body: %s", moaRes.Body.String())
	}
}
