package routers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type ConfigsStore interface {
	Load(ctx context.Context) (map[string]any, error)
	Save(ctx context.Context, data map[string]any) error
}

type ConfigsState struct {
	EnableDirectConnections            bool
	EnableBaseModelsCache              bool
	ToolServerConnections              []map[string]any
	TerminalServerConnections          []map[string]any
	DefaultPromptSuggestions           []map[string]any
	Banners                            []map[string]any
	DefaultModels                      string
	DefaultPinnedModels                string
	ModelOrderList                     []string
	DefaultModelMetadata               map[string]any
	DefaultModelParams                 map[string]any
	EnableCodeExecution                bool
	CodeExecutionEngine                string
	CodeExecutionJupyterURL            string
	CodeExecutionJupyterAuth           string
	CodeExecutionJupyterAuthToken      string
	CodeExecutionJupyterAuthPassword   string
	CodeExecutionJupyterTimeout        int
	EnableCodeInterpreter              bool
	CodeInterpreterEngine              string
	CodeInterpreterPromptTemplate      string
	CodeInterpreterJupyterURL          string
	CodeInterpreterJupyterAuth         string
	CodeInterpreterJupyterAuthToken    string
	CodeInterpreterJupyterAuthPassword string
	CodeInterpreterJupyterTimeout      int
}

type ConfigsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
	State          *ConfigsState
	HTTPClient     *http.Client
}

type ConfigsRouter struct {
	Config ConfigsRuntimeConfig
	Users  *models.UsersTable
	Store  ConfigsStore
}

type importConfigForm struct {
	Config map[string]any `json:"config"`
}

type connectionsConfigForm struct {
	EnableDirectConnections bool `json:"ENABLE_DIRECT_CONNECTIONS"`
	EnableBaseModelsCache   bool `json:"ENABLE_BASE_MODELS_CACHE"`
}

type terminalServersConfigForm struct {
	TerminalServerConnections []map[string]any `json:"TERMINAL_SERVER_CONNECTIONS"`
}

type toolServersConfigForm struct {
	ToolServerConnections []map[string]any `json:"TOOL_SERVER_CONNECTIONS"`
}

type suggestionsConfigForm struct {
	Suggestions []map[string]any `json:"suggestions"`
}

type bannersConfigForm struct {
	Banners []map[string]any `json:"banners"`
}

type codeExecutionConfigForm struct {
	EnableCodeExecution                bool    `json:"ENABLE_CODE_EXECUTION"`
	CodeExecutionEngine                string  `json:"CODE_EXECUTION_ENGINE"`
	CodeExecutionJupyterURL            *string `json:"CODE_EXECUTION_JUPYTER_URL"`
	CodeExecutionJupyterAuth           *string `json:"CODE_EXECUTION_JUPYTER_AUTH"`
	CodeExecutionJupyterAuthToken      *string `json:"CODE_EXECUTION_JUPYTER_AUTH_TOKEN"`
	CodeExecutionJupyterAuthPassword   *string `json:"CODE_EXECUTION_JUPYTER_AUTH_PASSWORD"`
	CodeExecutionJupyterTimeout        *int    `json:"CODE_EXECUTION_JUPYTER_TIMEOUT"`
	EnableCodeInterpreter              bool    `json:"ENABLE_CODE_INTERPRETER"`
	CodeInterpreterEngine              string  `json:"CODE_INTERPRETER_ENGINE"`
	CodeInterpreterPromptTemplate      *string `json:"CODE_INTERPRETER_PROMPT_TEMPLATE"`
	CodeInterpreterJupyterURL          *string `json:"CODE_INTERPRETER_JUPYTER_URL"`
	CodeInterpreterJupyterAuth         *string `json:"CODE_INTERPRETER_JUPYTER_AUTH"`
	CodeInterpreterJupyterAuthToken    *string `json:"CODE_INTERPRETER_JUPYTER_AUTH_TOKEN"`
	CodeInterpreterJupyterAuthPassword *string `json:"CODE_INTERPRETER_JUPYTER_AUTH_PASSWORD"`
	CodeInterpreterJupyterTimeout      *int    `json:"CODE_INTERPRETER_JUPYTER_TIMEOUT"`
}

