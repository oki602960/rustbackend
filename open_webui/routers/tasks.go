package routers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type TasksState struct {
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
}

type TasksRuntimeConfig struct {
	WebUISecretKey  string
	EnableAPIKeys   bool
	State           *TasksState
	Store           ConfigsStore
	ActiveChatsFunc func(chatIDs []string) []string
}

type TasksRouter struct {
	Config TasksRuntimeConfig
	Users  *models.UsersTable
}

type activeChatsForm struct {
	ChatIDs []string `json:"chat_ids"`
}

type taskConfigForm struct {
	TaskModel                            *string `json:"TASK_MODEL"`
	TaskModelExternal                    *string `json:"TASK_MODEL_EXTERNAL"`
	EnableTitleGeneration                bool    `json:"ENABLE_TITLE_GENERATION"`
	TitleGenerationPromptTemplate        string  `json:"TITLE_GENERATION_PROMPT_TEMPLATE"`
	ImagePromptGenerationPromptTemplate  string  `json:"IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE"`
	EnableAutocompleteGeneration         bool    `json:"ENABLE_AUTOCOMPLETE_GENERATION"`
	AutocompleteGenerationInputMaxLength int     `json:"AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH"`
	TagsGenerationPromptTemplate         string  `json:"TAGS_GENERATION_PROMPT_TEMPLATE"`
	FollowUpGenerationPromptTemplate     string  `json:"FOLLOW_UP_GENERATION_PROMPT_TEMPLATE"`
	EnableFollowUpGeneration             bool    `json:"ENABLE_FOLLOW_UP_GENERATION"`
	EnableTagsGeneration                 bool    `json:"ENABLE_TAGS_GENERATION"`
	EnableSearchQueryGeneration          bool    `json:"ENABLE_SEARCH_QUERY_GENERATION"`
	EnableRetrievalQueryGeneration       bool    `json:"ENABLE_RETRIEVAL_QUERY_GENERATION"`
	QueryGenerationPromptTemplate        string  `json:"QUERY_GENERATION_PROMPT_TEMPLATE"`
	ToolsFunctionCallingPromptTemplate   string  `json:"TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE"`
	VoiceModePromptTemplate              *string `json:"VOICE_MODE_PROMPT_TEMPLATE"`
}

func (h *TasksRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks/active/chats", h.CheckActiveChats)
	mux.HandleFunc("GET /api/v1/tasks/config", h.GetTaskConfig)
	mux.HandleFunc("POST /api/v1/tasks/config/update", h.UpdateTaskConfig)
	mux.HandleFunc("POST /api/v1/tasks/title/completions", h.GenerateTitle)
	mux.HandleFunc("POST /api/v1/tasks/follow_up/completions", h.GenerateFollowUps)
	mux.HandleFunc("POST /api/v1/tasks/follow_ups/completions", h.GenerateFollowUps)
	mux.HandleFunc("POST /api/v1/tasks/tags/completions", h.GenerateTags)
	mux.HandleFunc("POST /api/v1/tasks/image_prompt/completions", h.GenerateImagePrompt)
	mux.HandleFunc("POST /api/v1/tasks/emoji/completions", h.GenerateEmoji)
	mux.HandleFunc("POST /api/v1/tasks/queries/completions", h.GenerateQueries)
	mux.HandleFunc("POST /api/v1/tasks/auto/completions", h.GenerateAutoCompletion)
	mux.HandleFunc("POST /api/v1/tasks/moa/completions", h.GenerateMoACompletion)
}

func (h *TasksRouter) CheckActiveChats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	var form activeChatsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	activeChatIDs := []string{}
	if h.Config.ActiveChatsFunc != nil {
		activeChatIDs = h.Config.ActiveChatsFunc(form.ChatIDs)
	}
	writeJSON(w, http.StatusOK, map[string]any{"active_chat_ids": activeChatIDs})
}

func (h *TasksRouter) GetTaskConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.taskConfigResponse())
}

