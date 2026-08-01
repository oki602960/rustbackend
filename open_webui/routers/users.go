package routers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type UsersRuntimeConfig struct {
	WebUISecretKey string
	StaticDir      string
	EnableAPIKeys  bool
}

type UsersRouter struct {
	Config        UsersRuntimeConfig
	Users         *models.UsersTable
	Auths         *models.AuthsTable
	Groups        *models.GroupsTable
	OAuthSessions *models.OAuthSessionsTable
}

type userInfoResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	ProfileImageURL string `json:"profile_image_url"`
	IsActive        bool   `json:"is_active"`
	Groups          []any  `json:"groups"`
}

type userGroupIDsResponse struct {
	ID              string   `json:"id"`
	Email           string   `json:"email"`
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	ProfileImageURL string   `json:"profile_image_url"`
	GroupIDs        []string `json:"group_ids"`
}

type userListResponse struct {
	Users []any `json:"users"`
	Total int   `json:"total"`
}

type userActiveResponse struct {
	ID                    string         `json:"id"`
	Email                 string         `json:"email"`
	Username              string         `json:"username,omitempty"`
	Role                  string         `json:"role"`
	Name                  string         `json:"name"`
	ProfileImageURL       string         `json:"profile_image_url"`
	ProfileBannerImageURL string         `json:"profile_banner_image_url,omitempty"`
	Bio                   string         `json:"bio,omitempty"`
	Gender                string         `json:"gender,omitempty"`
	DateOfBirth           string         `json:"date_of_birth,omitempty"`
	Timezone              string         `json:"timezone,omitempty"`
	PresenceState         string         `json:"presence_state,omitempty"`
	StatusEmoji           string         `json:"status_emoji,omitempty"`
	StatusMessage         string         `json:"status_message,omitempty"`
	StatusExpiresAt       *int64         `json:"status_expires_at,omitempty"`
	Info                  map[string]any `json:"info,omitempty"`
	Settings              map[string]any `json:"settings,omitempty"`
	OAuth                 map[string]any `json:"oauth,omitempty"`
	SCIM                  map[string]any `json:"scim,omitempty"`
	LastActiveAt          int64          `json:"last_active_at"`
	UpdatedAt             int64          `json:"updated_at"`
	CreatedAt             int64          `json:"created_at"`
	Groups                []any          `json:"groups"`
	IsActive              bool           `json:"is_active"`
}

