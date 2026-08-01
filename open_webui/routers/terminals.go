package routers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
	accesscontrol "github.com/xxnuo/open-coreui/backend/open_webui/utils/access_control"
)

type TerminalsRuntimeConfig struct {
	WebUISecretKey  string
	EnableAPIKeys   bool
	State           *ConfigsState
	HTTPClient      *http.Client
	WebSocketDialer *websocket.Dialer
}

type TerminalsRouter struct {
	Config TerminalsRuntimeConfig
	Users  *models.UsersTable
	Groups *models.GroupsTable
}

type terminalServerResponse struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

var (
	errTerminalInvalidToken  = errors.New("invalid token")
	errTerminalAPIKeyBlocked = errors.New("api key not allowed")
	errTerminalNotConfigured = errors.New("terminal server url not configured")
)

func (h *TerminalsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/terminals/", h.ListTerminalServers)
	mux.HandleFunc("GET /api/v1/terminals/{server_id}/api/terminals/{session_id}", h.ProxyTerminalWebSocket)
	for _, method := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	} {
		mux.HandleFunc(method+" /api/v1/terminals/{server_id}", h.ProxyTerminalRoot)
		mux.HandleFunc(method+" /api/v1/terminals/{server_id}/{path...}", h.ProxyTerminal)
	}
}

func (h *TerminalsRouter) ProxyTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	clientConn, err := websocketUpgrader().Upgrade(w, r, nil)
	if err != nil {
		return
	}

	user, connection, err := h.resolveAuthenticatedWebSocket(r, clientConn)
	if err != nil {
		_ = clientConn.Close()
		return
	}

	upstreamURL, err := buildTerminalWebSocketURL(connection, r.PathValue("session_id"), user.ID)
	if err != nil {
		_ = closeWebSocket(clientConn, 4003, "Terminal server URL not configured")
		_ = clientConn.Close()
		return
	}

	headers := http.Header{}
	authType := strings.TrimSpace(stringValue(connection["auth_type"]))
	if authType == "" {
		authType = "bearer"
	}
	switch authType {
	case "session":
		if token := utils.ExtractTokenFromRequest(r); token != "" {
			headers.Set("Authorization", "Bearer "+token)
		}
		if value := r.Header.Get("Cookie"); value != "" {
			headers.Set("Cookie", value)
		}
	case "system_oauth":
		if token := r.Header.Get("X-OAuth-Access-Token"); token != "" {
			headers.Set("Authorization", "Bearer "+token)
		}
		if value := r.Header.Get("Cookie"); value != "" {
			headers.Set("Cookie", value)
		}
	}

	dialer := websocket.DefaultDialer
	if h.Config.WebSocketDialer != nil {
		dialer = h.Config.WebSocketDialer
	}
	upstreamConn, _, err := dialer.Dial(upstreamURL, headers)
	if err != nil {
		_ = closeWebSocket(clientConn, 1011, "Terminal proxy error")
		_ = clientConn.Close()
		return
	}

	if authType == "bearer" {
		if err := upstreamConn.WriteJSON(map[string]string{
			"type":  "auth",
			"token": stringValue(connection["key"]),
		}); err != nil {
			_ = upstreamConn.Close()
			_ = closeWebSocket(clientConn, 1011, "Terminal proxy error")
			_ = clientConn.Close()
			return
		}
	}

	done := make(chan struct{}, 2)
	go proxyWebSocketMessages(clientConn, upstreamConn, done)
	go proxyWebSocketMessages(upstreamConn, clientConn, done)
	<-done
	_ = upstreamConn.Close()
	_ = clientConn.Close()
}