func (h *TasksRouter) UpdateTaskConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form taskConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	state := h.state()
	state.TaskModel = derefOptionalTaskString(form.TaskModel)
	state.TaskModelExternal = derefOptionalTaskString(form.TaskModelExternal)
	state.EnableTitleGeneration = form.EnableTitleGeneration
	state.TitleGenerationPromptTemplate = form.TitleGenerationPromptTemplate
	state.ImagePromptGenerationPromptTemplate = form.ImagePromptGenerationPromptTemplate
	state.EnableAutocompleteGeneration = form.EnableAutocompleteGeneration
	state.AutocompleteGenerationInputMaxLength = form.AutocompleteGenerationInputMaxLength
	state.TagsGenerationPromptTemplate = form.TagsGenerationPromptTemplate
	state.FollowUpGenerationPromptTemplate = form.FollowUpGenerationPromptTemplate
	state.EnableFollowUpGeneration = form.EnableFollowUpGeneration
	state.EnableTagsGeneration = form.EnableTagsGeneration
	state.EnableSearchQueryGeneration = form.EnableSearchQueryGeneration
	state.EnableRetrievalQueryGeneration = form.EnableRetrievalQueryGeneration
	state.QueryGenerationPromptTemplate = form.QueryGenerationPromptTemplate
	state.ToolsFunctionCallingPromptTemplate = form.ToolsFunctionCallingPromptTemplate
	state.VoiceModePromptTemplate = derefOptionalTaskString(form.VoiceModePromptTemplate)

	if h.Config.Store != nil {
		configData, err := h.Config.Store.Load(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		if configData == nil {
			configData = defaultConfigPayload()
		}
		setConfigPathValue(configData, "task.model.default", state.TaskModel)
		setConfigPathValue(configData, "task.model.external", state.TaskModelExternal)
		setConfigPathValue(configData, "task.title.enable", state.EnableTitleGeneration)
		setConfigPathValue(configData, "task.title.prompt_template", state.TitleGenerationPromptTemplate)
		setConfigPathValue(configData, "task.image.prompt_template", state.ImagePromptGenerationPromptTemplate)
		setConfigPathValue(configData, "task.autocomplete.enable", state.EnableAutocompleteGeneration)
		setConfigPathValue(configData, "task.autocomplete.input_max_length", state.AutocompleteGenerationInputMaxLength)
		setConfigPathValue(configData, "task.tags.prompt_template", state.TagsGenerationPromptTemplate)
		setConfigPathValue(configData, "task.follow_up.prompt_template", state.FollowUpGenerationPromptTemplate)
		setConfigPathValue(configData, "task.follow_up.enable", state.EnableFollowUpGeneration)
		setConfigPathValue(configData, "task.tags.enable", state.EnableTagsGeneration)
		setConfigPathValue(configData, "task.query.search.enable", state.EnableSearchQueryGeneration)
		setConfigPathValue(configData, "task.query.retrieval.enable", state.EnableRetrievalQueryGeneration)
		setConfigPathValue(configData, "task.query.prompt_template", state.QueryGenerationPromptTemplate)
		setConfigPathValue(configData, "task.tools.function_calling.prompt_template", state.ToolsFunctionCallingPromptTemplate)
		setConfigPathValue(configData, "task.voice.prompt_template", state.VoiceModePromptTemplate)
		if err := h.Config.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, h.taskConfigResponse())
}

