package routers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type ModelsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
	StaticDir      string
}

type ModelsRouter struct {
	Config       ModelsRuntimeConfig
	Users        *models.UsersTable
	Models       *models.ModelsTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
}

type modelForm struct {
	ID          string         `json:"id"`
	BaseModelID *string        `json:"base_model_id,omitempty"`
	Name        string         `json:"name"`
	Meta        map[string]any `json:"meta,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	IsActive    bool           `json:"is_active"`
}

type modelsImportForm struct {
	Models []modelForm `json:"models"`
}

type modelListResponse struct {
	Items []models.Model `json:"items"`
	Total int            `json:"total"`
}

type modelAccessResponse struct {
	models.Model
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type modelAccessGrantsForm struct {
	ID           string           `json:"id"`
	Name         *string          `json:"name,omitempty"`
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *ModelsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/models/list", h.GetModelsList)
	mux.HandleFunc("GET /api/v1/models/base", h.GetBaseModels)
	mux.HandleFunc("GET /api/v1/models/tags", h.GetModelTags)
	mux.HandleFunc("POST /api/v1/models/create", h.CreateModel)
	mux.HandleFunc("GET /api/v1/models/export", h.ExportModels)
	mux.HandleFunc("POST /api/v1/models/import", h.ImportModels)
	mux.HandleFunc("GET /api/v1/models/model", h.GetModelByID)
	mux.HandleFunc("GET /api/v1/models/model/profile/image", h.GetModelProfileImage)
	mux.HandleFunc("POST /api/v1/models/model/toggle", h.ToggleModelByID)
	mux.HandleFunc("POST /api/v1/models/model/update", h.UpdateModelByID)
	mux.HandleFunc("POST /api/v1/models/model/access/update", h.UpdateModelAccessByID)
	mux.HandleFunc("POST /api/v1/models/model/delete", h.DeleteModelByID)
	mux.HandleFunc("DELETE /api/v1/models/delete/all", h.DeleteAllModels)
}

func (h *ModelsRouter) GetModelsList(w http.ResponseWriter, r *http.Request) {
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
	userID := ""
	if user.Role != "admin" {
		userID = user.ID
	}
	items, _, err := h.Models.SearchModels(r.Context(), userID, models.ModelSearchOptions{
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
	filtered := h.filterReadableModels(r, items, user)
	writeJSON(w, http.StatusOK, modelListResponse{Items: extractModels(filtered), Total: len(filtered)})
}

func (h *ModelsRouter) GetBaseModels(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAdminUser(w, r)
	if !ok {
		return
	}
	items, err := h.Models.GetModels(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeModels(r, items, user))
}

func (h *ModelsRouter) GetModelTags(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	tags, err := h.Models.GetTags(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (h *ModelsRouter) CreateModel(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form modelForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	existing, err := h.Models.GetModelByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "model id taken"})
		return
	}
	model, err := h.Models.InsertNewModel(r.Context(), models.ModelCreateParams{
		ID:          form.ID,
		UserID:      user.ID,
		BaseModelID: form.BaseModelID,
		Name:        form.Name,
		Params:      form.Params,
		Meta:        form.Meta,
		IsActive:    form.IsActive,
	})
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (h *ModelsRouter) ExportModels(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		items, err := h.Models.GetModels(r.Context(), false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, h.serializeModels(r, items, user))
		return
	}
	items, err := h.Models.GetModels(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, extractModels(h.filterReadableModels(r, items, user)))
}

func (h *ModelsRouter) ImportModels(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form modelsImportForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	for _, modelData := range form.Models {
		existing, err := h.Models.GetModelByID(r.Context(), modelData.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		if existing != nil {
			_, err = h.Models.UpdateModelByID(r.Context(), modelData.ID, models.ModelUpdateParams{
				BaseModelID: modelData.BaseModelID,
				Name:        modelStringPtr(modelData.Name),
				Params:      modelData.Params,
				Meta:        modelData.Meta,
				IsActive:    &modelData.IsActive,
			})
		} else {
			_, err = h.Models.InsertNewModel(r.Context(), models.ModelCreateParams{
				ID:          modelData.ID,
				UserID:      user.ID,
				BaseModelID: modelData.BaseModelID,
				Name:        modelData.Name,
				Params:      modelData.Params,
				Meta:        modelData.Meta,
				IsActive:    modelData.IsActive,
			})
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *ModelsRouter) GetModelByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	model, err := h.Models.GetModelByID(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if model == nil || !h.canReadModel(r, user, model) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeModel(r, *model, user))
}

func (h *ModelsRouter) GetModelProfileImage(w http.ResponseWriter, r *http.Request) {
	model, _ := h.Models.GetModelByID(r.Context(), r.URL.Query().Get("id"))
	if model == nil {
		http.ServeFile(w, r, filepath.Join(h.Config.StaticDir, "favicon.png"))
		return
	}
	if profileImageURL, ok := model.Meta["profile_image_url"].(string); ok && profileImageURL != "" {
		if strings.HasPrefix(profileImageURL, "http://") || strings.HasPrefix(profileImageURL, "https://") {
			http.Redirect(w, r, profileImageURL, http.StatusFound)
			return
		}
		if strings.HasPrefix(profileImageURL, "data:image") {
			header, base64Data, found := strings.Cut(profileImageURL, ",")
			if found {
				imageData, err := base64.StdEncoding.DecodeString(base64Data)
				if err == nil {
					mediaType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
					w.Header().Set("Content-Type", mediaType)
					_, _ = io.Copy(w, bytes.NewReader(imageData))
					return
				}
			}
		}
	}
	http.ServeFile(w, r, filepath.Join(h.Config.StaticDir, "favicon.png"))
}

func (h *ModelsRouter) ToggleModelByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	model, err := h.Models.GetModelByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if model == nil || !h.canWriteModel(r, user, model) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	model, err = h.Models.ToggleModelByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (h *ModelsRouter) UpdateModelAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form modelAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	model, err := h.Models.GetModelByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if model == nil {
		if user.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "access prohibited"})
			return
		}
		name := form.ID
		if form.Name != nil && *form.Name != "" {
			name = *form.Name
		}
		model, err = h.Models.InsertNewModel(r.Context(), models.ModelCreateParams{
			ID:       form.ID,
			UserID:   user.ID,
			Name:     name,
			Params:   map[string]any{},
			Meta:     map[string]any{},
			IsActive: true,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}
	if !h.canWriteModel(r, user, model) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "access prohibited"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "model", model.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Models.GetModelByID(r.Context(), model.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeModel(r, *updated, user))
}

func (h *ModelsRouter) UpdateModelByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form modelForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	model, err := h.Models.GetModelByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if model == nil || !h.canWriteModel(r, user, model) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	model, err = h.Models.UpdateModelByID(r.Context(), form.ID, models.ModelUpdateParams{
		BaseModelID: form.BaseModelID,
		Name:        modelStringPtr(form.Name),
		Params:      form.Params,
		Meta:        form.Meta,
		IsActive:    &form.IsActive,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (h *ModelsRouter) DeleteModelByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	model, err := h.Models.GetModelByID(r.Context(), payload.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if model == nil || (user.Role != "admin" && model.UserID != user.ID) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Models.DeleteModelByID(r.Context(), payload.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *ModelsRouter) DeleteAllModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	items, err := h.Models.GetModels(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	for _, model := range items {
		if _, err := h.Models.DeleteModelByID(r.Context(), model.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *ModelsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *ModelsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func modelStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *ModelsRouter) userGroupIDs(r *http.Request, userID string) []string {
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

func (h *ModelsRouter) canReadModel(r *http.Request, user *models.User, model *models.Model) bool {
	if user.Role == "admin" || model.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "model", model.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *ModelsRouter) canWriteModel(r *http.Request, user *models.User, model *models.Model) bool {
	if user.Role == "admin" || model.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "model", model.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *ModelsRouter) serializeModel(r *http.Request, model models.Model, user *models.User) modelAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "model", model.ID)
		if err == nil {
			grants = loaded
		}
	}
	return modelAccessResponse{
		Model:        model,
		WriteAccess:  h.canWriteModel(r, user, &model),
		AccessGrants: grants,
	}
}

func (h *ModelsRouter) serializeModels(r *http.Request, items []models.Model, user *models.User) []modelAccessResponse {
	result := make([]modelAccessResponse, 0, len(items))
	for _, item := range items {
		result = append(result, h.serializeModel(r, item, user))
	}
	return result
}

func (h *ModelsRouter) filterReadableModels(r *http.Request, items []models.Model, user *models.User) []modelAccessResponse {
	result := make([]modelAccessResponse, 0, len(items))
	for _, item := range items {
		if h.canReadModel(r, user, &item) {
			result = append(result, h.serializeModel(r, item, user))
		}
	}
	return result
}

func extractModels(items []modelAccessResponse) []models.Model {
	result := make([]models.Model, 0, len(items))
	for _, item := range items {
		result = append(result, item.Model)
	}
	return result
}