type modelsConfigForm struct {
	DefaultModels        *string        `json:"DEFAULT_MODELS"`
	DefaultPinnedModels  *string        `json:"DEFAULT_PINNED_MODELS"`
	ModelOrderList       []string       `json:"MODEL_ORDER_LIST"`
	DefaultModelMetadata map[string]any `json:"DEFAULT_MODEL_METADATA"`
	DefaultModelParams   map[string]any `json:"DEFAULT_MODEL_PARAMS"`
}
type terminalServerConnectionForm struct {
	URL      string         `json:"url"`
	Key      string         `json:"key"`
	AuthType string         `json:"auth_type"`
	Config   map[string]any `json:"config,omitempty"`
}

type terminalServerPolicyForm struct {
	URL        string         `json:"url"`
	Key        string         `json:"key"`
	AuthType   string         `json:"auth_type"`
	PolicyID   string         `json:"policy_id"`
	PolicyData map[string]any `json:"policy_data"`
}

type toolServerConnectionForm struct {
	URL      string         `json:"url"`
	Path     string         `json:"path"`
	Type     string         `json:"type"`
	AuthType string         `json:"auth_type"`
	Key      string         `json:"key"`
	Headers  map[string]any `json:"headers,omitempty"`
	Config   map[string]any `json:"config,omitempty"`
}

func (h *ConfigsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/configs/import", h.ImportConfig)
	mux.HandleFunc("GET /api/v1/configs/export", h.ExportConfig)
	mux.HandleFunc("GET /api/v1/configs/connections", h.GetConnectionsConfig)
	mux.HandleFunc("POST /api/v1/configs/connections", h.SetConnectionsConfig)
	mux.HandleFunc("GET /api/v1/configs/tool_servers", h.GetToolServersConfig)
	mux.HandleFunc("POST /api/v1/configs/tool_servers", h.SetToolServersConfig)
	mux.HandleFunc("POST /api/v1/configs/tool_servers/verify", h.VerifyToolServersConfig)
	mux.HandleFunc("GET /api/v1/configs/terminal_servers", h.GetTerminalServersConfig)
	mux.HandleFunc("POST /api/v1/configs/terminal_servers", h.SetTerminalServersConfig)
	mux.HandleFunc("POST /api/v1/configs/terminal_servers/verify", h.VerifyTerminalServerConnection)
	mux.HandleFunc("POST /api/v1/configs/terminal_servers/policy", h.PutTerminalServerPolicy)
	mux.HandleFunc("POST /api/v1/configs/suggestions", h.SetDefaultSuggestions)
	mux.HandleFunc("POST /api/v1/configs/banners", h.SetBanners)
	mux.HandleFunc("GET /api/v1/configs/banners", h.GetBanners)
	mux.HandleFunc("GET /api/v1/configs/code_execution", h.GetCodeExecutionConfig)
	mux.HandleFunc("POST /api/v1/configs/code_execution", h.SetCodeExecutionConfig)
	mux.HandleFunc("GET /api/v1/configs/models/defaults", h.GetModelsDefaults)
	mux.HandleFunc("GET /api/v1/configs/models", h.GetModelsConfig)
	mux.HandleFunc("POST /api/v1/configs/models", h.SetModelsConfig)
}

func (h *ConfigsRouter) ImportConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form importConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil || form.Config == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData := cloneConfigPayload(form.Config)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		loaded, err := h.loadConfig(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		h.applyConfig(loaded)
		writeJSON(w, http.StatusOK, loaded)
		return
	}

	h.applyConfig(configData)
	writeJSON(w, http.StatusOK, configData)
}

func (h *ConfigsRouter) ExportConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, configData)
}

func (h *ConfigsRouter) GetConnectionsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.connectionsResponse())
}

func (h *ConfigsRouter) SetConnectionsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form connectionsConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "direct.enable", form.EnableDirectConnections)
	setConfigPathValue(configData, "models.base_models_cache", form.EnableBaseModelsCache)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	state := h.state()
	state.EnableDirectConnections = form.EnableDirectConnections
	state.EnableBaseModelsCache = form.EnableBaseModelsCache
	writeJSON(w, http.StatusOK, h.connectionsResponse())
}

func (h *ConfigsRouter) GetTerminalServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.terminalServersResponse())
}

func (h *ConfigsRouter) GetToolServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.toolServersResponse())
}

func (h *ConfigsRouter) SetToolServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form toolServersConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "tool_server.connections", form.ToolServerConnections)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	h.state().ToolServerConnections = cloneConfigList(form.ToolServerConnections)
	writeJSON(w, http.StatusOK, h.toolServersResponse())
}

