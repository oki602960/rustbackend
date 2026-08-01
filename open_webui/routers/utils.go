package routers

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type UtilsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
	DatabaseURL    string
}

type UtilsRouter struct {
	Config UtilsRuntimeConfig
	Users  *models.UsersTable
}

type codeForm struct {
	Code string `json:"code"`
}

type markdownForm struct {
	MD string `json:"md"`
}

func (h *UtilsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/utils/gravatar", h.GetGravatar)
	mux.HandleFunc("POST /api/v1/utils/code/format", h.FormatCode)
	mux.HandleFunc("POST /api/v1/utils/markdown", h.MarkdownToHTML)
	mux.HandleFunc("GET /api/v1/utils/db/download", h.DownloadDB)
}

func (h *UtilsRouter) GetGravatar(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	hash := md5.Sum([]byte(email))
	writeJSON(w, http.StatusOK, map[string]string{
		"url": "https://www.gravatar.com/avatar/" + hex.EncodeToString(hash[:]) + "?d=mp",
	})
}

func (h *UtilsRouter) FormatCode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form codeForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": form.Code})
}

func (h *UtilsRouter) MarkdownToHTML(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	var form markdownForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	html := "<p>" + strings.ReplaceAll(strings.ReplaceAll(form.MD, "&", "&amp;"), "<", "&lt;") + "</p>"
	writeJSON(w, http.StatusOK, map[string]string{"html": html})
}

func (h *UtilsRouter) DownloadDB(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	if !strings.HasPrefix(h.Config.DatabaseURL, "sqlite:///") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "db not sqlite"})
		return
	}
	dbPath := strings.TrimPrefix(h.Config.DatabaseURL, "sqlite:///")
	if _, err := os.Stat(dbPath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	http.ServeFile(w, r, filepath.Clean(dbPath))
}

func (h *UtilsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *UtilsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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
