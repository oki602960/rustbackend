package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type GroupsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type GroupsRouter struct {
	Config GroupsRuntimeConfig
	Users  *models.UsersTable
	Groups *models.GroupsTable
}

type groupForm struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Permissions map[string]any `json:"permissions,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type userIDsForm struct {
	UserIDs []string `json:"user_ids"`
}

type groupResponse struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	Permissions map[string]any `json:"permissions,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
	MemberCount int            `json:"member_count"`
}

type groupExportResponse struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	Permissions map[string]any `json:"permissions,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
	MemberCount int            `json:"member_count"`
	UserIDs     []string       `json:"user_ids"`
}

func (h *GroupsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/groups/", h.GetGroups)
	mux.HandleFunc("POST /api/v1/groups/create", h.CreateGroup)
	mux.HandleFunc("GET /api/v1/groups/id/{id}", h.GetGroupByID)
	mux.HandleFunc("GET /api/v1/groups/id/{id}/info", h.GetGroupInfoByID)
	mux.HandleFunc("GET /api/v1/groups/id/{id}/export", h.ExportGroupByID)
	mux.HandleFunc("POST /api/v1/groups/id/{id}/users", h.GetUsersInGroup)
	mux.HandleFunc("POST /api/v1/groups/id/{id}/update", h.UpdateGroupByID)
	mux.HandleFunc("POST /api/v1/groups/id/{id}/users/add", h.AddUsersToGroup)
	mux.HandleFunc("POST /api/v1/groups/id/{id}/users/remove", h.RemoveUsersFromGroup)
	mux.HandleFunc("DELETE /api/v1/groups/id/{id}", h.DeleteGroupByID)
}

func (h *GroupsRouter) GetGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	memberID := ""
	if user.Role != "admin" {
		memberID = user.ID
	}
	groups, err := h.Groups.GetGroups(r.Context(), memberID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	response := make([]groupResponse, 0, len(groups))
	for _, group := range groups {
		memberCount, err := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		response = append(response, serializeGroup(group, memberCount))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *GroupsRouter) CreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAdminUser(w, r)
	if !ok {
		return
	}

	var form groupForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	group, err := h.Groups.InsertNewGroup(r.Context(), models.GroupCreateParams{
		UserID:      user.ID,
		Name:        form.Name,
		Description: form.Description,
		Data:        form.Data,
		Meta:        form.Meta,
		Permissions: form.Permissions,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, serializeGroup(*group, memberCount))
}

func (h *GroupsRouter) GetGroupByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	h.writeGroupByID(w, r)
}

func (h *GroupsRouter) GetGroupInfoByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	h.writeGroupByID(w, r)
}

func (h *GroupsRouter) ExportGroupByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	group, err := h.Groups.GetGroupByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	userIDs, _ := h.Groups.GetGroupUserIDsByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, groupExportResponse{
		ID:          group.ID,
		UserID:      group.UserID,
		Name:        group.Name,
		Description: group.Description,
		Data:        group.Data,
		Meta:        group.Meta,
		Permissions: group.Permissions,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
		MemberCount: memberCount,
		UserIDs:     userIDs,
	})
}

func (h *GroupsRouter) GetUsersInGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	users, err := h.Users.GetUsersByGroupID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	result := make([]userInfoResponse, 0, len(users))
	for _, user := range users {
		result = append(result, userInfoResponse{
			ID:              user.ID,
			Name:            user.Name,
			Email:           user.Email,
			Role:            user.Role,
			ProfileImageURL: user.ProfileImageURL,
			IsActive:        models.IsActive(&user),
			Groups:          []any{},
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *GroupsRouter) UpdateGroupByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	var form groupForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	groupID := r.PathValue("id")
	group, err := h.Groups.UpdateGroupByID(r.Context(), groupID, models.GroupUpdateParams{
		Name:        groupStringPtr(form.Name),
		Description: groupStringPtr(form.Description),
		Data:        form.Data,
		Meta:        form.Meta,
		Permissions: form.Permissions,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "not found"})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, serializeGroup(*group, memberCount))
}

func (h *GroupsRouter) AddUsersToGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	var form userIDsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	validUserIDs, err := h.Users.GetValidUserIDs(r.Context(), form.UserIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	group, err := h.Groups.AddUsersToGroup(r.Context(), r.PathValue("id"), validUserIDs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "not found"})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, serializeGroup(*group, memberCount))
}

func (h *GroupsRouter) RemoveUsersFromGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	var form userIDsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	group, err := h.Groups.RemoveUsersFromGroup(r.Context(), r.PathValue("id"), form.UserIDs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "not found"})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, serializeGroup(*group, memberCount))
}

func (h *GroupsRouter) DeleteGroupByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	deleted, err := h.Groups.DeleteGroupByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *GroupsRouter) writeGroupByID(w http.ResponseWriter, r *http.Request) {
	group, err := h.Groups.GetGroupByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if group == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
	writeJSON(w, http.StatusOK, serializeGroup(*group, memberCount))
}

func (h *GroupsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
			return nil, false
		}
		return user, true
	}
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
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	return user, true
}

func (h *GroupsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func serializeGroup(group models.Group, memberCount int) groupResponse {
	return groupResponse{
		ID:          group.ID,
		UserID:      group.UserID,
		Name:        group.Name,
		Description: group.Description,
		Data:        group.Data,
		Meta:        group.Meta,
		Permissions: group.Permissions,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
		MemberCount: memberCount,
	}
}

func groupStringPtr(value string) *string {
	return &value
}
