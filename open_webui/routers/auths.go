package routers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type AuthRuntimeConfig struct {
	WebUIAuth                bool
	EnableInitialAdminSignup bool
	EnablePasswordAuth       bool
	EnableAPIKeys            bool
	EnableSignup             bool
	DefaultUserRole          string
	ShowAdminDetails         bool
	AdminEmail               string
	WebUISecretKey           string
	JWTExpiresIn             string
	AuthCookieSameSite       string
	AuthCookieSecure         bool
	TrustedEmailHeader       string
}

type AuthsRouter struct {
	Config AuthRuntimeConfig
	Users  *models.UsersTable
	Auths  *models.AuthsTable
	Now    func() time.Time
}

type signupForm struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ProfileImageURL string `json:"profile_image_url"`
}

type signinForm struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type addUserForm struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ProfileImageURL string `json:"profile_image_url"`
	Role            string `json:"role"`
}

type updateProfileForm struct {
	ProfileImageURL string  `json:"profile_image_url"`
	Name            string  `json:"name"`
	Bio             *string `json:"bio"`
	Gender          *string `json:"gender"`
	DateOfBirth     *string `json:"date_of_birth"`
}

type updateTimezoneForm struct {
	Timezone string `json:"timezone"`
}

type updatePasswordForm struct {
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

type sessionUserResponse struct {
	Token           string `json:"token"`
	TokenType       string `json:"token_type"`
	ExpiresAt       *int64 `json:"expires_at"`
	ID              string `json:"id"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	ProfileImageURL string `json:"profile_image_url"`
}

type apiKeyResponse struct {
	APIKey string `json:"api_key,omitempty"`
}

type adminDetailsResponse struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type adminConfigResponse struct {
	ShowAdminDetails bool   `json:"SHOW_ADMIN_DETAILS"`
	AdminEmail       string `json:"ADMIN_EMAIL,omitempty"`
	EnableSignup     bool   `json:"ENABLE_SIGNUP"`
	EnableAPIKeys    bool   `json:"ENABLE_API_KEYS"`
	DefaultUserRole  string `json:"DEFAULT_USER_ROLE"`
	JWTExpiresIn     string `json:"JWT_EXPIRES_IN"`
}

func (h *AuthsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auths/", h.GetSessionUser)
	mux.HandleFunc("GET /api/v1/auths/signout", h.Signout)
	mux.HandleFunc("POST /api/v1/auths/signup", h.Signup)
	mux.HandleFunc("POST /api/v1/auths/signin", h.Signin)
	mux.HandleFunc("POST /api/v1/auths/add", h.AddUser)
	mux.HandleFunc("GET /api/v1/auths/admin/details", h.GetAdminDetails)
	mux.HandleFunc("GET /api/v1/auths/admin/config", h.GetAdminConfig)
	mux.HandleFunc("GET /api/v1/auths/api_key", h.GetAPIKey)
	mux.HandleFunc("POST /api/v1/auths/api_key", h.GenerateAPIKey)
	mux.HandleFunc("DELETE /api/v1/auths/api_key", h.DeleteAPIKey)
	mux.HandleFunc("POST /api/v1/auths/update/profile", h.UpdateProfile)
	mux.HandleFunc("POST /api/v1/auths/update/timezone", h.UpdateTimezone)
	mux.HandleFunc("POST /api/v1/auths/update/password", h.UpdatePassword)
}

func (h *AuthsRouter) GetSessionUser(w http.ResponseWriter, r *http.Request) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return
	}

	user, isAPIKey, expiresAt, err := h.resolveUserByCredential(r, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return
	}

	if isAPIKey {
		writeJSON(w, http.StatusOK, sessionUserResponse{
			Token:           token,
			TokenType:       "Bearer",
			ExpiresAt:       nil,
			ID:              user.ID,
			Email:           user.Email,
			Name:            user.Name,
			Role:            user.Role,
			ProfileImageURL: user.ProfileImageURL,
		})
		return
	}
	h.writeSessionResponse(w, user, token, expiresAt)
}

func (h *AuthsRouter) Signout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: parseSameSite(h.Config.AuthCookieSameSite),
		Secure:   h.Config.AuthCookieSecure,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *AuthsRouter) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	var form signupForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	form.Email = strings.ToLower(strings.TrimSpace(form.Email))
	if !utils.ValidateEmailFormat(form.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid email format"})
		return
	}

	hasUsers, err := h.Users.HasUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if h.Config.WebUIAuth && hasUsers && !h.Config.EnableSignup && !h.Config.EnableInitialAdminSignup {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "access prohibited"})
		return
	}

	existing, err := h.Users.GetUserByEmail(r.Context(), form.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "email already taken"})
		return
	}

	hashed, err := utils.GetPasswordHash(form.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	user, err := h.Auths.InsertNewAuth(r.Context(), models.AuthInsertParams{
		Email:           form.Email,
		Password:        hashed,
		Name:            form.Name,
		ProfileImageURL: form.ProfileImageURL,
		Role:            h.Config.DefaultUserRole,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	count, err := h.Users.CountUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if count == 1 {
		adminRole := "admin"
		user, err = h.Users.UpdateUserByID(r.Context(), user.ID, models.UserUpdateParams{Role: &adminRole})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	h.writeSessionResponse(w, user, "", nil)
}

func (h *AuthsRouter) Signin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if !h.Config.EnablePasswordAuth {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "password auth disabled"})
		return
	}

	var form signinForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	user, err := h.Auths.AuthenticateUser(r.Context(), strings.ToLower(strings.TrimSpace(form.Email)), func(password string) bool {
		return utils.VerifyPassword(form.Password, password)
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid credentials"})
		return
	}

	h.writeSessionResponse(w, user, "", nil)
}

func (h *AuthsRouter) AddUser(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "access prohibited"})
		return
	}

	var form addUserForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	form.Email = strings.ToLower(strings.TrimSpace(form.Email))
	if !utils.ValidateEmailFormat(form.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid email format"})
		return
	}
	if !utils.ValidateProfileImageURL(form.ProfileImageURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image url"})
		return
	}

	existing, err := h.Users.GetUserByEmail(r.Context(), form.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "email already taken"})
		return
	}

	hashed, err := utils.GetPasswordHash(form.Password)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	user, err := h.Auths.InsertNewAuth(r.Context(), models.AuthInsertParams{
		Email:           form.Email,
		Password:        hashed,
		Name:            form.Name,
		ProfileImageURL: form.ProfileImageURL,
		Role:            form.Role,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	token, _, err := utils.CreateToken(h.Config.WebUISecretKey, user.ID, h.Config.JWTExpiresIn, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, sessionUserResponse{
		Token:           token,
		TokenType:       "Bearer",
		ID:              user.ID,
		Email:           user.Email,
		Name:            user.Name,
		Role:            user.Role,
		ProfileImageURL: user.ProfileImageURL,
	})
}

func (h *AuthsRouter) GetAdminDetails(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}
	if !h.Config.ShowAdminDetails {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "action prohibited"})
		return
	}

	adminEmail := h.Config.AdminEmail
	adminName := ""
	var admin *models.User
	var err error
	if strings.TrimSpace(adminEmail) != "" {
		admin, err = h.Users.GetUserByEmail(r.Context(), adminEmail)
	} else {
		admin, err = h.Users.GetFirstUser(r.Context())
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if admin != nil {
		adminEmail = admin.Email
		adminName = admin.Name
	}

	writeJSON(w, http.StatusOK, adminDetailsResponse{
		Name:  adminName,
		Email: adminEmail,
	})
}

func (h *AuthsRouter) GetAdminConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "access prohibited"})
		return
	}
	writeJSON(w, http.StatusOK, adminConfigResponse{
		ShowAdminDetails: h.Config.ShowAdminDetails,
		AdminEmail:       h.Config.AdminEmail,
		EnableSignup:     h.Config.EnableSignup,
		EnableAPIKeys:    h.Config.EnableAPIKeys,
		DefaultUserRole:  h.Config.DefaultUserRole,
		JWTExpiresIn:     h.Config.JWTExpiresIn,
	})
}

func (h *AuthsRouter) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var form updateProfileForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	if !utils.ValidateProfileImageURL(form.ProfileImageURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image url"})
		return
	}

	updated, err := h.Users.UpdateUserByID(r.Context(), sessionUser.ID, models.UserUpdateParams{
		Name:            &form.Name,
		ProfileImageURL: &form.ProfileImageURL,
		Bio:             form.Bio,
		Gender:          form.Gender,
		DateOfBirth:     form.DateOfBirth,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid credentials"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                updated.ID,
		"name":              updated.Name,
		"email":             updated.Email,
		"role":              updated.Role,
		"profile_image_url": updated.ProfileImageURL,
	})
}

func (h *AuthsRouter) UpdateTimezone(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var form updateTimezoneForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	_, err := h.Users.UpdateUserByID(r.Context(), sessionUser.ID, models.UserUpdateParams{
		Timezone: &form.Timezone,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"status": true})
}

func (h *AuthsRouter) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.Config.TrustedEmailHeader) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "action prohibited"})
		return
	}

	sessionUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var form updatePasswordForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	user, err := h.Auths.AuthenticateUser(r.Context(), sessionUser.Email, func(password string) bool {
		return utils.VerifyPassword(form.Password, password)
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "incorrect password"})
		return
	}

	hashed, err := utils.GetPasswordHash(form.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	updated, err := h.Auths.UpdateUserPasswordByID(r.Context(), user.ID, hashed)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *AuthsRouter) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if !h.Config.EnableAPIKeys {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "api key creation not allowed"})
		return
	}

	user, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	apiKey, err := utils.CreateAPIKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	success, err := h.Users.UpdateUserAPIKeyByID(r.Context(), user.ID, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if !success {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "create api key error"})
		return
	}
	writeJSON(w, http.StatusOK, apiKeyResponse{APIKey: apiKey})
}

func (h *AuthsRouter) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	deleted, err := h.Users.DeleteUserAPIKeyByID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *AuthsRouter) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	apiKey, err := h.Users.GetUserAPIKeyByID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if apiKey == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "api key not found"})
		return
	}
	writeJSON(w, http.StatusOK, apiKeyResponse{APIKey: apiKey})
}

func (h *AuthsRouter) writeSessionResponse(w http.ResponseWriter, user *models.User, token string, expiresAt *time.Time) {
	if token == "" {
		now := time.Now
		if h.Now != nil {
			now = h.Now
		}

		var err error
		token, expiresAt, err = utils.CreateToken(h.Config.WebUISecretKey, user.ID, h.Config.JWTExpiresIn, now())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	var expiresUnix *int64
	cookie := &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: parseSameSite(h.Config.AuthCookieSameSite),
		Secure:   h.Config.AuthCookieSecure,
	}
	if expiresAt != nil {
		value := expiresAt.Unix()
		expiresUnix = &value
		cookie.Expires = *expiresAt
	}
	http.SetCookie(w, cookie)

	writeJSON(w, http.StatusOK, sessionUserResponse{
		Token:           token,
		TokenType:       "Bearer",
		ExpiresAt:       expiresUnix,
		ID:              user.ID,
		Email:           user.Email,
		Name:            user.Name,
		Role:            user.Role,
		ProfileImageURL: user.ProfileImageURL,
	})
}

func (h *AuthsRouter) requireCurrentUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}

	user, _, _, err := h.resolveUserByCredential(r, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	return user, true
}

func (h *AuthsRouter) resolveUserByCredential(r *http.Request, token string) (*models.User, bool, *time.Time, error) {
	if strings.HasPrefix(token, "sk-") {
		if !h.Config.EnableAPIKeys {
			return nil, true, nil, nil
		}
		user, err := h.Users.GetUserByAPIKey(r.Context(), token)
		return user, true, nil, err
	}

	claims, err := utils.DecodeToken(h.Config.WebUISecretKey, token)
	if err != nil {
		return nil, false, nil, err
	}

	user, err := h.Users.GetUserByID(r.Context(), claims.ID)
	if err != nil {
		return nil, false, nil, err
	}

	var expiresAt *time.Time
	if claims.ExpiresAt != nil {
		value := claims.ExpiresAt.Time
		expiresAt = &value
	}
	return user, false, expiresAt, nil
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		fallthrough
	default:
		return http.SameSiteLaxMode
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
