package routers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type FunctionsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type FunctionsRouter struct {
	Config    FunctionsRuntimeConfig
	Users     *models.UsersTable
	Functions *models.FunctionsTable
}

type functionForm struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func (h *FunctionsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/functions/", h.GetFunctions)
	mux.HandleFunc("GET /api/v1/functions/list", h.GetFunctionList)
	mux.HandleFunc("GET /api/v1/functions/export", h.ExportFunctions)
	mux.HandleFunc("POST /api/v1/functions/create", h.CreateFunction)
	mux.HandleFunc("GET /api/v1/functions/id/{id}", h.GetFunctionByID)
	mux.HandleFunc("POST /api/v1/functions/id/{id}/toggle", h.ToggleFunctionByID)
	mux.HandleFunc("POST /api/v1/functions/id/{id}/toggle/global", h.ToggleGlobalByID)
	mux.HandleFunc("POST /api/v1/functions/id/{id}/update", h.UpdateFunctionByID)
	mux.HandleFunc("DELETE /api/v1/functions/id/{id}/delete", h.DeleteFunctionByID)
}

func (h *FunctionsRouter) GetFunctions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	functions, err := h.Functions.GetFunctions(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, functions)
}

func (h *FunctionsRouter) GetFunctionList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	functions, err := h.Functions.GetFunctions(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, functions)
}

func (h *FunctionsRouter) ExportFunctions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	functions, err := h.Functions.GetFunctions(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, functions)
}

func (h *FunctionsRouter) CreateFunction(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAdminUser(w, r)
	if !ok {
		return
	}
	var form functionForm
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
	existing, err := h.Functions.GetFunctionByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "id taken"})
		return
	}
	function, err := h.Functions.InsertNewFunction(r.Context(), models.FunctionCreateParams{
		ID:      form.ID,
		UserID:  user.ID,
		Name:    form.Name,
		Type:    "native",
		Content: form.Content,
		Meta:    form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, function)
}

func (h *FunctionsRouter) GetFunctionByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	function, err := h.Functions.GetFunctionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if function == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, function)
}

func (h *FunctionsRouter) ToggleFunctionByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	function, err := h.Functions.GetFunctionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if function == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	next := !function.IsActive
	function, err = h.Functions.UpdateFunctionByID(r.Context(), function.ID, models.FunctionUpdateParams{IsActive: &next})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, function)
}

func (h *FunctionsRouter) ToggleGlobalByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	function, err := h.Functions.GetFunctionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if function == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	next := !function.IsGlobal
	function, err = h.Functions.UpdateFunctionByID(r.Context(), function.ID, models.FunctionUpdateParams{IsGlobal: &next})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, function)
}

func (h *FunctionsRouter) UpdateFunctionByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	function, err := h.Functions.GetFunctionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if function == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "not found"})
		return
	}
	var form functionForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Functions.UpdateFunctionByID(r.Context(), function.ID, models.FunctionUpdateParams{
		Name:    functionStringPtr(form.Name),
		Content: functionStringPtr(form.Content),
		Meta:    form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *FunctionsRouter) DeleteFunctionByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	deleted, err := h.Functions.DeleteFunctionByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *FunctionsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *FunctionsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func functionStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
