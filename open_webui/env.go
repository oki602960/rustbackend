package openwebui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RuntimeConfig struct {
	ListenAddr                           string
	PythonBaseURL                        string
	FrontendDir                          string
	DataDir                              string
	StaticDir                            string
	UploadDir                            string
	DatabaseURL                          string
	DatabaseSchema                       string
	EnableDBMigrations                   bool
	EnableDirectConnections              bool
	EnableBaseModelsCache                bool
	ToolServerConnections                []map[string]any
	TerminalServerConnections            []map[string]any
	DefaultPromptSuggestions             []map[string]any
	WebUIBanners                         []map[string]any
	EnableCodeExecution                  bool
	CodeExecutionEngine                  string
	CodeExecutionJupyterURL              string
	CodeExecutionJupyterAuth             string
	CodeExecutionJupyterAuthToken        string
	CodeExecutionJupyterAuthPassword     string
	CodeExecutionJupyterTimeout          int
	EnableCodeInterpreter                bool
	CodeInterpreterEngine                string
	CodeInterpreterPromptTemplate        string
	CodeInterpreterJupyterURL            string
	CodeInterpreterJupyterAuth           string
	CodeInterpreterJupyterAuthToken      string
	CodeInterpreterJupyterAuthPassword   string
	CodeInterpreterJupyterTimeout        int
	DefaultModels                        string
	DefaultPinnedModels                  string
	ModelOrderList                       []string
	DefaultModelMetadata                 map[string]any
	DefaultModelParams                   map[string]any
	TaskModel                            string
	TaskModelExternal                    string
	EnableTitleGeneration                bool
	TitleGenerationPromptTemplate        string
	ImagePromptGenerationPromptTemplate  string
	EnableAutocompleteGeneration         bool
	AutocompleteGenerationInputMaxLength int
	TagsGenerationPromptTemplate         string
	FollowUpGenerationPromptTemplate     string
	EnableFollowUpGeneration             bool
	EnableTagsGeneration                 bool
	EnableSearchQueryGeneration          bool
	EnableRetrievalQueryGeneration       bool
	QueryGenerationPromptTemplate        string
	ToolsFunctionCallingPromptTemplate   string
	VoiceModePromptTemplate              string
	DatabaseEnableSQLiteWAL              bool
	DatabaseEnableSessionShare           bool
	DatabasePoolSize                     int
	DatabasePoolMaxOverflow              int
	DatabasePoolTimeout                  time.Duration
	DatabasePoolRecycle                  time.Duration
	WebUIAuth                            bool
	EnableInitialAdminSignup             bool
	EnablePasswordAuth                   bool
	EnableAPIKeys                        bool
	EnableSignup                         bool
	DefaultUserRole                      string
	EnableEvaluationArenaModels          bool
	EvaluationArenaModels                []map[string]any
	ShowAdminDetails                     bool
	AdminEmail                           string
	WebUISecretKey                       string
	JWTExpiresIn                         string
	AuthCookieSameSite                   string
	AuthCookieSecure                     bool
	TrustedEmailHeader                   string
}