func (h *TerminalsRouter) ListTerminalServers(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	userGroupIDs, err := h.userGroupIDs(r, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	items := []terminalServerResponse{}
	for _, connection := range h.connections() {
		if !connectionEnabled(connection) {
			continue
		}
		if !accesscontrol.HasConnectionAccess(user, connection, userGroupIDs) {
			continue
		}
		items = append(items, terminalServerResponse{
			ID:   stringValue(connection["id"]),
			URL:  stringValue(connection["url"]),
			Name: stringValue(connection["name"]),
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *TerminalsRouter) ProxyTerminalRoot(w http.ResponseWriter, r *http.Request) {
	h.proxyTerminal(w, r, "")
}

func (h *TerminalsRouter) ProxyTerminal(w http.ResponseWriter, r *http.Request) {
	h.proxyTerminal(w, r, r.PathValue("path"))
}

func (h *TerminalsRouter) proxyTerminal(w http.ResponseWriter, r *http.Request, rawPath string) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	connection, found := h.connectionByID(r.PathValue("server_id"))
	if !found || !connectionEnabled(connection) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Terminal server not found"})
		return
	}
	userGroupIDs, err := h.userGroupIDs(r, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if !accesscontrol.HasConnectionAccess(user, connection, userGroupIDs) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Access denied"})
		return
	}

	baseURL := strings.TrimRight(stringValue(connection["url"]), "/")
	if baseURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Terminal server URL not configured"})
		return
	}
	safePath, valid := sanitizeTerminalProxyPath(rawPath)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid path"})
		return
	}

	targetURL := baseURL
	if policyID := strings.TrimSpace(stringValue(connection["policy_id"])); policyID != "" {
		targetURL += "/p/" + url.PathEscape(policyID)
	}
	if safePath != "" {
		targetURL += "/" + safePath
	}
	if rawQuery := r.URL.RawQuery; rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	var requestBody io.Reader
	if len(body) > 0 {
		requestBody = bytes.NewReader(body)
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, requestBody)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	h.fillProxyHeaders(upstreamReq, r, user, connection)

	client := h.Config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Terminal proxy error: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	copyTerminalResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (h *TerminalsRouter) fillProxyHeaders(upstreamReq *http.Request, r *http.Request, user *models.User, connection map[string]any) {
	upstreamReq.Header.Set("X-User-Id", user.ID)
	if value := r.Header.Get("Accept"); value != "" {
		upstreamReq.Header.Set("Accept", value)
	}
	if value := r.Header.Get("Content-Type"); value != "" {
		upstreamReq.Header.Set("Content-Type", value)
	}
	if value := r.Header.Get("Range"); value != "" {
		upstreamReq.Header.Set("Range", value)
	}

	authType := strings.TrimSpace(stringValue(connection["auth_type"]))
	if authType == "" {
		authType = "bearer"
	}
	switch authType {
	case "session":
		if token := utils.ExtractTokenFromRequest(r); token != "" {
			upstreamReq.Header.Set("Authorization", "Bearer "+token)
		}
		if value := r.Header.Get("Cookie"); value != "" {
			upstreamReq.Header.Set("Cookie", value)
		}
	case "system_oauth":
		if token := r.Header.Get("X-OAuth-Access-Token"); token != "" {
			upstreamReq.Header.Set("Authorization", "Bearer "+token)
		}
		if value := r.Header.Get("Cookie"); value != "" {
			upstreamReq.Header.Set("Cookie", value)
		}
	default:
		if key := stringValue(connection["key"]); key != "" {
			upstreamReq.Header.Set("Authorization", "Bearer "+key)
		}
	}
}

func (h *TerminalsRouter) userGroupIDs(r *http.Request, userID string) (map[string]struct{}, error) {
	if h.Groups == nil {
		return map[string]struct{}{}, nil
	}
	groups, err := h.Groups.GetGroupsByMemberID(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	ids := map[string]struct{}{}
	for _, group := range groups {
		ids[group.ID] = struct{}{}
	}
	return ids, nil
}

func (h *TerminalsRouter) connectionByID(id string) (map[string]any, bool) {
	for _, connection := range h.connections() {
		if stringValue(connection["id"]) == id {
			return connection, true
		}
	}
	return nil, false
}

func (h *TerminalsRouter) connections() []map[string]any {
	if h.Config.State == nil {
		return []map[string]any{}
	}
	return cloneConfigList(h.Config.State.TerminalServerConnections)
}

func (h *TerminalsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	user, err := h.resolveUserByToken(r, token)
	if err != nil {
		if errors.Is(err, errTerminalAPIKeyBlocked) {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": err.Error()})
			return nil, false
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	if user != nil {
		return user, true
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
	return nil, false
}

func (h *TerminalsRouter) resolveUserByToken(r *http.Request, token string) (*models.User, error) {
	if strings.HasPrefix(token, "sk-") {
		if !h.Config.EnableAPIKeys {
			return nil, errTerminalAPIKeyBlocked
		}
		return h.Users.GetUserByAPIKey(r.Context(), token)
	}
	claims, err := utils.DecodeToken(h.Config.WebUISecretKey, token)
	if err != nil {
		return nil, errTerminalInvalidToken
	}
	return h.Users.GetUserByID(r.Context(), claims.ID)
}

func (h *TerminalsRouter) resolveAuthenticatedWebSocket(r *http.Request, clientConn *websocket.Conn) (*models.User, map[string]any, error) {
	_, message, err := clientConn.ReadMessage()
	if err != nil {
		_ = closeWebSocket(clientConn, 4001, "Invalid token")
		return nil, nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(message, &payload); err != nil {
		_ = closeWebSocket(clientConn, 4001, "Invalid token")
		return nil, nil, err
	}
	if stringValue(payload["type"]) != "auth" {
		_ = closeWebSocket(clientConn, 4001, "Expected auth message")
		return nil, nil, errTerminalInvalidToken
	}

	user, err := h.resolveUserByToken(r, stringValue(payload["token"]))
	if err != nil || user == nil {
		_ = closeWebSocket(clientConn, 4001, "Invalid token")
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errTerminalInvalidToken
	}

	connection, found := h.connectionByID(r.PathValue("server_id"))
	if !found || !connectionEnabled(connection) {
		_ = closeWebSocket(clientConn, 4004, "Terminal server not found")
		return nil, nil, errTerminalInvalidToken
	}

	userGroupIDs, err := h.userGroupIDs(r, user.ID)
	if err != nil {
		_ = closeWebSocket(clientConn, 4003, "Access denied")
		return nil, nil, err
	}
	if !accesscontrol.HasConnectionAccess(user, connection, userGroupIDs) {
		_ = closeWebSocket(clientConn, 4003, "Access denied")
		return nil, nil, errTerminalInvalidToken
	}

	return user, connection, nil
}

func sanitizeTerminalProxyPath(rawPath string) (string, bool) {
	if rawPath == "" {
		return "", true
	}
	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", false
	}
	hadTrailingSlash := strings.HasSuffix(decoded, "/")
	normalized := path.Clean(decoded)
	cleaned := strings.TrimLeft(normalized, "/")
	if strings.HasPrefix(cleaned, "..") || cleaned == "." {
		return "", false
	}
	if hadTrailingSlash && cleaned != "" && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned, true
}

func copyTerminalResponseHeaders(target http.Header, source http.Header) {
	for key, values := range source {
		lowerKey := strings.ToLower(key)
		if lowerKey == "transfer-encoding" || lowerKey == "connection" || lowerKey == "content-encoding" || lowerKey == "content-length" {
			continue
		}
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func connectionEnabled(connection map[string]any) bool {
	value, ok := connection["enabled"]
	if !ok {
		return true
	}
	enabled, ok := value.(bool)
	if !ok {
		return true
	}
	return enabled
}

func stringValue(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}

func websocketUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

func buildTerminalWebSocketURL(connection map[string]any, sessionID string, userID string) (string, error) {
	baseURL := strings.TrimRight(stringValue(connection["url"]), "/")
	if baseURL == "" {
		return "", errTerminalNotConfigured
	}
	switch {
	case strings.HasPrefix(baseURL, "https://"):
		baseURL = "wss://" + strings.TrimPrefix(baseURL, "https://")
	case strings.HasPrefix(baseURL, "http://"):
		baseURL = "ws://" + strings.TrimPrefix(baseURL, "http://")
	case strings.HasPrefix(baseURL, "wss://"), strings.HasPrefix(baseURL, "ws://"):
	default:
		return "", errTerminalNotConfigured
	}

	targetURL := baseURL
	if policyID := strings.TrimSpace(stringValue(connection["policy_id"])); policyID != "" {
		targetURL += "/p/" + url.PathEscape(policyID)
	}
	targetURL += "/api/terminals/" + url.PathEscape(sessionID)
	params := url.Values{}
	params.Set("user_id", userID)
	targetURL += "?" + params.Encode()
	return targetURL, nil
}

func proxyWebSocketMessages(source *websocket.Conn, target *websocket.Conn, done chan<- struct{}) {
	defer func() {
		done <- struct{}{}
	}()
	for {
		messageType, message, err := source.ReadMessage()
		if err != nil {
			return
		}
		switch messageType {
		case websocket.TextMessage, websocket.BinaryMessage, websocket.PingMessage, websocket.PongMessage:
			if err := target.WriteMessage(messageType, message); err != nil {
				return
			}
		case websocket.CloseMessage:
			_ = target.WriteMessage(websocket.CloseMessage, message)
			return
		}
	}
}

func closeWebSocket(conn *websocket.Conn, code int, text string) error {
	return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(time.Second))
}