func (h *ConfigsRouter) SetTerminalServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form terminalServersConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "terminal_server.connections", form.TerminalServerConnections)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	h.state().TerminalServerConnections = cloneConfigList(form.TerminalServerConnections)
	writeJSON(w, http.StatusOK, h.terminalServersResponse())
}

func (h *ConfigsRouter) VerifyTerminalServerConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form terminalServerConnectionForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(form.URL), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Terminal server URL is required"})
		return
	}
	headers := buildConfigsBearerHeaders(form.AuthType, form.Key, nil)
	if h.verifyConfigsGet(baseURL+"/api/v1/policies", headers) {
		writeJSON(w, http.StatusOK, map[string]any{"status": true, "type": "orchestrator"})
		return
	}
	if h.verifyConfigsGet(baseURL+"/api/config", headers) {
		writeJSON(w, http.StatusOK, map[string]any{"status": true, "type": "terminal"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the terminal server"})
}

func (h *ConfigsRouter) PutTerminalServerPolicy(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form terminalServerPolicyForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(form.URL), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Terminal server URL is required"})
		return
	}
	body, err := json.Marshal(form.PolicyData)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid policy data"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPut, baseURL+"/api/v1/policies/"+url.PathEscape(form.PolicyID), strings.NewReader(string(body)))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	req.Header = buildConfigsBearerHeaders(form.AuthType, form.Key, map[string]string{"Content-Type": "application/json"})
	resp, err := h.httpClient().Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to save policy to terminal server"})
		return
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to save policy to terminal server"})
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, resp.StatusCode, map[string]string{"detail": strings.TrimSpace(string(payload))})
		return
	}
	var responsePayload any
	if err := json.Unmarshal(payload, &responsePayload); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"detail": strings.TrimSpace(string(payload))})
		return
	}
	writeJSON(w, http.StatusOK, responsePayload)
}

func (h *ConfigsRouter) VerifyToolServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form toolServerConnectionForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if strings.TrimSpace(form.Type) == "mcp" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(form.URL), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	path := strings.TrimSpace(form.Path)
	if path == "" {
		path = "/openapi.json"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, baseURL+path, nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	req.Header = buildConfigsBearerHeaders(form.AuthType, form.Key, normalizeConfigsHeaderValues(form.Headers))
	resp, err := h.httpClient().Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to connect to the tool server"})
		return
	}
	var responsePayload any
	if err := json.Unmarshal(payload, &responsePayload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": true})
		return
	}
	writeJSON(w, http.StatusOK, responsePayload)
}
func (h *ConfigsRouter) SetDefaultSuggestions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form suggestionsConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	h.state().DefaultPromptSuggestions = cloneConfigList(form.Suggestions)
	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "ui.prompt_suggestions", h.state().DefaultPromptSuggestions)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, cloneConfigList(h.state().DefaultPromptSuggestions))
}

func (h *ConfigsRouter) SetBanners(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form bannersConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	h.state().Banners = cloneConfigList(form.Banners)
	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "ui.banners", h.state().Banners)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, cloneConfigList(h.state().Banners))
}

func (h *ConfigsRouter) GetBanners(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, cloneConfigList(h.state().Banners))
}

func (h *ConfigsRouter) GetCodeExecutionConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.codeExecutionConfigResponse())
}

