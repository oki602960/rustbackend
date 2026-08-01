package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type MemoriesRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type MemoriesRouter struct {
	Config   MemoriesRuntimeConfig
	Users    *models.UsersTable
	Memories *models.MemoriesTable
}

type addMemoryForm struct {
	Content string `json:"content"`
}

type updateMemoryForm struct {
	Content string `json:"content"`
}

func (h *MemoriesRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/memories/", h.GetMemories)
	mux.HandleFunc("POST /api/v1/memories/add", h.AddMemory)
	mux.HandleFunc("DELETE /api/v1/memories/delete/user", h.DeleteMemoriesByUser)
	mux.HandleFunc("POST /api/v1/memories/{memory_id}/update", h.UpdateMemoryByID)
	mux.HandleFunc("DELETE /api/v1/memories/{memory_id}", h.DeleteMemoryByID)
}

func (h *MemoriesRouter) GetMemories(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	memories, err := h.Memories.GetMemoriesByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (h *MemoriesRouter) AddMemory(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form addMemoryForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	memory, err := h.Memories.InsertNewMemory(r.Context(), user.ID, form.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

func (h *MemoriesRouter) UpdateMemoryByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form updateMemoryForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	memory, err := h.Memories.UpdateMemoryByIDAndUserID(r.Context(), r.PathValue("memory_id"), user.ID, form.Content)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if memory == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

func (h *MemoriesRouter) DeleteMemoryByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	deleted, err := h.Memories.DeleteMemoryByIDAndUserID(r.Context(), r.PathValue("memory_id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *MemoriesRouter) DeleteMemoriesByUser(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	deleted, err := h.Memories.DeleteMemoriesByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *MemoriesRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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
