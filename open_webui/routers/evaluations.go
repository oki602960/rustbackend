package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type EvaluationsRuntimeConfig struct {
	WebUISecretKey              string
	EnableAPIKeys               bool
	EnableEvaluationArenaModels bool
	EvaluationArenaModels       []map[string]any
}

type EvaluationsRouter struct {
	Config    *EvaluationsRuntimeConfig
	Users     *models.UsersTable
	Feedbacks *models.FeedbacksTable
}

type feedbackForm struct {
	Type     string         `json:"type"`
	Data     map[string]any `json:"data,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
	Snapshot map[string]any `json:"snapshot,omitempty"`
}

type evaluationsConfigForm struct {
	EnableEvaluationArenaModels *bool            `json:"ENABLE_EVALUATION_ARENA_MODELS,omitempty"`
	EvaluationArenaModels       []map[string]any `json:"EVALUATION_ARENA_MODELS,omitempty"`
}

type feedbackListResponse struct {
	Items []models.Feedback `json:"items"`
	Total int               `json:"total"`
}

func (h *EvaluationsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/evaluations/config", h.GetConfig)
	mux.HandleFunc("POST /api/v1/evaluations/config", h.UpdateConfig)
	mux.HandleFunc("GET /api/v1/evaluations/feedbacks/all", h.GetAllFeedbacks)
	mux.HandleFunc("GET /api/v1/evaluations/feedbacks/all/ids", h.GetAllFeedbackIDs)
	mux.HandleFunc("GET /api/v1/evaluations/feedbacks/all/export", h.ExportAllFeedbacks)
	mux.HandleFunc("DELETE /api/v1/evaluations/feedbacks/all", h.DeleteAllFeedbacks)
	mux.HandleFunc("GET /api/v1/evaluations/feedbacks/user", h.GetUserFeedbacks)
	mux.HandleFunc("DELETE /api/v1/evaluations/feedbacks", h.DeleteUserFeedbacks)
	mux.HandleFunc("GET /api/v1/evaluations/feedbacks/list", h.GetFeedbackList)
	mux.HandleFunc("POST /api/v1/evaluations/feedback", h.CreateFeedback)
	mux.HandleFunc("GET /api/v1/evaluations/feedback/{id}", h.GetFeedbackByID)
	mux.HandleFunc("POST /api/v1/evaluations/feedback/{id}", h.UpdateFeedbackByID)
	mux.HandleFunc("DELETE /api/v1/evaluations/feedback/{id}", h.DeleteFeedbackByID)
}

func (h *EvaluationsRouter) GetConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ENABLE_EVALUATION_ARENA_MODELS": h.Config.EnableEvaluationArenaModels,
		"EVALUATION_ARENA_MODELS":        h.Config.EvaluationArenaModels,
	})
}

func (h *EvaluationsRouter) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form evaluationsConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if form.EnableEvaluationArenaModels != nil {
		h.Config.EnableEvaluationArenaModels = *form.EnableEvaluationArenaModels
	}
	if form.EvaluationArenaModels != nil {
		h.Config.EvaluationArenaModels = form.EvaluationArenaModels
	}
	h.GetConfig(w, r)
}

func (h *EvaluationsRouter) GetAllFeedbacks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	feedbacks, err := h.Feedbacks.GetAllFeedbacks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, feedbacks)
}

func (h *EvaluationsRouter) GetAllFeedbackIDs(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	feedbacks, err := h.Feedbacks.GetAllFeedbackIDs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, feedbacks)
}

func (h *EvaluationsRouter) DeleteAllFeedbacks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	deleted, err := h.Feedbacks.DeleteAllFeedbacks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *EvaluationsRouter) ExportAllFeedbacks(w http.ResponseWriter, r *http.Request) {
	h.GetAllFeedbacks(w, r)
}

func (h *EvaluationsRouter) GetUserFeedbacks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	feedbacks, err := h.Feedbacks.GetFeedbacksByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, feedbacks)
}

func (h *EvaluationsRouter) DeleteUserFeedbacks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	deleted, err := h.Feedbacks.DeleteFeedbacksByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *EvaluationsRouter) GetFeedbackList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 30
	skip := (page - 1) * limit
	items, total, err := h.Feedbacks.GetFeedbackItems(r.Context(), models.FeedbackListOptions{
		OrderBy:   r.URL.Query().Get("order_by"),
		Direction: r.URL.Query().Get("direction"),
		Skip:      skip,
		Limit:     limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, feedbackListResponse{Items: items, Total: total})
}

func (h *EvaluationsRouter) CreateFeedback(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form feedbackForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	feedback, err := h.Feedbacks.InsertNewFeedback(r.Context(), models.FeedbackCreateParams{
		UserID:   user.ID,
		Type:     form.Type,
		Data:     form.Data,
		Meta:     form.Meta,
		Snapshot: form.Snapshot,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

func (h *EvaluationsRouter) GetFeedbackByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var feedback *models.Feedback
	var err error
	if user.Role == "admin" {
		feedback, err = h.Feedbacks.GetFeedbackByID(r.Context(), r.PathValue("id"))
	} else {
		feedback, err = h.Feedbacks.GetFeedbackByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if feedback == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

func (h *EvaluationsRouter) UpdateFeedbackByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form feedbackForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	var feedback *models.Feedback
	var err error
	if user.Role == "admin" {
		feedback, err = h.Feedbacks.UpdateFeedbackByID(r.Context(), r.PathValue("id"), models.FeedbackUpdateParams{
			Type:     evaluationStringPtr(form.Type),
			Data:     form.Data,
			Meta:     form.Meta,
			Snapshot: form.Snapshot,
		})
	} else {
		feedback, err = h.Feedbacks.UpdateFeedbackByIDAndUserID(r.Context(), r.PathValue("id"), user.ID, models.FeedbackUpdateParams{
			Type:     evaluationStringPtr(form.Type),
			Data:     form.Data,
			Meta:     form.Meta,
			Snapshot: form.Snapshot,
		})
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if feedback == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

func (h *EvaluationsRouter) DeleteFeedbackByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var deleted bool
	var err error
	if user.Role == "admin" {
		deleted, err = h.Feedbacks.DeleteFeedbackByID(r.Context(), r.PathValue("id"))
	} else {
		deleted, err = h.Feedbacks.DeleteFeedbackByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *EvaluationsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *EvaluationsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func evaluationStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