func (h *TasksRouter) GenerateTitle(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	if !h.state().EnableTitleGeneration {
		writeJSON(w, http.StatusOK, map[string]string{"detail": "Title generation is disabled"})
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	title := buildTaskTitle(taskPayloadText(payload))
	writeJSON(w, http.StatusOK, taskCompletionResponse(`{"title":"`+escapeTaskJSONString(title)+`"}`))
}

func (h *TasksRouter) GenerateFollowUps(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	if !h.state().EnableFollowUpGeneration {
		writeJSON(w, http.StatusOK, map[string]string{"detail": "Follow-up generation is disabled"})
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	followUps := buildFollowUps(taskPayloadText(payload))
	body, _ := json.Marshal(map[string]any{"follow_ups": followUps})
	writeJSON(w, http.StatusOK, taskCompletionResponse(string(body)))
}

func (h *TasksRouter) GenerateTags(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	if !h.state().EnableTagsGeneration {
		writeJSON(w, http.StatusOK, map[string]string{"detail": "Tags generation is disabled"})
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	tags := buildTaskTags(taskPayloadText(payload))
	body, _ := json.Marshal(map[string]any{"tags": tags})
	writeJSON(w, http.StatusOK, taskCompletionResponse(string(body)))
}

func (h *TasksRouter) GenerateEmoji(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, taskCompletionResponse(selectEmoji(taskPayloadText(payload))))
}

func (h *TasksRouter) GenerateImagePrompt(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	body, _ := json.Marshal(map[string]any{"prompt": buildImagePrompt(taskPayloadText(payload))})
	writeJSON(w, http.StatusOK, taskCompletionResponse(string(body)))
}

func (h *TasksRouter) GenerateQueries(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	queryType, _ := payload["type"].(string)
	if queryType == "web_search" && !h.state().EnableSearchQueryGeneration {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Search query generation is disabled"})
		return
	}
	if queryType == "retrieval" && !h.state().EnableRetrievalQueryGeneration {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Query generation is disabled"})
		return
	}
	queries := buildQueries(taskPayloadText(payload), payloadPrompt(payload))
	body, _ := json.Marshal(map[string]any{"queries": queries})
	writeJSON(w, http.StatusOK, taskCompletionResponse(string(body)))
}

func (h *TasksRouter) GenerateAutoCompletion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	if !h.state().EnableAutocompleteGeneration {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Autocompletion generation is disabled"})
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	prompt := payloadPrompt(payload)
	limit := h.state().AutocompleteGenerationInputMaxLength
	if limit > 0 && len(prompt) > limit {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Input prompt exceeds maximum length"})
		return
	}
	body, _ := json.Marshal(map[string]any{"text": buildAutocomplete(prompt)})
	writeJSON(w, http.StatusOK, taskCompletionResponse(string(body)))
}

func (h *TasksRouter) GenerateMoACompletion(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	payload, ok := decodeTaskPayload(w, r)
	if !ok {
		return
	}
	content := buildMoAResponse(payloadPrompt(payload), payloadResponses(payload))

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "streaming unsupported"})
		return
	}

	for _, chunk := range splitTaskChunks(content, 8) {
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{
					"delta": map[string]any{
						"content": chunk,
					},
				},
			},
		})
		_, _ = w.Write([]byte("data: " + string(body) + "\n\n"))
		flusher.Flush()
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (h *TasksRouter) taskConfigResponse() map[string]any {
	state := h.state()
	return map[string]any{
		"TASK_MODEL":                               state.TaskModel,
		"TASK_MODEL_EXTERNAL":                      state.TaskModelExternal,
		"TITLE_GENERATION_PROMPT_TEMPLATE":         state.TitleGenerationPromptTemplate,
		"IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE":  state.ImagePromptGenerationPromptTemplate,
		"ENABLE_AUTOCOMPLETE_GENERATION":           state.EnableAutocompleteGeneration,
		"AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH": state.AutocompleteGenerationInputMaxLength,
		"TAGS_GENERATION_PROMPT_TEMPLATE":          state.TagsGenerationPromptTemplate,
		"FOLLOW_UP_GENERATION_PROMPT_TEMPLATE":     state.FollowUpGenerationPromptTemplate,
		"ENABLE_FOLLOW_UP_GENERATION":              state.EnableFollowUpGeneration,
		"ENABLE_TAGS_GENERATION":                   state.EnableTagsGeneration,
		"ENABLE_TITLE_GENERATION":                  state.EnableTitleGeneration,
		"ENABLE_SEARCH_QUERY_GENERATION":           state.EnableSearchQueryGeneration,
		"ENABLE_RETRIEVAL_QUERY_GENERATION":        state.EnableRetrievalQueryGeneration,
		"QUERY_GENERATION_PROMPT_TEMPLATE":         state.QueryGenerationPromptTemplate,
		"TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE":   state.ToolsFunctionCallingPromptTemplate,
		"VOICE_MODE_PROMPT_TEMPLATE":               state.VoiceModePromptTemplate,
	}
}

func (h *TasksRouter) state() *TasksState {
	if h.Config.State == nil {
		h.Config.State = &TasksState{}
	}
	return h.Config.State
}

func (h *TasksRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *TasksRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func derefOptionalTaskString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func decodeTaskPayload(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return nil, false
	}
	return payload, true
}

func taskCompletionResponse(content string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"content": content,
				},
			},
		},
	}
}

func taskPayloadText(payload map[string]any) string {
	prompt := payloadPrompt(payload)
	if strings.TrimSpace(prompt) != "" {
		return prompt
	}
	if rawMessages, ok := payload["messages"].([]any); ok {
		parts := make([]string, 0, len(rawMessages))
		for _, raw := range rawMessages {
			message, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			content := taskContentText(message["content"])
			if strings.TrimSpace(content) != "" {
				parts = append(parts, content)
			}
		}
		return strings.Join(parts, " ")
	}
	if rawMessages, ok := payload["messages"].(string); ok {
		return rawMessages
	}
	return ""
}

func payloadPrompt(payload map[string]any) string {
	prompt, _ := payload["prompt"].(string)
	return strings.TrimSpace(prompt)
}

