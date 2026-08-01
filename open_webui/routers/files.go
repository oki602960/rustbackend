package routers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/storage"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type FilesRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type FilesRouter struct {
	Config       FilesRuntimeConfig
	Users        *models.UsersTable
	Files        *models.FilesTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
	Storage      storage.Provider
}

type fileListResponse struct {
	Items []models.File `json:"items"`
	Total int           `json:"total"`
}

type fileAccessResponse struct {
	models.File
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type fileAccessGrantsForm struct {
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *FilesRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/files/", h.UploadFile)
	mux.HandleFunc("GET /api/v1/files/", h.GetFiles)
	mux.HandleFunc("GET /api/v1/files/search", h.SearchFiles)
	mux.HandleFunc("GET /api/v1/files/{id}", h.GetFileByID)
	mux.HandleFunc("POST /api/v1/files/{id}/access/update", h.UpdateFileAccessByID)
	mux.HandleFunc("GET /api/v1/files/{id}/content", h.GetFileContentByID)
	mux.HandleFunc("DELETE /api/v1/files/{id}", h.DeleteFileByID)
	mux.HandleFunc("DELETE /api/v1/files/all", h.DeleteAllFiles)
}

func (h *FilesRouter) UploadFile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid multipart body"})
		return
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "missing file"})
		return
	}
	defer file.Close()

	metadata := map[string]any{}
	if raw := r.FormValue("metadata"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &metadata)
	}

	id := uuid.NewString()
	storedFilename := id + "_" + filepath.Base(fileHeader.Filename)
	contents, filePath, err := h.Storage.UploadFile(file, storedFilename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	fileModel, err := h.Files.InsertNewFile(r.Context(), models.FileCreateParams{
		ID:       id,
		UserID:   user.ID,
		Filename: fileHeader.Filename,
		Path:     filePath,
		Data:     map[string]any{},
		Meta: map[string]any{
			"name":         fileHeader.Filename,
			"content_type": fileHeader.Header.Get("Content-Type"),
			"size":         len(contents),
			"data":         metadata,
		},
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeFile(r, *fileModel, user))
}

func (h *FilesRouter) GetFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 50
	skip := (page - 1) * limit
	items, _, err := h.Files.GetFileList(r.Context(), models.FileListOptions{Skip: skip, Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	filtered := h.filterReadableFiles(r, items, user)
	writeJSON(w, http.StatusOK, fileListResponse{Items: extractFiles(filtered), Total: len(filtered)})
}

func (h *FilesRouter) SearchFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	items, _, err := h.Files.GetFileList(r.Context(), models.FileListOptions{Query: r.URL.Query().Get("query"), Limit: 100})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, extractFiles(h.filterReadableFiles(r, items, user)))
}

func (h *FilesRouter) GetFileByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil || !h.canReadFile(r, user, file) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeFile(r, *file, user))
}

func (h *FilesRouter) UpdateFileAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Files.GetFileByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteFile(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form fileAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "file", current.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Files.GetFileByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeFile(r, *updated, user))
}

func (h *FilesRouter) GetFileContentByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil || !h.canReadFile(r, user, file) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	localPath, err := h.Storage.GetFile(file.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	http.ServeFile(w, r, localPath)
}

func (h *FilesRouter) DeleteFileByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil || !h.canWriteFile(r, user, file) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	if err := h.Storage.DeleteFile(file.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	deleted, err := h.Files.DeleteFileByID(r.Context(), file.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *FilesRouter) DeleteAllFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	items, _, err := h.Files.GetFileList(r.Context(), models.FileListOptions{Limit: 10000})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	for _, file := range items {
		if !h.canWriteFile(r, user, &file) {
			continue
		}
		if err := h.Storage.DeleteFile(file.Path); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
		if _, err := h.Files.DeleteFileByID(r.Context(), file.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *FilesRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *FilesRouter) userGroupIDs(r *http.Request, userID string) []string {
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

func (h *FilesRouter) canReadFile(r *http.Request, user *models.User, file *models.File) bool {
	if user.Role == "admin" || file.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "file", file.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *FilesRouter) canWriteFile(r *http.Request, user *models.User, file *models.File) bool {
	if user.Role == "admin" || file.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "file", file.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *FilesRouter) serializeFile(r *http.Request, file models.File, user *models.User) fileAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "file", file.ID)
		if err == nil {
			grants = loaded
		}
	}
	return fileAccessResponse{
		File:         file,
		WriteAccess:  h.canWriteFile(r, user, &file),
		AccessGrants: grants,
	}
}

func (h *FilesRouter) filterReadableFiles(r *http.Request, items []models.File, user *models.User) []fileAccessResponse {
	result := make([]fileAccessResponse, 0, len(items))
	for _, item := range items {
		if h.canReadFile(r, user, &item) {
			result = append(result, h.serializeFile(r, item, user))
		}
	}
	return result
}

func extractFiles(items []fileAccessResponse) []models.File {
	result := make([]models.File, 0, len(items))
	for _, item := range items {
		result = append(result, item.File)
	}
	return result
}
