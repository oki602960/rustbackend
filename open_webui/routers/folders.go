package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type FoldersRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type FoldersRouter struct {
	Config  FoldersRuntimeConfig
	Users   *models.UsersTable
	Folders *models.FoldersTable
}

type folderForm struct {
	Name       string         `json:"name"`
	Data       map[string]any `json:"data,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	ParentID   *string        `json:"parent_id,omitempty"`
	IsExpanded *bool          `json:"is_expanded,omitempty"`
}

type folderParentForm struct {
	ParentID *string `json:"parent_id,omitempty"`
}

type folderExpandedForm struct {
	IsExpanded bool `json:"is_expanded"`
}

func (h *FoldersRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/folders/", h.GetFolders)
	mux.HandleFunc("POST /api/v1/folders/", h.CreateFolder)
	mux.HandleFunc("GET /api/v1/folders/{id}", h.GetFolderByID)
	mux.HandleFunc("POST /api/v1/folders/{id}/update", h.UpdateFolderByID)
	mux.HandleFunc("POST /api/v1/folders/{id}/update/parent", h.UpdateFolderParentByID)
	mux.HandleFunc("POST /api/v1/folders/{id}/update/expanded", h.UpdateFolderExpandedByID)
	mux.HandleFunc("DELETE /api/v1/folders/{id}", h.DeleteFolderByID)
}

func (h *FoldersRouter) GetFolders(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	folders, err := h.Folders.GetFoldersByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

func (h *FoldersRouter) CreateFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var form folderForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	existing, err := h.Folders.GetFolderByParentIDAndUserIDAndName(r.Context(), form.ParentID, user.ID, form.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "folder already exists"})
		return
	}

	folder, err := h.Folders.InsertNewFolder(r.Context(), models.FolderCreateParams{
		UserID:     user.ID,
		ParentID:   form.ParentID,
		Name:       form.Name,
		Data:       form.Data,
		Meta:       form.Meta,
		IsExpanded: form.IsExpanded != nil && *form.IsExpanded,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

func (h *FoldersRouter) GetFolderByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	folder, err := h.Folders.GetFolderByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if folder == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, folder)
}

func (h *FoldersRouter) UpdateFolderByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	folder, err := h.Folders.GetFolderByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if folder == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}

	var form folderForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if form.Name != "" {
		existing, err := h.Folders.GetFolderByParentIDAndUserIDAndName(r.Context(), nullableStringPtr(folder.ParentID), user.ID, form.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		if existing != nil && existing.ID != folder.ID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "folder already exists"})
			return
		}
	}
	updated, err := h.Folders.UpdateFolderByIDAndUserID(r.Context(), folder.ID, user.ID, models.FolderUpdateParams{
		Name: folderNamePtr(form.Name),
		Data: form.Data,
		Meta: form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *FoldersRouter) UpdateFolderParentByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	folder, err := h.Folders.GetFolderByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if folder == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form folderParentForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	existing, err := h.Folders.GetFolderByParentIDAndUserIDAndName(r.Context(), form.ParentID, user.ID, folder.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil && existing.ID != folder.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "folder already exists"})
		return
	}
	updated, err := h.Folders.UpdateFolderByIDAndUserID(r.Context(), folder.ID, user.ID, models.FolderUpdateParams{
		ParentID: form.ParentID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *FoldersRouter) UpdateFolderExpandedByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form folderExpandedForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Folders.UpdateFolderByIDAndUserID(r.Context(), r.PathValue("id"), user.ID, models.FolderUpdateParams{
		IsExpanded: &form.IsExpanded,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *FoldersRouter) DeleteFolderByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	deleted, err := h.Folders.DeleteFolderByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *FoldersRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func folderNamePtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