func taskContentText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			block, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			text, _ := block["text"].(string)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func buildTaskTitle(source string) string {
	text := sanitizeTaskText(source)
	if text == "" {
		return "New Chat"
	}
	words := strings.Fields(text)
	if len(words) >= 3 {
		if len(words) > 5 {
			words = words[:5]
		}
		return strings.Join(words, " ")
	}
	if utf8.RuneCountInString(text) > 18 {
		runes := []rune(text)
		return string(runes[:18])
	}
	return text
}

func buildFollowUps(source string) []string {
	title := buildTaskTitle(source)
	return []string{
		"Can you explain " + title + " in more detail?",
		"What are the key takeaways about " + title + "?",
		"What should I do next about " + title + "?",
	}
}

func buildTaskTags(source string) []string {
	text := strings.ToLower(sanitizeTaskText(source))
	wordPattern := regexp.MustCompile(`[a-z0-9][a-z0-9_-]{2,}`)
	matches := wordPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return []string{"General"}
	}
	counts := map[string]int{}
	for _, match := range matches {
		counts[match]++
	}
	tags := make([]string, 0, len(counts))
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		if counts[tags[i]] == counts[tags[j]] {
			return tags[i] < tags[j]
		}
		return counts[tags[i]] > counts[tags[j]]
	})
	if len(tags) > 3 {
		tags = tags[:3]
	}
	return tags
}

func selectEmoji(source string) string {
	text := strings.ToLower(source)
	switch {
	case strings.Contains(text, "code"), strings.Contains(text, "bug"), strings.Contains(text, "dev"):
		return "💻"
	case strings.Contains(text, "data"), strings.Contains(text, "chart"), strings.Contains(text, "analysis"):
		return "📊"
	case strings.Contains(text, "idea"), strings.Contains(text, "plan"), strings.Contains(text, "think"):
		return "💡"
	case strings.Contains(text, "money"), strings.Contains(text, "price"), strings.Contains(text, "finance"):
		return "💰"
	default:
		return "💬"
	}
}

func buildQueries(source string, prompt string) []string {
	text := sanitizeTaskText(firstNonEmptyTask(prompt, source))
	if text == "" {
		return []string{}
	}
	if len(text) > 80 {
		runes := []rune(text)
		text = string(runes[:80])
	}
	return []string{text}
}

func buildAutocomplete(prompt string) string {
	text := strings.TrimSpace(prompt)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	switch {
	case strings.HasSuffix(text, "?"):
		return " Please explain the next steps."
	case strings.Contains(lower, "how"):
		return " step by step."
	case strings.Contains(lower, "why"):
		return " with the main reasons."
	case strings.Contains(lower, "best"):
		return " options for this use case."
	default:
		return " in more detail."
	}
}

func buildImagePrompt(source string) string {
	text := sanitizeTaskText(source)
	if text == "" {
		return "A clear, detailed illustration."
	}
	return "Create an image that visually represents: " + text
}

func buildMoAResponse(prompt string, responses []string) string {
	cleanPrompt := sanitizeTaskText(prompt)
	if cleanPrompt == "" {
		cleanPrompt = "the request"
	}
	parts := make([]string, 0, len(responses))
	for _, response := range responses {
		text := strings.TrimSpace(response)
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "Combined response for " + cleanPrompt + "."
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return "Combined response for " + cleanPrompt + ": " + strings.Join(parts, " ")
}

func payloadResponses(payload map[string]any) []string {
	rawResponses, ok := payload["responses"].([]any)
	if !ok {
		return []string{}
	}
	responses := make([]string, 0, len(rawResponses))
	for _, raw := range rawResponses {
		text, _ := raw.(string)
		if strings.TrimSpace(text) != "" {
			responses = append(responses, text)
		}
	}
	return responses
}

func splitTaskChunks(content string, size int) []string {
	if size <= 0 || len(content) <= size {
		return []string{content}
	}
	runes := []rune(content)
	chunks := make([]string, 0, (len(runes)/size)+1)
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func sanitizeTaskText(source string) string {
	parts := strings.FieldsFunc(source, func(r rune) bool {
		return unicode.IsSpace(r) || slices.Contains([]rune{',', '.', ';', ':', '!', '?', '\n', '\r', '\t'}, r)
	})
	return strings.TrimSpace(strings.Join(parts, " "))
}

func firstNonEmptyTask(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func escapeTaskJSONString(value string) string {
	body, _ := json.Marshal(value)
	if len(body) >= 2 {
		return string(body[1 : len(body)-1])
	}
	return value
}