func (h *ConfigsRouter) SetCodeExecutionConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form codeExecutionConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	state := h.state()
	state.EnableCodeExecution = form.EnableCodeExecution
	state.CodeExecutionEngine = form.CodeExecutionEngine
	state.CodeExecutionJupyterURL = optionalStringValue(form.CodeExecutionJupyterURL)
	state.CodeExecutionJupyterAuth = optionalStringValue(form.CodeExecutionJupyterAuth)
	state.CodeExecutionJupyterAuthToken = optionalStringValue(form.CodeExecutionJupyterAuthToken)
	state.CodeExecutionJupyterAuthPassword = optionalStringValue(form.CodeExecutionJupyterAuthPassword)
	state.CodeExecutionJupyterTimeout = optionalIntValue(form.CodeExecutionJupyterTimeout)
	state.EnableCodeInterpreter = form.EnableCodeInterpreter
	state.CodeInterpreterEngine = form.CodeInterpreterEngine
	state.CodeInterpreterPromptTemplate = optionalStringValue(form.CodeInterpreterPromptTemplate)
	state.CodeInterpreterJupyterURL = optionalStringValue(form.CodeInterpreterJupyterURL)
	state.CodeInterpreterJupyterAuth = optionalStringValue(form.CodeInterpreterJupyterAuth)
	state.CodeInterpreterJupyterAuthToken = optionalStringValue(form.CodeInterpreterJupyterAuthToken)
	state.CodeInterpreterJupyterAuthPassword = optionalStringValue(form.CodeInterpreterJupyterAuthPassword)
	state.CodeInterpreterJupyterTimeout = optionalIntValue(form.CodeInterpreterJupyterTimeout)

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "code_execution.enable", state.EnableCodeExecution)
	setConfigPathValue(configData, "code_execution.engine", state.CodeExecutionEngine)
	setConfigPathValue(configData, "code_execution.jupyter.url", state.CodeExecutionJupyterURL)
	setConfigPathValue(configData, "code_execution.jupyter.auth", state.CodeExecutionJupyterAuth)
	setConfigPathValue(configData, "code_execution.jupyter.auth_token", state.CodeExecutionJupyterAuthToken)
	setConfigPathValue(configData, "code_execution.jupyter.auth_password", state.CodeExecutionJupyterAuthPassword)
	setConfigPathValue(configData, "code_execution.jupyter.timeout", state.CodeExecutionJupyterTimeout)
	setConfigPathValue(configData, "code_interpreter.enable", state.EnableCodeInterpreter)
	setConfigPathValue(configData, "code_interpreter.engine", state.CodeInterpreterEngine)
	setConfigPathValue(configData, "code_interpreter.prompt_template", state.CodeInterpreterPromptTemplate)
	setConfigPathValue(configData, "code_interpreter.jupyter.url", state.CodeInterpreterJupyterURL)
	setConfigPathValue(configData, "code_interpreter.jupyter.auth", state.CodeInterpreterJupyterAuth)
	setConfigPathValue(configData, "code_interpreter.jupyter.auth_token", state.CodeInterpreterJupyterAuthToken)
	setConfigPathValue(configData, "code_interpreter.jupyter.auth_password", state.CodeInterpreterJupyterAuthPassword)
	setConfigPathValue(configData, "code_interpreter.jupyter.timeout", state.CodeInterpreterJupyterTimeout)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, h.codeExecutionConfigResponse())
}

func (h *ConfigsRouter) GetModelsDefaults(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"DEFAULT_MODEL_METADATA": cloneConfigMap(h.state().DefaultModelMetadata),
	})
}

func (h *ConfigsRouter) GetModelsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.modelsConfigResponse())
}

func (h *ConfigsRouter) SetModelsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form modelsConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	state := h.state()
	state.DefaultModels = optionalStringValue(form.DefaultModels)
	state.DefaultPinnedModels = optionalStringValue(form.DefaultPinnedModels)
	state.ModelOrderList = cloneConfigStringList(form.ModelOrderList)
	state.DefaultModelMetadata = cloneConfigMap(form.DefaultModelMetadata)
	state.DefaultModelParams = cloneConfigMap(form.DefaultModelParams)

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "ui.default_models", state.DefaultModels)
	setConfigPathValue(configData, "ui.default_pinned_models", state.DefaultPinnedModels)
	setConfigPathValue(configData, "ui.model_order_list", state.ModelOrderList)
	setConfigPathValue(configData, "models.default_metadata", state.DefaultModelMetadata)
	setConfigPathValue(configData, "models.default_params", state.DefaultModelParams)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, h.modelsConfigResponse())
}
func (h *ConfigsRouter) loadConfig(ctx context.Context) (map[string]any, error) {
	if h.Store == nil {
		return defaultConfigPayload(), nil
	}
	configData, err := h.Store.Load(ctx)
	if err != nil {
		return nil, err
	}
	if configData == nil {
		return defaultConfigPayload(), nil
	}
	return cloneConfigPayload(configData), nil
}

