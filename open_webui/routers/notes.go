package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type NotesRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type NotesRouter struct {
	Config       NotesRuntimeConfig
	Users        *models.UsersTable
	Notes        *models.NotesTable
	Groups       *models.GroupsTable
	AccessGrants *models.AccessGrantsTable
}

type noteForm struct {
	Title string         `json:"title"`
	Data  map[string]any `json:"data,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type noteListResponse struct {
	Items []models.Note `json:"items"`
	Total int           `json:"total"`
}

type noteAccessResponse struct {
	models.Note
	WriteAccess  bool                 `json:"write_access"`
	AccessGrants []models.AccessGrant `json:"access_grants,omitempty"`
}

type noteAccessGrantsForm struct {
	AccessGrants []map[string]any `json:"access_grants"`
}

func (h *NotesRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notes/", h.GetNotes)
	mux.HandleFunc("GET /api/v1/notes/search", h.SearchNotes)
	mux.HandleFunc("POST /api/v1/notes/create", h.CreateNote)
	mux.HandleFunc("GET /api/v1/notes/{id}", h.GetNoteByID)
	mux.HandleFunc("POST /api/v1/notes/{id}/access/update", h.UpdateNoteAccessByID)
	mux.HandleFunc("POST /api/v1/notes/{id}/update", h.UpdateNoteByID)
	mux.HandleFunc("DELETE /api/v1/notes/{id}", h.DeleteNoteByID)
}

func (h *NotesRouter) GetNotes(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 0)
	limit := 0
	skip := 0
	if page > 0 {
		limit = 60
		skip = (page - 1) * limit
	}
	notes, err := h.Notes.GetNotesByUserID(r.Context(), user.ID, skip, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeNotes(r, notes, user))
}

func (h *NotesRouter) SearchNotes(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 60
	skip := (page - 1) * limit
	notes, _, err := h.Notes.SearchNotes(r.Context(), user.ID, models.NoteListOptions{
		Query: r.URL.Query().Get("query"),
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	filtered := h.filterReadableNotes(r, notes, user)
	writeJSON(w, http.StatusOK, noteListResponse{Items: extractNotes(filtered), Total: len(filtered)})
}

func (h *NotesRouter) CreateNote(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form noteForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	note, err := h.Notes.InsertNewNote(r.Context(), models.NoteCreateParams{
		UserID: user.ID,
		Title:  form.Title,
		Data:   form.Data,
		Meta:   form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *NotesRouter) GetNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	note, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if note == nil || !h.canReadNote(r, user, note) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeNote(r, *note, user))
}

func (h *NotesRouter) UpdateNoteAccessByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteNote(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form noteAccessGrantsForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if err := h.AccessGrants.SetAccessGrants(r.Context(), "note", current.ID, form.AccessGrants); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Notes.GetNoteByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.serializeNote(r, *updated, user))
}

func (h *NotesRouter) UpdateNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteNote(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}

	var form noteForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	note, err := h.Notes.UpdateNoteByID(r.Context(), current.ID, models.NoteUpdateParams{
		Title: noteStringPtr(form.Title),
		Data:  form.Data,
		Meta:  form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *NotesRouter) DeleteNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || !h.canWriteNote(r, user, current) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Notes.DeleteNoteByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *NotesRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func noteStringPtr(value string) *string {
	return &value
}

func (h *NotesRouter) userGroupIDs(r *http.Request, userID string) []string {
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

func (h *NotesRouter) canReadNote(r *http.Request, user *models.User, note *models.Note) bool {
	if user.Role == "admin" || note.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "note", note.ID, "read", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *NotesRouter) canWriteNote(r *http.Request, user *models.User, note *models.Note) bool {
	if user.Role == "admin" || note.UserID == user.ID {
		return true
	}
	if h.AccessGrants == nil {
		return false
	}
	allowed, err := h.AccessGrants.HasAccess(r.Context(), user.ID, "note", note.ID, "write", h.userGroupIDs(r, user.ID))
	return err == nil && allowed
}

func (h *NotesRouter) serializeNote(r *http.Request, note models.Note, user *models.User) noteAccessResponse {
	grants := []models.AccessGrant{}
	if h.AccessGrants != nil {
		loaded, err := h.AccessGrants.GetGrantsByResource(r.Context(), "note", note.ID)
		if err == nil {
			grants = loaded
		}
	}
	return noteAccessResponse{
		Note:         note,
		WriteAccess:  h.canWriteNote(r, user, &note),
		AccessGrants: grants,
	}
}

func (h *NotesRouter) serializeNotes(r *http.Request, notes []models.Note, user *models.User) []noteAccessResponse {
	result := make([]noteAccessResponse, 0, len(notes))
	for _, note := range notes {
		result = append(result, h.serializeNote(r, note, user))
	}
	return result
}

func (h *NotesRouter) filterReadableNotes(r *http.Request, notes []models.Note, user *models.User) []noteAccessResponse {
	result := make([]noteAccessResponse, 0, len(notes))
	for _, note := range notes {
		if h.canReadNote(r, user, &note) {
			result = append(result, h.serializeNote(r, note, user))
		}
	}
	return result
}

func extractNotes(items []noteAccessResponse) []models.Note {
	result := make([]models.Note, 0, len(items))
	for _, item := range items {
		result = append(result, item.Note)
	}
	return result
}
