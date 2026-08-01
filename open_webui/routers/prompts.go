package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type PromptsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type PromptsRouter struct {
	Config       PromptsRuntimeConfig
	Users        *models.UsersTable
	Prompts      *models.PromptsTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
}

type promptForm struct {
	Command string         `json:"command"`
	Name    string         `json:"name"`
	Content string         `json:"content"`
	Data    map[string]any `json:"data,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
}

type promptListResponse struct {
	Items []models.Prompt `json:"items"`
	Total int             `json:"total"`
}

type promptAccessResponse struct {
	models.Prompt
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type promptAccessGrantsForm struct {
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *PromptsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/prompts/", h.GetPrompts)
	mux.HandleFunc("GET /api/v1/prompts/tags", h.GetPromptTags)
	mux.HandleFunc("GET /api/v1/prompts/list", h.GetPromptList)
	mux.HandleFunc("POST /api/v1/prompts/create", h.CreatePrompt)
	mux.HandleFunc("GET /api/v1/prompts/command/{command}", h.GetPromptByCommand)
	mux.HandleFunc("GET /api/v1/prompts/id/{prompt_id}", h.GetPromptByID)
	mux.HandleFunc("POST /api/v1/prompts/id/{prompt_id}/access/update", h.UpdatePromptAccessByID)
	mux.HandleFunc("POST /api/v1/prompts/id/{prompt_id}/update", h.UpdatePromptByID)
	mux.HandleFunc("DELETE /api/v1/prompts/id/{prompt_id}", h.DeletePromptByID)
}

func (h *PromptsRouter) GetPrompts(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		prompts, err := h.Prompts.GetPrompts(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, h.serializePrompts(r, prompts, user))
		return
	}
	prompts, err := h.Prompts.GetPrompts(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.filterReadablePrompts(r, prompts, user))
}

func (h *PromptsRouter) GetPromptTags(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	tags, err := h.Prompts.GetTags(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *PromptsRouter) GetPromptList(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 30
	skip := (page - 1) * limit
	items, _, err := h.Prompts.SearchPrompts(r.Context(), "", models.PromptSearchOptions{
		Query: r.URL.Query().Get("query"),
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	filtered := h.filterReadablePrompts(r, items, user)
	writeJSON(w, http.StatusOK, promptListResponse{Items: extractPrompts(filtered), Total: len(filtered)})
}

func (h *PromptsRouter) CreatePrompt(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form promptForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	existing, err := h.Prompts.GetPromptByCommand(r.Context(), form.Command)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "command taken"})
		return
	}
	prompt, err := h.Prompts.InsertNewPrompt(r.Context(), models.PromptCreateParams{
		UserID:  user.ID,
		Command: form.Command,
		Name:    form.Name,
		Content: form.Content,
		Data:    form.Data,
		Meta:    form.Meta,
		Tags:    form.Tags,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, prompt)
}

func (h *PromptsRouter) GetPromptByCommand(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	prompt, err := h.Prompts.GetPromptByCommand(r.Context(), r.PathValue("command"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if prompt == nil || !h.canReadPrompt(r, user, prompt) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializePrompt(r, *prompt, user))
}

func (h *PromptsRouter) GetPromptByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	prompt, err := h.Prompts.GetPromptByID(r.Context(), r.PathValue("prompt_id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if prompt == nil || !h.canReadPrompt(r, user, prompt) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializePrompt(r, *prompt, user))
}

func (h *PromptsRouter) UpdatePromptAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Prompts.GetPromptByID(r.Context(), r.PathValue("prompt_id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWritePrompt(r, user, current) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	var form promptAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "prompt", current.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Prompts.GetPromptByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializePrompt(r, *updated, user))
}

func (h *PromptsRouter) UpdatePromptByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Prompts.GetPromptByID(r.Context(), r.PathValue("prompt_id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWritePrompt(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form promptForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	prompt, err := h.Prompts.UpdatePromptByID(r.Context(), current.ID, models.PromptUpdateParams{
		Command: promptStringPtr(form.Command),
		Name:    promptStringPtr(form.Name),
		Content: promptStringPtr(form.Content),
		Data:    form.Data,
		Meta:    form.Meta,
		Tags:    form.Tags,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, prompt)
}

func (h *PromptsRouter) DeletePromptByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Prompts.GetPromptByID(r.Context(), r.PathValue("prompt_id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWritePrompt(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Prompts.DeletePromptByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *PromptsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func promptStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *PromptsRouter) userGroupIDs(r *http.Request, userID string) []string {
	if h.Groups == nil {
		return nil
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), userID)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

func (h *PromptsRouter) canReadPrompt(r *http.Request, user *models.User, prompt *models.Prompt) bool {
	if user.Role == "admin" || prompt.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "prompt", prompt.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *PromptsRouter) canWritePrompt(r *http.Request, user *models.User, prompt *models.Prompt) bool {
	if user.Role == "admin" || prompt.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "prompt", prompt.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *PromptsRouter) serializePrompt(r *http.Request, prompt models.Prompt, user *models.User) promptAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "prompt", prompt.ID)
		if err == nil {
			grants = loaded
		}
	}
	return promptAccessResponse{
		Prompt:       prompt,
		WriteAccess:  h.canWritePrompt(r, user, &prompt),
		AccessGrants: grants,
	}
}

func (h *PromptsRouter) serializePrompts(r *http.Request, prompts []models.Prompt, user *models.User) []promptAccessResponse {
	result := make([]promptAccessResponse, 0, len(prompts))
	for _, prompt := range prompts {
		result = append(result, h.serializePrompt(r, prompt, user))
	}
	return result
}

func (h *PromptsRouter) filterReadablePrompts(r *http.Request, prompts []models.Prompt, user *models.User) []promptAccessResponse {
	result := make([]promptAccessResponse, 0, len(prompts))
	for _, prompt := range prompts {
		if h.canReadPrompt(r, user, &prompt) {
			result = append(result, h.serializePrompt(r, prompt, user))
		}
	}
	return result
}

func extractPrompts(items []promptAccessResponse) []models.Prompt {
	result := make([]models.Prompt, 0, len(items))
	for _, item := range items {
		result = append(result, item.Prompt)
	}
	return result
}