type updateUserForm struct {
	Role            string  `json:"role"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	ProfileImageURL string  `json:"profile_image_url"`
	Password        *string `json:"password"`
}

type userStatusForm struct {
	StatusEmoji     *string `json:"status_emoji"`
	StatusMessage   *string `json:"status_message"`
	StatusExpiresAt *int64  `json:"status_expires_at"`
}

func (h *UsersRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/", h.ListUsers)
	mux.HandleFunc("GET /api/v1/users/all", h.GetAllUsers)
	mux.HandleFunc("GET /api/v1/users/search", h.SearchUsers)
	mux.HandleFunc("GET /api/v1/users/groups", h.GetSessionUserGroups)
	mux.HandleFunc("GET /api/v1/users/user/settings", h.GetSessionUserSettings)
	mux.HandleFunc("POST /api/v1/users/user/settings/update", h.UpdateSessionUserSettings)
	mux.HandleFunc("GET /api/v1/users/user/status", h.GetSessionUserStatus)
	mux.HandleFunc("POST /api/v1/users/user/status/update", h.UpdateSessionUserStatus)
	mux.HandleFunc("GET /api/v1/users/user/info", h.GetSessionUserInfo)
	mux.HandleFunc("POST /api/v1/users/user/info/update", h.UpdateSessionUserInfo)
	mux.HandleFunc("GET /api/v1/users/{user_id}", h.GetUserByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/active", h.GetUserActiveStatusByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/groups", h.GetUserGroupsByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/oauth/sessions", h.GetUserOAuthSessionsByID)
	mux.HandleFunc("POST /api/v1/users/{user_id}/update", h.UpdateUserByID)
	mux.HandleFunc("DELETE /api/v1/users/{user_id}", h.DeleteUserByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/info", h.GetUserInfoByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/profile/image", h.GetUserProfileImageByID)
}

func (h *UsersRouter) ListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}

	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 30
	skip := (page - 1) * limit
	users, total, err := h.Users.GetUsers(r.Context(), models.UserListOptions{
		Query:     r.URL.Query().Get("query"),
		OrderBy:   r.URL.Query().Get("order_by"),
		Direction: r.URL.Query().Get("direction"),
		Skip:      skip,
		Limit:     limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	result := make([]any, 0, len(users))
	for _, user := range users {
		groupIDs := h.getUserGroupIDs(r, user.ID)
		result = append(result, userGroupIDsResponse{
			ID:              user.ID,
			Email:           user.Email,
			Name:            user.Name,
			Role:            user.Role,
			ProfileImageURL: user.ProfileImageURL,
			GroupIDs:        groupIDs,
		})
	}
	writeJSON(w, http.StatusOK, userListResponse{Users: result, Total: total})
}

func (h *UsersRouter) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	h.writeUserSearchResult(w, r, models.UserListOptions{})
}

func (h *UsersRouter) SearchUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 30
	skip := (page - 1) * limit
	h.writeUserSearchResult(w, r, models.UserListOptions{
		Query:     r.URL.Query().Get("query"),
		OrderBy:   r.URL.Query().Get("order_by"),
		Direction: r.URL.Query().Get("direction"),
		Skip:      skip,
		Limit:     limit,
	})
}

func (h *UsersRouter) GetSessionUserGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if h.Groups == nil {
		writeJSON(w, http.StatusOK, []groupResponse{})
		return
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	response := make([]groupResponse, 0, len(groups))
	for _, group := range groups {
		memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
		response = append(response, serializeGroup(group, memberCount))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *UsersRouter) GetSessionUserSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user.Settings)
}

func (h *UsersRouter) UpdateSessionUserSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Users.UpdateUserSettingsByID(r.Context(), user.ID, payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated.Settings)
}

func (h *UsersRouter) GetSessionUserStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(user))
}

func (h *UsersRouter) UpdateSessionUserStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var form userStatusForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	updated, err := h.Users.UpdateUserByID(r.Context(), user.ID, models.UserUpdateParams{
		StatusEmoji:     form.StatusEmoji,
		StatusMessage:   form.StatusMessage,
		StatusExpiresAt: form.StatusExpiresAt,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(updated))
}

func (h *UsersRouter) GetSessionUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user.Info)
}

func (h *UsersRouter) UpdateSessionUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Users.UpdateUserInfoByID(r.Context(), user.ID, payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated.Info)
}

func (h *UsersRouter) GetUserInfoByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, userInfoResponse{
		ID:              user.ID,
		Name:            user.Name,
		Email:           user.Email,
		Role:            user.Role,
		ProfileImageURL: user.ProfileImageURL,
		IsActive:        models.IsActive(user),
		Groups:          h.getUserGroups(r, user.ID),
	})
}

func (h *UsersRouter) GetUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, serializeUser(user))
}

func (h *UsersRouter) GetUserActiveStatusByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	userID := r.PathValue("user_id")
	active, err := h.Users.IsUserActive(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": active})
}

func (h *UsersRouter) GetUserProfileImageByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	switch {
	case strings.HasPrefix(user.ProfileImageURL, "http://"), strings.HasPrefix(user.ProfileImageURL, "https://"):
		http.Redirect(w, r, user.ProfileImageURL, http.StatusFound)
		return
	case strings.HasPrefix(user.ProfileImageURL, "data:image"):
		header, base64Data, found := strings.Cut(user.ProfileImageURL, ",")
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image"})
			return
		}
		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image"})
			return
		}
		mediaType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
		w.Header().Set("Content-Type", mediaType)
		_, _ = io.Copy(w, bytes.NewReader(imageData))
		return
	default:
		profilePath := filepath.Join(h.Config.StaticDir, "user.png")
		if strings.TrimSpace(user.ProfileImageURL) == "/static/favicon.png" {
			profilePath = filepath.Join(h.Config.StaticDir, "favicon.png")
		}
		if _, err := os.Stat(profilePath); err != nil {
			profilePath = filepath.Join(h.Config.StaticDir, "user.png")
		}
		http.ServeFile(w, r, profilePath)
	}
}

func (h *UsersRouter) UpdateUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	var form updateUserForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	firstUser, err := h.Users.GetFirstUser(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "could not verify primary admin status"})
		return
	}
	if firstUser != nil && userID == firstUser.ID {
		if sessionUser.ID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
			return
		}
		if form.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
			return
		}
	}

	currentUser, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if currentUser == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	if !utils.ValidateProfileImageURL(form.ProfileImageURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image url"})
		return
	}

	form.Email = strings.ToLower(strings.TrimSpace(form.Email))
	emailUser, err := h.Users.GetUserByEmail(r.Context(), form.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if emailUser != nil && emailUser.ID != currentUser.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "email already taken"})
		return
	}

	if form.Password != nil && strings.TrimSpace(*form.Password) != "" {
		hashed, err := utils.GetPasswordHash(*form.Password)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
		if _, err := h.Auths.UpdateUserPasswordByID(r.Context(), userID, hashed); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	if _, err := h.Auths.UpdateEmailByID(r.Context(), userID, form.Email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	updatedUser, err := h.Users.UpdateUserByID(r.Context(), userID, models.UserUpdateParams{
		Role:            &form.Role,
		Name:            &form.Name,
		Email:           &form.Email,
		ProfileImageURL: &form.ProfileImageURL,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updatedUser == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(updatedUser))
}

func (h *UsersRouter) DeleteUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	firstUser, err := h.Users.GetFirstUser(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "could not verify primary admin status"})
		return
	}
	if firstUser != nil && userID == firstUser.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}
	if sessionUser.ID == userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	deleted, err := h.Auths.DeleteAuthByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "delete user error"})
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *UsersRouter) GetUserGroupsByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), r.PathValue("user_id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	response := make([]groupResponse, 0, len(groups))
	for _, group := range groups {
		memberCount, _ := h.Groups.GetGroupMemberCountByID(r.Context(), group.ID)
		response = append(response, serializeGroup(group, memberCount))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *UsersRouter) GetUserOAuthSessionsByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	if h.OAuthSessions == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	sessions, err := h.OAuthSessions.GetSessionsByUserID(r.Context(), r.PathValue("user_id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if len(sessions) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *UsersRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *UsersRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return nil, false
	}
	if user.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return nil, false
	}
	return user, true
}

func serializeUser(user *models.User) userActiveResponse {
	return userActiveResponse{
		ID:                    user.ID,
		Email:                 user.Email,
		Username:              user.Username,
		Role:                  user.Role,
		Name:                  user.Name,
		ProfileImageURL:       user.ProfileImageURL,
		ProfileBannerImageURL: user.ProfileBannerImageURL,
		Bio:                   user.Bio,
		Gender:                user.Gender,
		DateOfBirth:           user.DateOfBirth,
		Timezone:              user.Timezone,
		PresenceState:         user.PresenceState,
		StatusEmoji:           user.StatusEmoji,
		StatusMessage:         user.StatusMessage,
		StatusExpiresAt:       user.StatusExpiresAt,
		Info:                  user.Info,
		Settings:              user.Settings,
		OAuth:                 user.OAuth,
		SCIM:                  user.SCIM,
		LastActiveAt:          user.LastActiveAt,
		UpdatedAt:             user.UpdatedAt,
		CreatedAt:             user.CreatedAt,
		Groups:                []any{},
		IsActive:              models.IsActive(user),
	}
}

func (h *UsersRouter) getUserGroups(r *http.Request, userID string) []any {
	if h.Groups == nil {
		return []any{}
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), userID)
	if err != nil {
		return []any{}
	}
	result := make([]any, 0, len(groups))
	for _, group := range groups {
		result = append(result, map[string]any{
			"id":   group.ID,
			"name": group.Name,
		})
	}
	return result
}

func (h *UsersRouter) getUserGroupIDs(r *http.Request, userID string) []string {
	if h.Groups == nil {
		return []string{}
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), userID)
	if err != nil {
		return []string{}
	}
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		result = append(result, group.ID)
	}
	return result
}

func (h *UsersRouter) writeUserSearchResult(w http.ResponseWriter, r *http.Request, options models.UserListOptions) {
	users, total, err := h.Users.GetUsers(r.Context(), options)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	result := make([]any, 0, len(users))
	for _, user := range users {
		result = append(result, userInfoResponse{
			ID:              user.ID,
			Name:            user.Name,
			Email:           user.Email,
			Role:            user.Role,
			ProfileImageURL: user.ProfileImageURL,
			IsActive:        models.IsActive(&user),
			Groups:          h.getUserGroups(r, user.ID),
		})
	}
	writeJSON(w, http.StatusOK, userListResponse{Users: result, Total: total})
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}
