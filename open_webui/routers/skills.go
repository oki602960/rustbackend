package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type SkillsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type SkillsRouter struct {
	Config       SkillsRuntimeConfig
	Users        *models.UsersTable
	Skills       *models.SkillsTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
}

type skillForm struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Content     string         `json:"content"`
	Meta        map[string]any `json:"meta,omitempty"`
	IsActive    bool           `json:"is_active"`
}

type skillListResponse struct {
	Items []models.Skill `json:"items"`
	Total int            `json:"total"`
}

type skillAccessResponse struct {
	models.Skill
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type skillAccessGrantsForm struct {
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *SkillsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/skills/", h.GetSkills)
	mux.HandleFunc("GET /api/v1/skills/list", h.GetSkillList)
	mux.HandleFunc("GET /api/v1/skills/export", h.ExportSkills)
	mux.HandleFunc("POST /api/v1/skills/create", h.CreateSkill)
	mux.HandleFunc("GET /api/v1/skills/id/{id}", h.GetSkillByID)
	mux.HandleFunc("POST /api/v1/skills/id/{id}/access/update", h.UpdateSkillAccessByID)
	mux.HandleFunc("POST /api/v1/skills/id/{id}/toggle", h.ToggleSkillByID)
	mux.HandleFunc("POST /api/v1/skills/id/{id}/update", h.UpdateSkillByID)
	mux.HandleFunc("DELETE /api/v1/skills/id/{id}", h.DeleteSkillByID)
}

func (h *SkillsRouter) GetSkills(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		skills, err := h.Skills.GetSkills(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, h.serializeSkills(r, skills, user))
		return
	}
	skills, err := h.Skills.GetSkills(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.filterReadableSkills(r, skills, user))
}

func (h *SkillsRouter) GetSkillList(w http.ResponseWriter, r *http.Request) {
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
	items, _, err := h.Skills.SearchSkills(r.Context(), user.ID, models.SkillSearchOptions{
		Query: r.URL.Query().Get("query"),
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	filtered := h.filterReadableSkills(r, items, user)
	writeJSON(w, http.StatusOK, skillListResponse{Items: extractSkills(filtered), Total: len(filtered)})
}

func (h *SkillsRouter) ExportSkills(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		skills, err := h.Skills.GetSkills(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, skills)
		return
	}
	skills, err := h.Skills.GetSkills(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, extractSkills(h.filterReadableSkills(r, skills, user)))
}

func (h *SkillsRouter) CreateSkill(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form skillForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	form.ID = strings.ToLower(strings.ReplaceAll(form.ID, " ", "-"))
	existing, err := h.Skills.GetSkillByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "id taken"})
		return
	}
	skill, err := h.Skills.InsertNewSkill(r.Context(), models.SkillCreateParams{
		ID:          form.ID,
		UserID:      user.ID,
		Name:        form.Name,
		Description: form.Description,
		Content:     form.Content,
		Meta:        form.Meta,
		IsActive:    form.IsActive,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *SkillsRouter) GetSkillByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	skill, err := h.Skills.GetSkillByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if skill == nil || !h.canReadSkill(r, user, skill) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeSkill(r, *skill, user))
}

func (h *SkillsRouter) UpdateSkillAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Skills.GetSkillByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteSkill(r, user, current) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	var form skillAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "skill", current.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Skills.GetSkillByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeSkill(r, *updated, user))
}

func (h *SkillsRouter) ToggleSkillByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Skills.GetSkillByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteSkill(r, user, current) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	next := !current.IsActive
	updated, err := h.Skills.UpdateSkillByID(r.Context(), current.ID, models.SkillUpdateParams{IsActive: &next})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *SkillsRouter) UpdateSkillByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Skills.GetSkillByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteSkill(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form skillForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Skills.UpdateSkillByID(r.Context(), current.ID, models.SkillUpdateParams{
		Name:        skillStringPtr(form.Name),
		Description: skillStringPtrAllowEmpty(form.Description),
		Content:     skillStringPtr(form.Content),
		Meta:        form.Meta,
		IsActive:    &form.IsActive,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *SkillsRouter) DeleteSkillByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Skills.GetSkillByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteSkill(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Skills.DeleteSkillByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *SkillsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func skillStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func skillStringPtrAllowEmpty(value string) *string {
	return &value
}

func (h *SkillsRouter) userGroupIDs(r *http.Request, userID string) []string {
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

func (h *SkillsRouter) canReadSkill(r *http.Request, user *models.User, skill *models.Skill) bool {
	if user.Role == "admin" || skill.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "skill", skill.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *SkillsRouter) canWriteSkill(r *http.Request, user *models.User, skill *models.Skill) bool {
	if user.Role == "admin" || skill.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "skill", skill.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *SkillsRouter) serializeSkill(r *http.Request, skill models.Skill, user *models.User) skillAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "skill", skill.ID)
		if err == nil {
			grants = loaded
		}
	}
	return skillAccessResponse{
		Skill:        skill,
		WriteAccess:  h.canWriteSkill(r, user, &skill),
		AccessGrants: grants,
	}
}

func (h *SkillsRouter) serializeSkills(r *http.Request, skills []models.Skill, user *models.User) []skillAccessResponse {
	result := make([]skillAccessResponse, 0, len(skills))
	for _, skill := range skills {
		result = append(result, h.serializeSkill(r, skill, user))
	}
	return result
}

func (h *SkillsRouter) filterReadableSkills(r *http.Request, skills []models.Skill, user *models.User) []skillAccessResponse {
	result := make([]skillAccessResponse, 0, len(skills))
	for _, skill := range skills {
		if h.canReadSkill(r, user, &skill) {
			result = append(result, h.serializeSkill(r, skill, user))
		}
	}
	return result
}

func extractSkills(items []skillAccessResponse) []models.Skill {
	result := make([]models.Skill, 0, len(items))
	for _, item := range items {
		result = append(result, item.Skill)
	}
	return result
}