func ConfigFromEnv() RuntimeConfig {
	dataDir := firstExistingPath(
		os.Getenv("DATA_DIR"),
		filepath.Join("open-webui", "backend", "data"),
		filepath.Join("..", "open-webui", "backend", "data"),
		filepath.Join("backend", "data"),
		"data",
	)

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseType := strings.TrimSpace(os.Getenv("DATABASE_TYPE"))
		databaseUser := strings.TrimSpace(os.Getenv("DATABASE_USER"))
		databasePassword := strings.TrimSpace(os.Getenv("DATABASE_PASSWORD"))
		databaseHost := strings.TrimSpace(os.Getenv("DATABASE_HOST"))
		databasePort := strings.TrimSpace(os.Getenv("DATABASE_PORT"))
		databaseName := strings.TrimSpace(os.Getenv("DATABASE_NAME"))

		if databaseType != "" && databaseUser != "" && databasePassword != "" && databaseHost != "" && databasePort != "" && databaseName != "" {
			databaseURL = databaseType + "://" + databaseUser + ":" + databasePassword + "@" + databaseHost + ":" + databasePort + "/" + databaseName
		} else if databaseType == "sqlite+sqlcipher" {
			databaseURL = "sqlite+sqlcipher:///" + filepath.Join(dataDir, "webui.db")
		} else {
			databaseURL = "sqlite:///" + filepath.Join(dataDir, "webui.db")
		}
	}

	if strings.HasPrefix(databaseURL, "postgres://") {
		databaseURL = "postgresql://" + strings.TrimPrefix(databaseURL, "postgres://")
	}

	return RuntimeConfig{
		ListenAddr:    firstNonEmpty(os.Getenv("OPEN_COREUI_GO_ADDR"), ":8081"),
		PythonBaseURL: strings.TrimSpace(os.Getenv("OPEN_COREUI_PYTHON_BASE_URL")),
		FrontendDir: firstExistingPath(
			os.Getenv("FRONTEND_DIR"),
			filepath.Join("open-webui", "build"),
			filepath.Join("..", "open-webui", "build"),
			"build",
		),
		DataDir: dataDir,
		StaticDir: firstExistingPath(
			os.Getenv("STATIC_DIR"),
			filepath.Join("open-webui", "backend", "open_webui", "static"),
			filepath.Join("..", "open-webui", "backend", "open_webui", "static"),
			filepath.Join("open_webui", "static"),
		),
		UploadDir: firstExistingPath(
			os.Getenv("UPLOAD_DIR"),
			filepath.Join(dataDir, "uploads"),
			"uploads",
		),
		DatabaseURL:                          databaseURL,
		DatabaseSchema:                       strings.TrimSpace(os.Getenv("DATABASE_SCHEMA")),
		EnableDBMigrations:                   parseBoolEnv("ENABLE_DB_MIGRATIONS", true),
		EnableDirectConnections:              parseBoolEnv("ENABLE_DIRECT_CONNECTIONS", false),
		EnableBaseModelsCache:                parseBoolEnv("ENABLE_BASE_MODELS_CACHE", false),
		ToolServerConnections:                parseJSONArrayMapEnv("TOOL_SERVER_CONNECTIONS"),
		TerminalServerConnections:            parseJSONArrayMapEnv("TERMINAL_SERVER_CONNECTIONS"),
		DefaultPromptSuggestions:             parseJSONArrayMapEnv("DEFAULT_PROMPT_SUGGESTIONS"),
		WebUIBanners:                         parseJSONArrayMapEnv("WEBUI_BANNERS"),
		EnableCodeExecution:                  parseBoolEnv("ENABLE_CODE_EXECUTION", true),
		CodeExecutionEngine:                  firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_EXECUTION_ENGINE")), "pyodide"),
		CodeExecutionJupyterURL:              strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_URL")),
		CodeExecutionJupyterAuth:             strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH")),
		CodeExecutionJupyterAuthToken:        strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH_TOKEN")),
		CodeExecutionJupyterAuthPassword:     strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH_PASSWORD")),
		CodeExecutionJupyterTimeout:          parseIntEnv("CODE_EXECUTION_JUPYTER_TIMEOUT", 60),
		EnableCodeInterpreter:                parseBoolEnv("ENABLE_CODE_INTERPRETER", true),
		CodeInterpreterEngine:                firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_INTERPRETER_ENGINE")), "pyodide"),
		CodeInterpreterPromptTemplate:        strings.TrimSpace(os.Getenv("CODE_INTERPRETER_PROMPT_TEMPLATE")),
		CodeInterpreterJupyterURL:            firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_INTERPRETER_JUPYTER_URL")), strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_URL"))),
		CodeInterpreterJupyterAuth:           firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_INTERPRETER_JUPYTER_AUTH")), strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH"))),
		CodeInterpreterJupyterAuthToken:      firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_INTERPRETER_JUPYTER_AUTH_TOKEN")), strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH_TOKEN"))),
		CodeInterpreterJupyterAuthPassword:   firstNonEmpty(strings.TrimSpace(os.Getenv("CODE_INTERPRETER_JUPYTER_AUTH_PASSWORD")), strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_AUTH_PASSWORD"))),
		CodeInterpreterJupyterTimeout:        parseIntEnv("CODE_INTERPRETER_JUPYTER_TIMEOUT", parseIntEnv("CODE_EXECUTION_JUPYTER_TIMEOUT", 60)),
		DefaultModels:                        strings.TrimSpace(os.Getenv("DEFAULT_MODELS")),
		DefaultPinnedModels:                  strings.TrimSpace(os.Getenv("DEFAULT_PINNED_MODELS")),
		ModelOrderList:                       parseJSONArrayStringEnv("MODEL_ORDER_LIST"),
		DefaultModelMetadata:                 parseJSONMapEnv("DEFAULT_MODEL_METADATA"),
		DefaultModelParams:                   parseJSONMapEnv("DEFAULT_MODEL_PARAMS"),
		TaskModel:                            strings.TrimSpace(os.Getenv("TASK_MODEL")),
		TaskModelExternal:                    strings.TrimSpace(os.Getenv("TASK_MODEL_EXTERNAL")),
		EnableTitleGeneration:                parseBoolEnv("ENABLE_TITLE_GENERATION", true),
		TitleGenerationPromptTemplate:        strings.TrimSpace(os.Getenv("TITLE_GENERATION_PROMPT_TEMPLATE")),
		ImagePromptGenerationPromptTemplate:  strings.TrimSpace(os.Getenv("IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE")),
		EnableAutocompleteGeneration:         parseBoolEnv("ENABLE_AUTOCOMPLETE_GENERATION", false),
		AutocompleteGenerationInputMaxLength: parseIntEnv("AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH", -1),
		TagsGenerationPromptTemplate:         strings.TrimSpace(os.Getenv("TAGS_GENERATION_PROMPT_TEMPLATE")),
		FollowUpGenerationPromptTemplate:     strings.TrimSpace(os.Getenv("FOLLOW_UP_GENERATION_PROMPT_TEMPLATE")),
		EnableFollowUpGeneration:             parseBoolEnv("ENABLE_FOLLOW_UP_GENERATION", true),
		EnableTagsGeneration:                 parseBoolEnv("ENABLE_TAGS_GENERATION", true),
		EnableSearchQueryGeneration:          parseBoolEnv("ENABLE_SEARCH_QUERY_GENERATION", true),
		EnableRetrievalQueryGeneration:       parseBoolEnv("ENABLE_RETRIEVAL_QUERY_GENERATION", true),
		QueryGenerationPromptTemplate:        strings.TrimSpace(os.Getenv("QUERY_GENERATION_PROMPT_TEMPLATE")),
		ToolsFunctionCallingPromptTemplate:   strings.TrimSpace(os.Getenv("TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE")),
		VoiceModePromptTemplate:              strings.TrimSpace(os.Getenv("VOICE_MODE_PROMPT_TEMPLATE")),
		DatabaseEnableSQLiteWAL:              parseBoolEnv("DATABASE_ENABLE_SQLITE_WAL", false),
		DatabaseEnableSessionShare:           parseBoolEnv("DATABASE_ENABLE_SESSION_SHARING", false),
		DatabasePoolSize:                     parseIntEnv("DATABASE_POOL_SIZE", 0),
		DatabasePoolMaxOverflow:              parseIntEnv("DATABASE_POOL_MAX_OVERFLOW", 0),
		DatabasePoolTimeout:                  time.Duration(parseIntEnv("DATABASE_POOL_TIMEOUT", 30)) * time.Second,
		DatabasePoolRecycle:                  time.Duration(parseIntEnv("DATABASE_POOL_RECYCLE", 3600)) * time.Second,
		WebUIAuth:                            parseBoolEnv("WEBUI_AUTH", true),
		EnableInitialAdminSignup:             parseBoolEnv("ENABLE_INITIAL_ADMIN_SIGNUP", false),
		EnablePasswordAuth:                   parseBoolEnv("ENABLE_PASSWORD_AUTH", true),
		EnableAPIKeys:                        parseBoolEnv("ENABLE_API_KEYS", true),
		EnableSignup:                         parseBoolEnv("ENABLE_SIGNUP", true),
		DefaultUserRole:                      firstNonEmpty(strings.TrimSpace(os.Getenv("DEFAULT_USER_ROLE")), "pending"),
		EnableEvaluationArenaModels:          parseBoolEnv("ENABLE_EVALUATION_ARENA_MODELS", false),
		EvaluationArenaModels:                parseJSONArrayMapEnv("EVALUATION_ARENA_MODELS"),
		ShowAdminDetails:                     parseBoolEnv("SHOW_ADMIN_DETAILS", true),
		AdminEmail:                           strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		WebUISecretKey:                       firstNonEmpty(os.Getenv("WEBUI_SECRET_KEY"), "open-coreui-dev-secret"),
		JWTExpiresIn:                         firstNonEmpty(os.Getenv("JWT_EXPIRES_IN"), "4w"),
		AuthCookieSameSite:                   firstNonEmpty(os.Getenv("WEBUI_AUTH_COOKIE_SAME_SITE"), "Lax"),
		AuthCookieSecure:                     parseBoolEnv("WEBUI_AUTH_COOKIE_SECURE", false),
		TrustedEmailHeader:                   strings.TrimSpace(os.Getenv("WEBUI_AUTH_TRUSTED_EMAIL_HEADER")),
	}
}

func parseBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstExistingPath(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		cleaned := filepath.Clean(value)
		if info, err := os.Stat(cleaned); err == nil && info.IsDir() {
			return cleaned
		}
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return filepath.Clean(value)
		}
	}
	return "data"
}

func parseJSONArrayMapEnv(key string) []map[string]any {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return []map[string]any{}
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []map[string]any{}
	}
	return items
}

func parseJSONArrayStringEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []string{}
	}
	return items
}

func parseJSONMapEnv(key string) map[string]any {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return map[string]any{}
	}
	var item map[string]any
	if err := json.Unmarshal([]byte(value), &item); err != nil {
		return map[string]any{}
	}
	return item
}