func (h *ConfigsRouter) applyConfig(configData map[string]any) {
	state := h.state()
	if value, ok := getConfigPathValue(configData, "direct.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableDirectConnections = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "models.base_models_cache"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableBaseModelsCache = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "tool_server.connections"); ok {
		if connections, typeOK := normalizeConfigList(value); typeOK {
			state.ToolServerConnections = connections
		}
	}
	if value, ok := getConfigPathValue(configData, "terminal_server.connections"); ok {
		if connections, typeOK := normalizeConfigList(value); typeOK {
			state.TerminalServerConnections = connections
		}
	}
	if value, ok := getConfigPathValue(configData, "ui.prompt_suggestions"); ok {
		if suggestions, typeOK := normalizeConfigList(value); typeOK {
			state.DefaultPromptSuggestions = suggestions
		}
	}
	if value, ok := getConfigPathValue(configData, "ui.banners"); ok {
		if banners, typeOK := normalizeConfigList(value); typeOK {
			state.Banners = banners
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableCodeExecution = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.engine"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeExecutionEngine = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.jupyter.url"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeExecutionJupyterURL = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.jupyter.auth"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeExecutionJupyterAuth = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.jupyter.auth_token"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeExecutionJupyterAuthToken = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.jupyter.auth_password"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeExecutionJupyterAuthPassword = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_execution.jupyter.timeout"); ok {
		if timeout, typeOK := normalizeConfigInt(value); typeOK {
			state.CodeExecutionJupyterTimeout = timeout
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableCodeInterpreter = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.engine"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterEngine = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterPromptTemplate = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.jupyter.url"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterJupyterURL = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.jupyter.auth"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterJupyterAuth = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.jupyter.auth_token"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterJupyterAuthToken = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.jupyter.auth_password"); ok {
		if text, typeOK := value.(string); typeOK {
			state.CodeInterpreterJupyterAuthPassword = text
		}
	}
	if value, ok := getConfigPathValue(configData, "code_interpreter.jupyter.timeout"); ok {
		if timeout, typeOK := normalizeConfigInt(value); typeOK {
			state.CodeInterpreterJupyterTimeout = timeout
		}
	}
	if value, ok := getConfigPathValue(configData, "ui.default_models"); ok {
		if text, typeOK := value.(string); typeOK {
			state.DefaultModels = text
		}
	}
	if value, ok := getConfigPathValue(configData, "ui.default_pinned_models"); ok {
		if text, typeOK := value.(string); typeOK {
			state.DefaultPinnedModels = text
		}
	}
	if value, ok := getConfigPathValue(configData, "ui.model_order_list"); ok {
		if items, typeOK := normalizeConfigStringList(value); typeOK {
			state.ModelOrderList = items
		}
	}
	if value, ok := getConfigPathValue(configData, "models.default_metadata"); ok {
		if payload, typeOK := normalizeConfigMap(value); typeOK {
			state.DefaultModelMetadata = payload
		}
	}
	if value, ok := getConfigPathValue(configData, "models.default_params"); ok {
		if payload, typeOK := normalizeConfigMap(value); typeOK {
			state.DefaultModelParams = payload
		}
	}
}

func (h *ConfigsRouter) connectionsResponse() connectionsConfigForm {
	state := h.state()
	return connectionsConfigForm{
		EnableDirectConnections: state.EnableDirectConnections,
		EnableBaseModelsCache:   state.EnableBaseModelsCache,
	}
}

func (h *ConfigsRouter) terminalServersResponse() terminalServersConfigForm {
	return terminalServersConfigForm{
		TerminalServerConnections: cloneConfigList(h.state().TerminalServerConnections),
	}
}

func (h *ConfigsRouter) toolServersResponse() toolServersConfigForm {
	return toolServersConfigForm{
		ToolServerConnections: cloneConfigList(h.state().ToolServerConnections),
	}
}

func (h *ConfigsRouter) codeExecutionConfigResponse() codeExecutionConfigForm {
	state := h.state()
	return codeExecutionConfigForm{
		EnableCodeExecution:                state.EnableCodeExecution,
		CodeExecutionEngine:                state.CodeExecutionEngine,
		CodeExecutionJupyterURL:            optionalConfigString(state.CodeExecutionJupyterURL),
		CodeExecutionJupyterAuth:           optionalConfigString(state.CodeExecutionJupyterAuth),
		CodeExecutionJupyterAuthToken:      optionalConfigString(state.CodeExecutionJupyterAuthToken),
		CodeExecutionJupyterAuthPassword:   optionalConfigString(state.CodeExecutionJupyterAuthPassword),
		CodeExecutionJupyterTimeout:        optionalConfigInt(state.CodeExecutionJupyterTimeout),
		EnableCodeInterpreter:              state.EnableCodeInterpreter,
		CodeInterpreterEngine:              state.CodeInterpreterEngine,
		CodeInterpreterPromptTemplate:      optionalConfigString(state.CodeInterpreterPromptTemplate),
		CodeInterpreterJupyterURL:          optionalConfigString(state.CodeInterpreterJupyterURL),
		CodeInterpreterJupyterAuth:         optionalConfigString(state.CodeInterpreterJupyterAuth),
		CodeInterpreterJupyterAuthToken:    optionalConfigString(state.CodeInterpreterJupyterAuthToken),
		CodeInterpreterJupyterAuthPassword: optionalConfigString(state.CodeInterpreterJupyterAuthPassword),
		CodeInterpreterJupyterTimeout:      optionalConfigInt(state.CodeInterpreterJupyterTimeout),
	}
}

func (h *ConfigsRouter) modelsConfigResponse() modelsConfigForm {
	state := h.state()
	return modelsConfigForm{
		DefaultModels:        optionalConfigString(state.DefaultModels),
		DefaultPinnedModels:  optionalConfigString(state.DefaultPinnedModels),
		ModelOrderList:       cloneConfigStringList(state.ModelOrderList),
		DefaultModelMetadata: cloneConfigMap(state.DefaultModelMetadata),
		DefaultModelParams:   cloneConfigMap(state.DefaultModelParams),
	}
}

func (h *ConfigsRouter) state() *ConfigsState {
	if h.Config.State == nil {
		h.Config.State = &ConfigsState{}
	}
	return h.Config.State
}

func (h *ConfigsRouter) httpClient() *http.Client {
	if h.Config.HTTPClient != nil {
		return h.Config.HTTPClient
	}
	return http.DefaultClient
}

func (h *ConfigsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	if strings.HasPrefix(token, "sk-") {
		if !h.Config.EnableAPIKeys {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "api key not allowed"})
			return nil, false
		}
		user, err := h.Users.GetUserByAPIKey(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return nil, false
		}
		if user != nil {
			return user, true
		}
	} else {
		claims, err := utils.DecodeToken(h.Config.WebUISecretKey, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
			return nil, false
		}
		user, err := h.Users.GetUserByID(r.Context(), claims.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return nil, false
		}
		if user != nil {
			return user, true
		}
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
	return nil, false
}

func (h *ConfigsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return nil, false
	}
	if user.Role != "admin" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "access prohibited"})
		return nil, false
	}
	return user, true
}

