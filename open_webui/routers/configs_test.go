package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

type testConfigsStore struct {
	data map[string]any
}

func (s *testConfigsStore) Load(context.Context) (map[string]any, error) {
	if s.data == nil {
		return nil, nil
	}
	return cloneConfigPayload(s.data), nil
}

func (s *testConfigsStore) Save(_ context.Context, data map[string]any) error {
	s.data = cloneConfigPayload(data)
	return nil
}

func TestConfigsRouterImportExportConnections(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	store := &testConfigsStore{}
	state := &ConfigsState{}

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

	configsRouter := &ConfigsRouter{
		Config: ConfigsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			State:          state,
		},
		Users: users,
		Store: store,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	configsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Config User",
		"email":             "config@example.com",
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

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status: %d", exportRes.Code)
	}

	importBody, _ := json.Marshal(map[string]any{
		"config": map[string]any{
			"version": 2,
			"ui": map[string]any{
				"theme": "dark",
			},
			"direct": map[string]any{
				"enable": true,
			},
			"models": map[string]any{
				"base_models_cache": true,
			},
		},
	})
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/import", bytes.NewReader(importBody))
	importReq.Header.Set("Authorization", "Bearer "+token)
	importRes := httptest.NewRecorder()
	mux.ServeHTTP(importRes, importReq)
	if importRes.Code != http.StatusOK {
		t.Fatalf("unexpected import status: %d", importRes.Code)
	}
	if !state.EnableDirectConnections || !state.EnableBaseModelsCache {
		t.Fatal("expected config import to update runtime state")
	}

	connectionsReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/connections", nil)
	connectionsReq.Header.Set("Authorization", "Bearer "+token)
	connectionsRes := httptest.NewRecorder()
	mux.ServeHTTP(connectionsRes, connectionsReq)
	if connectionsRes.Code != http.StatusOK {
		t.Fatalf("unexpected connections status: %d", connectionsRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"ENABLE_DIRECT_CONNECTIONS": false,
		"ENABLE_BASE_MODELS_CACHE":  false,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/connections", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update connections status: %d", updateRes.Code)
	}
	if state.EnableDirectConnections || state.EnableBaseModelsCache {
		t.Fatal("expected connections update to update runtime state")
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after update: %d", exportRes.Code)
	}

	var exportPayload map[string]any
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	direct, ok := getConfigPathValue(exportPayload, "direct.enable")
	if !ok || direct != false {
		t.Fatalf("unexpected direct.enable: %v", direct)
	}
	cache, ok := getConfigPathValue(exportPayload, "models.base_models_cache")
	if !ok || cache != false {
		t.Fatalf("unexpected models.base_models_cache: %v", cache)
	}

	terminalServersBody, _ := json.Marshal(map[string]any{
		"TERMINAL_SERVER_CONNECTIONS": []map[string]any{
			{
				"id":      "term-1",
				"url":     "http://127.0.0.1:19000",
				"name":    "Terminal 1",
				"enabled": true,
				"config": map[string]any{
					"access_grants": []map[string]any{
						{
							"principal_type": "user",
							"principal_id":   "*",
							"permission":     "read",
						},
					},
				},
			},
		},
	})
	terminalServersReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/terminal_servers", bytes.NewReader(terminalServersBody))
	terminalServersReq.Header.Set("Authorization", "Bearer "+token)
	terminalServersRes := httptest.NewRecorder()
	mux.ServeHTTP(terminalServersRes, terminalServersReq)
	if terminalServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal servers update status: %d", terminalServersRes.Code)
	}
	if len(state.TerminalServerConnections) != 1 {
		t.Fatalf("unexpected terminal server state: %d", len(state.TerminalServerConnections))
	}

	terminalServersReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/terminal_servers", nil)
	terminalServersReq.Header.Set("Authorization", "Bearer "+token)
	terminalServersRes = httptest.NewRecorder()
	mux.ServeHTTP(terminalServersRes, terminalServersReq)
	if terminalServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal servers status: %d", terminalServersRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after terminal servers update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	terminalConnections, ok := getConfigPathValue(exportPayload, "terminal_server.connections")
	if !ok {
		t.Fatal("missing terminal_server.connections")
	}
	connections, ok := normalizeConfigList(terminalConnections)
	if !ok || len(connections) != 1 {
		t.Fatalf("unexpected terminal_server.connections: %v", terminalConnections)
	}

	toolServersBody, _ := json.Marshal(map[string]any{
		"TOOL_SERVER_CONNECTIONS": []map[string]any{
			{
				"url":       "http://127.0.0.1:20000",
				"path":      "/openapi.json",
				"type":      "openapi",
				"auth_type": "none",
				"config": map[string]any{
					"access_grants": []map[string]any{
						{
							"principal_type": "user",
							"principal_id":   "*",
							"permission":     "read",
						},
					},
				},
			},
		},
	})
	toolServersReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/tool_servers", bytes.NewReader(toolServersBody))
	toolServersReq.Header.Set("Authorization", "Bearer "+token)
	toolServersRes := httptest.NewRecorder()
	mux.ServeHTTP(toolServersRes, toolServersReq)
	if toolServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool servers update status: %d", toolServersRes.Code)
	}
	if len(state.ToolServerConnections) != 1 {
		t.Fatalf("unexpected tool server state: %d", len(state.ToolServerConnections))
	}

	toolServersReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/tool_servers", nil)
	toolServersReq.Header.Set("Authorization", "Bearer "+token)
	toolServersRes = httptest.NewRecorder()
	mux.ServeHTTP(toolServersRes, toolServersReq)
	if toolServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool servers status: %d", toolServersRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after tool servers update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	toolConnections, ok := getConfigPathValue(exportPayload, "tool_server.connections")
	if !ok {
		t.Fatal("missing tool_server.connections")
	}
	connections, ok = normalizeConfigList(toolConnections)
	if !ok || len(connections) != 1 {
		t.Fatalf("unexpected tool_server.connections: %v", toolConnections)
	}

	suggestionsBody, _ := json.Marshal(map[string]any{
		"suggestions": []map[string]any{
			{
				"title":   []string{"Help me", "study"},
				"content": "test content",
			},
		},
	})
	suggestionsReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/suggestions", bytes.NewReader(suggestionsBody))
	suggestionsReq.Header.Set("Authorization", "Bearer "+token)
	suggestionsRes := httptest.NewRecorder()
	mux.ServeHTTP(suggestionsRes, suggestionsReq)
	if suggestionsRes.Code != http.StatusOK {
		t.Fatalf("unexpected suggestions update status: %d", suggestionsRes.Code)
	}
	if len(state.DefaultPromptSuggestions) != 1 {
		t.Fatalf("unexpected suggestions state: %v", state.DefaultPromptSuggestions)
	}

	bannersBody, _ := json.Marshal(map[string]any{
		"banners": []map[string]any{
			{
				"id":          "banner-1",
				"type":        "info",
				"title":       "hello",
				"content":     "world",
				"dismissible": true,
				"timestamp":   123,
			},
		},
	})
	bannersReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/banners", bytes.NewReader(bannersBody))
	bannersReq.Header.Set("Authorization", "Bearer "+token)
	bannersRes := httptest.NewRecorder()
	mux.ServeHTTP(bannersRes, bannersReq)
	if bannersRes.Code != http.StatusOK {
		t.Fatalf("unexpected banners update status: %d", bannersRes.Code)
	}
	if len(state.Banners) != 1 {
		t.Fatalf("unexpected banners state: %v", state.Banners)
	}

	bannersReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/banners", nil)
	bannersReq.Header.Set("Authorization", "Bearer "+token)
	bannersRes = httptest.NewRecorder()
	mux.ServeHTTP(bannersRes, bannersReq)
	if bannersRes.Code != http.StatusOK {
		t.Fatalf("unexpected banners status: %d", bannersRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after suggestions and banners update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	promptSuggestions, ok := getConfigPathValue(exportPayload, "ui.prompt_suggestions")
	if !ok {
		t.Fatal("missing ui.prompt_suggestions")
	}
	suggestionsPayload, ok := normalizeConfigList(promptSuggestions)
	if !ok || len(suggestionsPayload) != 1 {
		t.Fatalf("unexpected ui.prompt_suggestions: %v", promptSuggestions)
	}
	bannersValue, ok := getConfigPathValue(exportPayload, "ui.banners")
	if !ok {
		t.Fatal("missing ui.banners")
	}
	bannersPayload, ok := normalizeConfigList(bannersValue)
	if !ok || len(bannersPayload) != 1 {
		t.Fatalf("unexpected ui.banners: %v", bannersValue)
	}

	codeExecutionReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/code_execution", nil)
	codeExecutionReq.Header.Set("Authorization", "Bearer "+token)
	codeExecutionRes := httptest.NewRecorder()
	mux.ServeHTTP(codeExecutionRes, codeExecutionReq)
	if codeExecutionRes.Code != http.StatusOK {
		t.Fatalf("unexpected code execution status: %d", codeExecutionRes.Code)
	}

	codeExecutionBody, _ := json.Marshal(map[string]any{
		"ENABLE_CODE_EXECUTION":                  false,
		"CODE_EXECUTION_ENGINE":                  "jupyter",
		"CODE_EXECUTION_JUPYTER_URL":             "http://127.0.0.1:8888",
		"CODE_EXECUTION_JUPYTER_AUTH":            "token",
		"CODE_EXECUTION_JUPYTER_AUTH_TOKEN":      "exec-token",
		"CODE_EXECUTION_JUPYTER_AUTH_PASSWORD":   "exec-password",
		"CODE_EXECUTION_JUPYTER_TIMEOUT":         120,
		"ENABLE_CODE_INTERPRETER":                true,
		"CODE_INTERPRETER_ENGINE":                "jupyter",
		"CODE_INTERPRETER_PROMPT_TEMPLATE":       "prompt",
		"CODE_INTERPRETER_JUPYTER_URL":           "http://127.0.0.1:9999",
		"CODE_INTERPRETER_JUPYTER_AUTH":          "password",
		"CODE_INTERPRETER_JUPYTER_AUTH_TOKEN":    "interp-token",
		"CODE_INTERPRETER_JUPYTER_AUTH_PASSWORD": "interp-password",
		"CODE_INTERPRETER_JUPYTER_TIMEOUT":       240,
	})
	codeExecutionReq = httptest.NewRequest(http.MethodPost, "/api/v1/configs/code_execution", bytes.NewReader(codeExecutionBody))
	codeExecutionReq.Header.Set("Authorization", "Bearer "+token)
	codeExecutionRes = httptest.NewRecorder()
	mux.ServeHTTP(codeExecutionRes, codeExecutionReq)
	if codeExecutionRes.Code != http.StatusOK {
		t.Fatalf("unexpected code execution update status: %d", codeExecutionRes.Code)
	}
	if state.CodeExecutionEngine != "jupyter" || state.CodeInterpreterJupyterTimeout != 240 {
		t.Fatalf("unexpected code execution state: %+v", state)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after code execution update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	codeExecutionEngine, ok := getConfigPathValue(exportPayload, "code_execution.engine")
	if !ok || codeExecutionEngine != "jupyter" {
		t.Fatalf("unexpected code_execution.engine: %v", codeExecutionEngine)
	}
	codeInterpreterPrompt, ok := getConfigPathValue(exportPayload, "code_interpreter.prompt_template")
	if !ok || codeInterpreterPrompt != "prompt" {
		t.Fatalf("unexpected code_interpreter.prompt_template: %v", codeInterpreterPrompt)
	}
	modelDefaultsReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/models/defaults", nil)
	modelDefaultsReq.Header.Set("Authorization", "Bearer "+token)
	modelDefaultsRes := httptest.NewRecorder()
	mux.ServeHTTP(modelDefaultsRes, modelDefaultsReq)
	if modelDefaultsRes.Code != http.StatusOK {
		t.Fatalf("unexpected model defaults status: %d", modelDefaultsRes.Code)
	}

	modelsBody, _ := json.Marshal(map[string]any{
		"DEFAULT_MODELS":        "model-a",
		"DEFAULT_PINNED_MODELS": "model-b",
		"MODEL_ORDER_LIST":      []string{"model-a", "model-b"},
		"DEFAULT_MODEL_METADATA": map[string]any{
			"temperature": 0.7,
		},
		"DEFAULT_MODEL_PARAMS": map[string]any{
			"top_p": 0.9,
		},
	})
	modelsReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/models", bytes.NewReader(modelsBody))
	modelsReq.Header.Set("Authorization", "Bearer "+token)
	modelsRes := httptest.NewRecorder()
	mux.ServeHTTP(modelsRes, modelsReq)
	if modelsRes.Code != http.StatusOK {
		t.Fatalf("unexpected models config update status: %d", modelsRes.Code)
	}
	if state.DefaultModels != "model-a" || state.DefaultPinnedModels != "model-b" {
		t.Fatalf("unexpected models state: %+v", state)
	}
	if len(state.ModelOrderList) != 2 {
		t.Fatalf("unexpected model order list: %v", state.ModelOrderList)
	}

	modelsReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/models", nil)
	modelsReq.Header.Set("Authorization", "Bearer "+token)
	modelsRes = httptest.NewRecorder()
	mux.ServeHTTP(modelsRes, modelsReq)
	if modelsRes.Code != http.StatusOK {
		t.Fatalf("unexpected models config status: %d", modelsRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after models config update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	defaultModels, ok := getConfigPathValue(exportPayload, "ui.default_models")
	if !ok || defaultModels != "model-a" {
		t.Fatalf("unexpected ui.default_models: %v", defaultModels)
	}
	orderList, ok := getConfigPathValue(exportPayload, "ui.model_order_list")
	if !ok {
		t.Fatal("missing ui.model_order_list")
	}
	modelOrderList, ok := normalizeConfigStringList(orderList)
	if !ok || len(modelOrderList) != 2 {
		t.Fatalf("unexpected ui.model_order_list: %v", orderList)
	}
	defaultMetadata, ok := getConfigPathValue(exportPayload, "models.default_metadata")
	if !ok {
		t.Fatal("missing models.default_metadata")
	}
	metadataPayload, ok := normalizeConfigMap(defaultMetadata)
	if !ok || metadataPayload["temperature"] != 0.7 {
		t.Fatalf("unexpected models.default_metadata: %v", defaultMetadata)
	}
}

func TestConfigsRouterVerifyEndpoints(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)

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

	configsRouter := &ConfigsRouter{
		Config: ConfigsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			State:          &ConfigsState{},
		},
		Users: users,
		Store: &testConfigsStore{},
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	configsRouter.Register(mux)

	adminToken, _ := signupAnalyticsUser(t, mux, "Configs Admin", "configs-admin@example.com")

	terminalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer terminal-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/policies":
			writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/policies/policy-1":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			writeJSON(w, http.StatusOK, map[string]any{"saved": true, "payload": payload})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer terminalServer.Close()

	toolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tool-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/openapi.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title": "Tool API",
			},
		})
	}))
	defer toolServer.Close()

	terminalVerifyBody, _ := json.Marshal(map[string]any{
		"url":       terminalServer.URL,
		"key":       "terminal-key",
		"auth_type": "bearer",
	})
	terminalVerifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/terminal_servers/verify", bytes.NewReader(terminalVerifyBody))
	terminalVerifyReq.Header.Set("Authorization", "Bearer "+adminToken)
	terminalVerifyRes := httptest.NewRecorder()
	mux.ServeHTTP(terminalVerifyRes, terminalVerifyReq)
	if terminalVerifyRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal verify status: %d", terminalVerifyRes.Code)
	}

	policyBody, _ := json.Marshal(map[string]any{
		"url":         terminalServer.URL,
		"key":         "terminal-key",
		"auth_type":   "bearer",
		"policy_id":   "policy-1",
		"policy_data": map[string]any{"name": "example"},
	})
	policyReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/terminal_servers/policy", bytes.NewReader(policyBody))
	policyReq.Header.Set("Authorization", "Bearer "+adminToken)
	policyRes := httptest.NewRecorder()
	mux.ServeHTTP(policyRes, policyReq)
	if policyRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal policy status: %d", policyRes.Code)
	}

	toolVerifyBody, _ := json.Marshal(map[string]any{
		"url":       toolServer.URL,
		"path":      "/openapi.json",
		"type":      "openapi",
		"auth_type": "bearer",
		"key":       "tool-key",
	})
	toolVerifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/tool_servers/verify", bytes.NewReader(toolVerifyBody))
	toolVerifyReq.Header.Set("Authorization", "Bearer "+adminToken)
	toolVerifyRes := httptest.NewRecorder()
	mux.ServeHTTP(toolVerifyRes, toolVerifyReq)
	if toolVerifyRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool verify status: %d", toolVerifyRes.Code)
	}
}
