package openwebui

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/xxnuo/open-coreui/backend/internal/platform/proxy"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/routers"
	"github.com/xxnuo/open-coreui/backend/open_webui/storage"
)

// spaHandler serves a SvelteKit static build: known files are served directly,
// all other paths fall back to index.html so the SPA router can handle them.
type spaHandler struct {
	root string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.root, filepath.Clean("/"+r.URL.Path))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(h.root, "index.html"))
}

func NewHandler(cfg RuntimeConfig) (http.Handler, error) {

	ctx := context.Background()
	db, err := dbinternal.Open(ctx, dbinternal.Options{
		DatabaseURL:     cfg.DatabaseURL,
		DatabaseSchema:  cfg.DatabaseSchema,
		EnableSQLiteWAL: cfg.DatabaseEnableSQLiteWAL,
		PoolSize:        cfg.DatabasePoolSize,
		PoolRecycle:     cfg.DatabasePoolRecycle,
		OpenTimeout:     cfg.DatabasePoolTimeout,
	})
	if err != nil {
		return nil, err
	}
	if cfg.EnableDBMigrations {
		if err := migrations.Run(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	configStore, err := NewConfigStoreWithHandle(ctx, cfg, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	configsState := &routers.ConfigsState{
		EnableDirectConnections:            cfg.EnableDirectConnections,
		EnableBaseModelsCache:              cfg.EnableBaseModelsCache,
		ToolServerConnections:              cloneRuntimeConfigList(cfg.ToolServerConnections),
		TerminalServerConnections:          cloneRuntimeConfigList(cfg.TerminalServerConnections),
		DefaultPromptSuggestions:           cloneRuntimeConfigList(cfg.DefaultPromptSuggestions),
		Banners:                            cloneRuntimeConfigList(cfg.WebUIBanners),
		DefaultModels:                      cfg.DefaultModels,
		DefaultPinnedModels:                cfg.DefaultPinnedModels,
		ModelOrderList:                     cloneRuntimeStringList(cfg.ModelOrderList),
		DefaultModelMetadata:               cloneRuntimeMap(cfg.DefaultModelMetadata),
		DefaultModelParams:                 cloneRuntimeMap(cfg.DefaultModelParams),
		EnableCodeExecution:                cfg.EnableCodeExecution,
		CodeExecutionEngine:                cfg.CodeExecutionEngine,
		CodeExecutionJupyterURL:            cfg.CodeExecutionJupyterURL,
		CodeExecutionJupyterAuth:           cfg.CodeExecutionJupyterAuth,
		CodeExecutionJupyterAuthToken:      cfg.CodeExecutionJupyterAuthToken,
		CodeExecutionJupyterAuthPassword:   cfg.CodeExecutionJupyterAuthPassword,
		CodeExecutionJupyterTimeout:        cfg.CodeExecutionJupyterTimeout,
		EnableCodeInterpreter:              cfg.EnableCodeInterpreter,
		CodeInterpreterEngine:              cfg.CodeInterpreterEngine,
		CodeInterpreterPromptTemplate:      cfg.CodeInterpreterPromptTemplate,
		CodeInterpreterJupyterURL:          cfg.CodeInterpreterJupyterURL,
		CodeInterpreterJupyterAuth:         cfg.CodeInterpreterJupyterAuth,
		CodeInterpreterJupyterAuthToken:    cfg.CodeInterpreterJupyterAuthToken,
		CodeInterpreterJupyterAuthPassword: cfg.CodeInterpreterJupyterAuthPassword,
		CodeInterpreterJupyterTimeout:      cfg.CodeInterpreterJupyterTimeout,
	}
	tasksState := &routers.TasksState{
		TaskModel:                            cfg.TaskModel,
		TaskModelExternal:                    cfg.TaskModelExternal,
		EnableTitleGeneration:                cfg.EnableTitleGeneration,
		TitleGenerationPromptTemplate:        cfg.TitleGenerationPromptTemplate,
		ImagePromptGenerationPromptTemplate:  cfg.ImagePromptGenerationPromptTemplate,
		EnableAutocompleteGeneration:         cfg.EnableAutocompleteGeneration,
		AutocompleteGenerationInputMaxLength: cfg.AutocompleteGenerationInputMaxLength,
		TagsGenerationPromptTemplate:         cfg.TagsGenerationPromptTemplate,
		FollowUpGenerationPromptTemplate:     cfg.FollowUpGenerationPromptTemplate,
		EnableFollowUpGeneration:             cfg.EnableFollowUpGeneration,
		EnableTagsGeneration:                 cfg.EnableTagsGeneration,
		EnableSearchQueryGeneration:          cfg.EnableSearchQueryGeneration,
		EnableRetrievalQueryGeneration:       cfg.EnableRetrievalQueryGeneration,
		QueryGenerationPromptTemplate:        cfg.QueryGenerationPromptTemplate,
		ToolsFunctionCallingPromptTemplate:   cfg.ToolsFunctionCallingPromptTemplate,
		VoiceModePromptTemplate:              cfg.VoiceModePromptTemplate,
	}
	configData, err := configStore.Load(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if value, ok := GetConfigValue(configData, "direct.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableDirectConnections = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "models.base_models_cache"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableBaseModelsCache = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "tool_server.connections"); ok {
		if connections, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.ToolServerConnections = connections
		}
	}
	if value, ok := GetConfigValue(configData, "terminal_server.connections"); ok {
		if connections, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.TerminalServerConnections = connections
		}
	}
	if value, ok := GetConfigValue(configData, "ui.prompt_suggestions"); ok {
		if suggestions, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.DefaultPromptSuggestions = suggestions
		}
	}
	if value, ok := GetConfigValue(configData, "ui.banners"); ok {
		if banners, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.Banners = banners
		}
	}
	if value, ok := GetConfigValue(configData, "ui.default_models"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.DefaultModels = text
		}
	}
	if value, ok := GetConfigValue(configData, "ui.default_pinned_models"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.DefaultPinnedModels = text
		}
	}
	if value, ok := GetConfigValue(configData, "ui.model_order_list"); ok {
		if items, typeOK := decodeRuntimeStringList(value); typeOK {
			configsState.ModelOrderList = items
		}
	}
	if value, ok := GetConfigValue(configData, "models.default_metadata"); ok {
		if payload, typeOK := decodeRuntimeMap(value); typeOK {
			configsState.DefaultModelMetadata = payload
		}
	}
	if value, ok := GetConfigValue(configData, "models.default_params"); ok {
		if payload, typeOK := decodeRuntimeMap(value); typeOK {
			configsState.DefaultModelParams = payload
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableCodeExecution = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.engine"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeExecutionEngine = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.jupyter.url"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeExecutionJupyterURL = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.jupyter.auth"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeExecutionJupyterAuth = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.jupyter.auth_token"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeExecutionJupyterAuthToken = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.jupyter.auth_password"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeExecutionJupyterAuthPassword = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_execution.jupyter.timeout"); ok {
		if timeout, typeOK := decodeRuntimeInt(value); typeOK {
			configsState.CodeExecutionJupyterTimeout = timeout
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableCodeInterpreter = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.engine"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterEngine = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.jupyter.url"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterJupyterURL = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.jupyter.auth"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterJupyterAuth = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.jupyter.auth_token"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterJupyterAuthToken = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.jupyter.auth_password"); ok {
		if text, typeOK := value.(string); typeOK {
			configsState.CodeInterpreterJupyterAuthPassword = text
		}
	}
	if value, ok := GetConfigValue(configData, "code_interpreter.jupyter.timeout"); ok {
		if timeout, typeOK := decodeRuntimeInt(value); typeOK {
			configsState.CodeInterpreterJupyterTimeout = timeout
		}
	}
	if value, ok := GetConfigValue(configData, "task.model.default"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.TaskModel = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.model.external"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.TaskModelExternal = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.title.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableTitleGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.title.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.TitleGenerationPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.image.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.ImagePromptGenerationPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.autocomplete.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableAutocompleteGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.autocomplete.input_max_length"); ok {
		if amount, typeOK := decodeRuntimeInt(value); typeOK {
			tasksState.AutocompleteGenerationInputMaxLength = amount
		}
	}
	if value, ok := GetConfigValue(configData, "task.tags.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.TagsGenerationPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.follow_up.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.FollowUpGenerationPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.follow_up.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableFollowUpGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.tags.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableTagsGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.query.search.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableSearchQueryGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.query.retrieval.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			tasksState.EnableRetrievalQueryGeneration = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "task.query.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.QueryGenerationPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.tools.function_calling.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.ToolsFunctionCallingPromptTemplate = text
		}
	} else if value, ok := GetConfigValue(configData, "task.tools.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.ToolsFunctionCallingPromptTemplate = text
		}
	}
	if value, ok := GetConfigValue(configData, "task.voice.prompt_template"); ok {
		if text, typeOK := value.(string); typeOK {
			tasksState.VoiceModePromptTemplate = text
		}
	}

	var defaultHandler http.Handler
	if cfg.PythonBaseURL != "" {
		up, err := proxy.New(cfg.PythonBaseURL)
		if err != nil {
			return nil, err
		}
		defaultHandler = up
	} else {
		defaultHandler = spaHandler{root: cfg.FrontendDir}
	}

	mux := http.NewServeMux()
	usersTable := models.NewUsersTable(db)
	authRouter := &routers.AuthsRouter{
		Config: routers.AuthRuntimeConfig{
			WebUIAuth:                cfg.WebUIAuth,
			EnableInitialAdminSignup: cfg.EnableInitialAdminSignup,
			EnablePasswordAuth:       cfg.EnablePasswordAuth,
			EnableAPIKeys:            cfg.EnableAPIKeys,
			EnableSignup:             cfg.EnableSignup,
			DefaultUserRole:          cfg.DefaultUserRole,
			ShowAdminDetails:         cfg.ShowAdminDetails,
			AdminEmail:               cfg.AdminEmail,
			WebUISecretKey:           cfg.WebUISecretKey,
			JWTExpiresIn:             cfg.JWTExpiresIn,
			AuthCookieSameSite:       cfg.AuthCookieSameSite,
			AuthCookieSecure:         cfg.AuthCookieSecure,
			TrustedEmailHeader:       cfg.TrustedEmailHeader,
		},
		Users: usersTable,
		Auths: models.NewAuthsTable(db, usersTable),
	}
	authRouter.Register(mux)
	usersRouter := &routers.UsersRouter{
		Config: routers.UsersRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			StaticDir:      cfg.StaticDir,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:         usersTable,
		Auths:         authRouter.Auths,
		Groups:        models.NewGroupsTable(db),
		OAuthSessions: models.NewOAuthSessionsTable(db),
	}
	usersRouter.Register(mux)
	groupsTable := models.NewGroupsTable(db)
	accessGrantsTable := models.NewAccessGrantsTable(db)
	groupsRouter := &routers.GroupsRouter{
		Config: routers.GroupsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:  usersTable,
		Groups: groupsTable,
	}
	groupsRouter.Register(mux)
	notesRouter := &routers.NotesRouter{
		Config: routers.NotesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Notes:        models.NewNotesTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	notesRouter.Register(mux)
	memoriesRouter := &routers.MemoriesRouter{
		Config: routers.MemoriesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:    usersTable,
		Memories: models.NewMemoriesTable(db),
	}
	memoriesRouter.Register(mux)
	foldersRouter := &routers.FoldersRouter{
		Config: routers.FoldersRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:   usersTable,
		Folders: models.NewFoldersTable(db),
	}
	foldersRouter.Register(mux)
	promptsRouter := &routers.PromptsRouter{
		Config: routers.PromptsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Prompts:      models.NewPromptsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	promptsRouter.Register(mux)
	functionsRouter := &routers.FunctionsRouter{
		Config: routers.FunctionsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:     usersTable,
		Functions: models.NewFunctionsTable(db),
	}
	functionsRouter.Register(mux)
	skillsRouter := &routers.SkillsRouter{
		Config: routers.SkillsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Skills:       models.NewSkillsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	skillsRouter.Register(mux)
	modelsRouter := &routers.ModelsRouter{
		Config: routers.ModelsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			StaticDir:      cfg.StaticDir,
		},
		Users:        usersTable,
		Models:       models.NewModelsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	modelsRouter.Register(mux)
	filesRouter := &routers.FilesRouter{
		Config: routers.FilesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Files:        models.NewFilesTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
		Storage:      storage.NewLocalProvider(cfg.UploadDir),
	}
	filesRouter.Register(mux)
	evaluationsRouter := &routers.EvaluationsRouter{
		Config: &routers.EvaluationsRuntimeConfig{
			WebUISecretKey:              cfg.WebUISecretKey,
			EnableAPIKeys:               cfg.EnableAPIKeys,
			EnableEvaluationArenaModels: cfg.EnableEvaluationArenaModels,
			EvaluationArenaModels:       cfg.EvaluationArenaModels,
		},
		Users:     usersTable,
		Feedbacks: models.NewFeedbacksTable(db),
	}
	evaluationsRouter.Register(mux)
	toolsRouter := &routers.ToolsRouter{
		Config: routers.ToolsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Tools:        models.NewToolsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	toolsRouter.Register(mux)
	configsRouter := &routers.ConfigsRouter{
		Config: routers.ConfigsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			State:          configsState,
		},
		Users: usersTable,
		Store: configStore,
	}
	configsRouter.Register(mux)
	terminalsRouter := &routers.TerminalsRouter{
		Config: routers.TerminalsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			State:          configsState,
		},
		Users:  usersTable,
		Groups: groupsTable,
	}
	terminalsRouter.Register(mux)
	utilsRouter := &routers.UtilsRouter{
		Config: routers.UtilsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			DatabaseURL:    cfg.DatabaseURL,
		},
		Users: usersTable,
	}
	utilsRouter.Register(mux)
	analyticsRouter := &routers.AnalyticsRouter{
		Config: routers.AnalyticsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		ChatMessages: models.NewChatMessagesTable(db),
		Chats:        models.NewChatsTable(db),
		Feedbacks:    models.NewFeedbacksTable(db),
	}
	analyticsRouter.Register(mux)
	tasksRouter := &routers.TasksRouter{
		Config: routers.TasksRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			State:          tasksState,
			Store:          configStore,
		},
		Users: usersTable,
	}
	tasksRouter.Register(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", defaultHandler)

	return mux, nil
}

func Run() error {
	cfg := ConfigFromEnv()
	handler, err := NewHandler(cfg)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}

func cloneRuntimeConfigList(source []map[string]any) []map[string]any {
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

func decodeRuntimeConfigList(value any) ([]map[string]any, bool) {
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

func decodeRuntimeInt(value any) (int, bool) {
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
func cloneRuntimeStringList(source []string) []string {
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

func decodeRuntimeStringList(value any) ([]string, bool) {
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

func cloneRuntimeMap(source map[string]any) map[string]any {
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

func decodeRuntimeMap(value any) (map[string]any, bool) {
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