func defaultConfigPayload() map[string]any {
	return map[string]any{
		"version": 0,
		"ui":      map[string]any{},
	}
}

func cloneConfigPayload(source map[string]any) map[string]any {
	body, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	var target map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return map[string]any{}
	}
	return target
}

func cloneConfigList(source []map[string]any) []map[string]any {
	if source == nil {
		return []map[string]any{}
	}
	body, err := json.Marshal(source)
	if err != nil {
		return []map[string]any{}
	}
	var target []map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return []map[string]any{}
	}
	return target
}

func cloneConfigStringList(source []string) []string {
	if source == nil {
		return []string{}
	}
	body, err := json.Marshal(source)
	if err != nil {
		return []string{}
	}
	var target []string
	if err := json.Unmarshal(body, &target); err != nil {
		return []string{}
	}
	return target
}

func cloneConfigMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	body, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	var target map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return map[string]any{}
	}
	return target
}

func getConfigPathValue(configData map[string]any, path string) (any, bool) {
	current := any(configData)
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := node[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func setConfigPathValue(configData map[string]any, path string, value any) {
	current := configData
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func normalizeConfigList(value any) ([]map[string]any, bool) {
	if value == nil {
		return []map[string]any{}, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, false
	}
	return items, true
}

func normalizeConfigStringList(value any) ([]string, bool) {
	if value == nil {
		return []string{}, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var items []string
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, false
	}
	return items, true
}

func normalizeConfigMap(value any) (map[string]any, bool) {
	if value == nil {
		return map[string]any{}, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func normalizeConfigInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func optionalIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func optionalConfigInt(value int) *int {
	result := value
	return &result
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalConfigString(value string) *string {
	result := value
	return &result
}
func buildConfigsBearerHeaders(authType string, key string, extras map[string]string) http.Header {
	headers := http.Header{}
	if strings.TrimSpace(authType) == "bearer" && strings.TrimSpace(key) != "" {
		headers.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	}
	for headerKey, headerValue := range extras {
		if strings.TrimSpace(headerValue) != "" {
			headers.Set(headerKey, headerValue)
		}
	}
	return headers
}

func (h *ConfigsRouter) verifyConfigsGet(rawURL string, headers http.Header) bool {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	req.Header = headers
	resp, err := h.httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func normalizeConfigsHeaderValues(headers map[string]any) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	values := map[string]string{}
	for key, value := range headers {
		if text, ok := value.(string); ok {
			values[key] = text
		}
	}
	return values
}
