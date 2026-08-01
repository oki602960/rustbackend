package routers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type ToolsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type ToolsRouter struct {
	Config       ToolsRuntimeConfig
	Users        *models.UsersTable
	Tools        *models.ToolsTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
}

type toolForm struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type toolAccessResponse struct {
	models.Tool
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type toolAccessGrantsForm struct {
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *ToolsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tools/", h.GetTools)
	mux.HandleFunc("GET /api/v1/tools/list", h.GetToolList)
	mux.HandleFunc("GET /api/v1/tools/export", h.ExportTools)
	mux.HandleFunc("POST /api/v1/tools/create", h.CreateTool)
	mux.HandleFunc("GET /api/v1/tools/id/{id}", h.GetToolByID)
	mux.HandleFunc("POST /api/v1/tools/id/{id}/access/update", h.UpdateToolAccessByID)
	mux.HandleFunc("POST /api/v1/tools/id/{id}/update", h.UpdateToolByID)
	mux.HandleFunc("DELETE /api/v1/tools/id/{id}/delete", h.DeleteToolByID)
}

func (h *ToolsRouter) GetTools(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		tools, err := h.Tools.GetTools(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, h.serializeTools(r, tools, user))
		return
	}
	tools, err := h.Tools.GetTools(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.filterReadableTools(r, tools, user))
}

func (h *ToolsRouter) GetToolList(w http.ResponseWriter, r *http.Request) {
	h.GetTools(w, r)
}

func (h *ToolsRouter) ExportTools(w http.ResponseWriter, r *http.Request) {
	h.GetTools(w, r)
}

func (h *ToolsRouter) CreateTool(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form toolForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	matched, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, form.ID)
	if !matched {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Only alphanumeric characters and underscores are allowed in the id"})
		return
	}
	form.ID = strings.ToLower(form.ID)
	existing, err := h.Tools.GetToolByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "id taken"})
		return
	}
	tool, err := h.Tools.InsertNewTool(r.Context(), models.ToolCreateParams{
		ID:      form.ID,
		UserID:  user.ID,
		Name:    form.Name,
		Content: form.Content,
		Meta:    form.Meta,
		Specs:   []map[string]any{},
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

func (h *ToolsRouter) GetToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	tool, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if tool == nil || !h.canReadTool(r, user, tool) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeTool(r, *tool, user))
}

func (h *ToolsRouter) UpdateToolAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteTool(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form toolAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "tool", current.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Tools.GetToolByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeTool(r, *updated, user))
}

func (h *ToolsRouter) UpdateToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteTool(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form toolForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	tool, err := h.Tools.UpdateToolByID(r.Context(), current.ID, models.ToolUpdateParams{
		Name:    toolStringPtr(form.Name),
		Content: toolStringPtr(form.Content),
		Meta:    form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

func (h *ToolsRouter) DeleteToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteTool(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Tools.DeleteToolByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *ToolsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func toolStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *ToolsRouter) userGroupIDs(r *http.Request, userID string) []string {
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

func (h *ToolsRouter) canReadTool(r *http.Request, user *models.User, tool *models.Tool) bool {
	if user.Role == "admin" || tool.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "tool", tool.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *ToolsRouter) canWriteTool(r *http.Request, user *models.User, tool *models.Tool) bool {
	if user.Role == "admin" || tool.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "tool", tool.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *ToolsRouter) serializeTool(r *http.Request, tool models.Tool, user *models.User) toolAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "tool", tool.ID)
		if err == nil {
			grants = loaded
		}
	}
	return toolAccessResponse{
		Tool:         tool,
		WriteAccess:  h.canWriteTool(r, user, &tool),
		AccessGrants: grants,
	}
}

func (h *ToolsRouter) serializeTools(r *http.Request, tools []models.Tool, user *models.User) []toolAccessResponse {
	result := make([]toolAccessResponse, 0, len(tools))
	for _, tool := range tools {
		result = append(result, h.serializeTool(r, tool, user))
	}
	return result
}

func (h *ToolsRouter) filterReadableTools(r *http.Request, tools []models.Tool, user *models.User) []toolAccessResponse {
	result := make([]toolAccessResponse, 0, len(tools))
	for _, tool := range tools {
		if h.canReadTool(r, user, &tool) {
			result = append(result, h.serializeTool(r, tool, user))
		}
	}
	return result
}
