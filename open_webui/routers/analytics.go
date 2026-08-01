package routers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type AnalyticsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type AnalyticsRouter struct {
	Config       AnalyticsRuntimeConfig
	Users        *models.UsersTable
	ChatMessages *models.ChatMessagesTable
	Chats        *models.ChatsTable
	Feedbacks    *models.FeedbacksTable
	Now          func() time.Time
}

type modelAnalyticsEntry struct {
	ModelID string `json:"model_id"`
	Count   int    `json:"count"`
}

type modelAnalyticsResponse struct {
	Models []modelAnalyticsEntry `json:"models"`
}

type userAnalyticsEntry struct {
	UserID       string `json:"user_id"`
	Name         string `json:"name,omitempty"`
	Email        string `json:"email,omitempty"`
	Count        int    `json:"count"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
}

type userAnalyticsResponse struct {
	Users []userAnalyticsEntry `json:"users"`
}

type summaryResponse struct {
	TotalMessages int `json:"total_messages"`
	TotalChats    int `json:"total_chats"`
	TotalModels   int `json:"total_models"`
	TotalUsers    int `json:"total_users"`
}

type dailyStatsEntry struct {
	Date   string         `json:"date"`
	Models map[string]int `json:"models"`
}

type dailyStatsResponse struct {
	Data []dailyStatsEntry `json:"data"`
}

type tokenUsageEntry struct {
	ModelID      string `json:"model_id"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	MessageCount int    `json:"message_count"`
}

type tokenUsageResponse struct {
	Models            []tokenUsageEntry `json:"models"`
	TotalInputTokens  int               `json:"total_input_tokens"`
	TotalOutputTokens int               `json:"total_output_tokens"`
	TotalTokens       int               `json:"total_tokens"`
}

type modelChatEntry struct {
	ChatID       string `json:"chat_id"`
	UserID       string `json:"user_id,omitempty"`
	UserName     string `json:"user_name,omitempty"`
	FirstMessage string `json:"first_message,omitempty"`
	UpdatedAt    int64  `json:"updated_at"`
}

type modelChatsResponse struct {
	Chats []modelChatEntry `json:"chats"`
	Total int              `json:"total"`
}

type historyEntry struct {
	Date string `json:"date"`
	Won  int    `json:"won"`
	Lost int    `json:"lost"`
}

type tagEntry struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type modelOverviewResponse struct {
	History []historyEntry `json:"history"`
	Tags    []tagEntry     `json:"tags"`
}

func (h *AnalyticsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/analytics/models", h.GetModelAnalytics)
	mux.HandleFunc("GET /api/v1/analytics/users", h.GetUserAnalytics)
	mux.HandleFunc("GET /api/v1/analytics/messages", h.GetMessages)
	mux.HandleFunc("GET /api/v1/analytics/summary", h.GetSummary)
	mux.HandleFunc("GET /api/v1/analytics/daily", h.GetDailyStats)
	mux.HandleFunc("GET /api/v1/analytics/tokens", h.GetTokenUsage)
	mux.HandleFunc("GET /api/v1/analytics/models/{model_id}/chats", h.GetModelChats)
	mux.HandleFunc("GET /api/v1/analytics/models/{model_id}/overview", h.GetModelOverview)
}

func (h *AnalyticsRouter) GetModelAnalytics(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	counts, err := h.ChatMessages.GetMessageCountByModel(r.Context(), startDate, endDate, strings.TrimSpace(r.URL.Query().Get("group_id")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	items := make([]modelAnalyticsEntry, 0, len(counts))
	for modelID, count := range counts {
		items = append(items, modelAnalyticsEntry{ModelID: modelID, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].ModelID < items[j].ModelID
		}
		return items[i].Count > items[j].Count
	})
	writeJSON(w, http.StatusOK, modelAnalyticsResponse{Models: items})
}

func (h *AnalyticsRouter) GetUserAnalytics(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	limit := parseIntQuery(r, "limit", 50)
	if limit < 1 {
		limit = 50
	}

	counts, err := h.ChatMessages.GetMessageCountByUser(r.Context(), startDate, endDate, groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	usage, err := h.ChatMessages.GetTokenUsageByUser(r.Context(), startDate, endDate, groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	userIDs := make([]string, 0, len(counts))
	for userID := range counts {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool {
		if counts[userIDs[i]] == counts[userIDs[j]] {
			return userIDs[i] < userIDs[j]
		}
		return counts[userIDs[i]] > counts[userIDs[j]]
	})
	if len(userIDs) > limit {
		userIDs = userIDs[:limit]
	}

	items := make([]userAnalyticsEntry, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := h.Users.GetUserByID(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		entry := userAnalyticsEntry{
			UserID: userID,
			Count:  counts[userID],
		}
		if user != nil {
			entry.Name = user.Name
			entry.Email = user.Email
		}
		if item := usage[userID]; item != nil {
			entry.InputTokens = item["input_tokens"]
			entry.OutputTokens = item["output_tokens"]
			entry.TotalTokens = item["total_tokens"]
		}
		items = append(items, entry)
	}
	writeJSON(w, http.StatusOK, userAnalyticsResponse{Users: items})
}

func (h *AnalyticsRouter) GetMessages(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	modelID := strings.TrimSpace(r.URL.Query().Get("model_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	chatID := strings.TrimSpace(r.URL.Query().Get("chat_id"))
	startDate, endDate := parseAnalyticsDateRange(r)
	skip := parseIntQuery(r, "skip", 0)
	limit := parseIntQuery(r, "limit", 50)
	if limit < 1 {
		limit = 50
	}

	if chatID != "" {
		items, err := h.ChatMessages.GetMessagesByChatID(r.Context(), chatID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if modelID != "" {
		items, err := h.ChatMessages.GetMessagesByModelID(r.Context(), modelID, startDate, endDate, skip, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if userID != "" {
		items, err := h.ChatMessages.GetMessagesByUserID(r.Context(), userID, skip, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	writeJSON(w, http.StatusOK, []models.ChatMessage{})
}

func (h *AnalyticsRouter) GetSummary(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	modelCounts, err := h.ChatMessages.GetMessageCountByModel(r.Context(), startDate, endDate, groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	userCounts, err := h.ChatMessages.GetMessageCountByUser(r.Context(), startDate, endDate, groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	chatCounts, err := h.ChatMessages.GetMessageCountByChat(r.Context(), startDate, endDate, groupID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	totalMessages := 0
	for _, count := range modelCounts {
		totalMessages += count
	}
	writeJSON(w, http.StatusOK, summaryResponse{
		TotalMessages: totalMessages,
		TotalChats:    len(chatCounts),
		TotalModels:   len(modelCounts),
		TotalUsers:    len(userCounts),
	})
}

func (h *AnalyticsRouter) GetDailyStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	granularity := strings.TrimSpace(r.URL.Query().Get("granularity"))

	var (
		counts map[string]map[string]int
		err    error
	)
	if granularity == "hourly" {
		counts, err = h.ChatMessages.GetHourlyMessageCountsByModel(r.Context(), startDate, endDate)
	} else {
		counts, err = h.ChatMessages.GetDailyMessageCountsByModel(r.Context(), startDate, endDate, groupID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]dailyStatsEntry, 0, len(keys))
	for _, key := range keys {
		items = append(items, dailyStatsEntry{Date: key, Models: counts[key]})
	}
	writeJSON(w, http.StatusOK, dailyStatsResponse{Data: items})
}

func (h *AnalyticsRouter) GetTokenUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	usage, err := h.ChatMessages.GetTokenUsageByModel(r.Context(), startDate, endDate, strings.TrimSpace(r.URL.Query().Get("group_id")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	items := make([]tokenUsageEntry, 0, len(usage))
	totalInput := 0
	totalOutput := 0
	for modelID, item := range usage {
		entry := tokenUsageEntry{
			ModelID:      modelID,
			InputTokens:  item["input_tokens"],
			OutputTokens: item["output_tokens"],
			TotalTokens:  item["total_tokens"],
			MessageCount: item["message_count"],
		}
		totalInput += entry.InputTokens
		totalOutput += entry.OutputTokens
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalTokens == items[j].TotalTokens {
			return items[i].ModelID < items[j].ModelID
		}
		return items[i].TotalTokens > items[j].TotalTokens
	})
	writeJSON(w, http.StatusOK, tokenUsageResponse{
		Models:            items,
		TotalInputTokens:  totalInput,
		TotalOutputTokens: totalOutput,
		TotalTokens:       totalInput + totalOutput,
	})
}

func (h *AnalyticsRouter) GetModelChats(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	startDate, endDate := parseAnalyticsDateRange(r)
	chatIDs, err := h.ChatMessages.GetChatIDsByModelID(
		r.Context(),
		r.PathValue("model_id"),
		startDate,
		endDate,
		parseIntToMinimum(r, "skip", 0, 0),
		parseIntToMinimum(r, "limit", 50, 1),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	items := make([]modelChatEntry, 0, len(chatIDs))
	for _, chatID := range chatIDs {
		messages, err := h.ChatMessages.GetMessagesByChatID(r.Context(), chatID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		if len(messages) == 0 {
			continue
		}
		var firstUserMessage *models.ChatMessage
		updatedAt := int64(0)
		for _, message := range messages {
			if message.Role == "user" && firstUserMessage == nil {
				current := message
				firstUserMessage = &current
			}
			if message.CreatedAt > updatedAt {
				updatedAt = message.CreatedAt
			}
		}
		entry := modelChatEntry{
			ChatID:    chatID,
			UpdatedAt: updatedAt,
		}
		if firstUserMessage != nil {
			entry.UserID = firstUserMessage.UserID
			entry.FirstMessage = analyticsContentPreview(firstUserMessage.Content)
			user, err := h.Users.GetUserByID(r.Context(), firstUserMessage.UserID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
				return
			}
			if user != nil {
				entry.UserName = user.Name
			}
		}
		items = append(items, entry)
	}
	writeJSON(w, http.StatusOK, modelChatsResponse{Chats: items, Total: len(items)})
}

func (h *AnalyticsRouter) GetModelOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	days := parseIntQuery(r, "days", 30)
	chatIDs, err := h.ChatMessages.GetChatIDsByModelID(r.Context(), r.PathValue("model_id"), nil, nil, 0, 10000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	var startTime *time.Time
	if days > 0 {
		value := now.AddDate(0, 0, -days)
		startTime = &value
	}

	historyCounts := map[string]historyEntry{}
	tagCounts := map[string]int{}
	for _, chatID := range chatIDs {
		feedbacks, err := h.Feedbacks.GetFeedbacksByChatID(r.Context(), chatID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		for _, feedback := range feedbacks {
			rating := analyticsRatingValue(feedback.Data["rating"])
			feedbackTime := time.Unix(feedback.CreatedAt, 0).UTC()
			if startTime != nil && feedbackTime.Before(*startTime) {
				continue
			}
			key := feedbackTime.Format("2006-01-02")
			entry := historyCounts[key]
			entry.Date = key
			if rating == 1 {
				entry.Won++
			} else if rating == -1 {
				entry.Lost++
			}
			historyCounts[key] = entry
		}

		chat, err := h.Chats.GetChatByID(r.Context(), chatID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		if chat == nil {
			continue
		}
		for _, tag := range analyticsTags(chat.Meta["tags"]) {
			tagCounts[tag]++
		}
	}

	history := make([]historyEntry, 0)
	if len(historyCounts) > 0 || days > 0 {
		current := now
		if startTime != nil {
			current = *startTime
		} else {
			minKey := ""
			for key := range historyCounts {
				if minKey == "" || key < minKey {
					minKey = key
				}
			}
			if minKey != "" {
				parsed, err := time.Parse("2006-01-02", minKey)
				if err == nil {
					current = parsed.UTC()
				}
			}
		}
		for !current.After(now) {
			key := current.Format("2006-01-02")
			entry := historyCounts[key]
			entry.Date = key
			history = append(history, entry)
			current = current.AddDate(0, 0, 1)
		}
	}

	tags := make([]tagEntry, 0, len(tagCounts))
	for tag, count := range tagCounts {
		tags = append(tags, tagEntry{Tag: tag, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count == tags[j].Count {
			return tags[i].Tag < tags[j].Tag
		}
		return tags[i].Count > tags[j].Count
	})
	if len(tags) > 10 {
		tags = tags[:10]
	}
	writeJSON(w, http.StatusOK, modelOverviewResponse{History: history, Tags: tags})
}

func (h *AnalyticsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *AnalyticsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func parseAnalyticsDateRange(r *http.Request) (*int64, *int64) {
	startDate := parseOptionalInt64Query(r, "start_date")
	endDate := parseOptionalInt64Query(r, "end_date")
	return startDate, endDate
}

func parseOptionalInt64Query(r *http.Request, key string) *int64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil
	}
	var parsed int64
	if _, err := fmtSscanfInt64(value, &parsed); err != nil {
		return nil
	}
	return &parsed
}

func parseIntToMinimum(r *http.Request, key string, fallback int, minimum int) int {
	value := parseIntQuery(r, key, fallback)
	if value < minimum {
		return minimum
	}
	return value
}

func analyticsContentPreview(content any) string {
	switch typed := content.(type) {
	case string:
		if len(typed) > 200 {
			return typed[:200]
		}
		return typed
	case []any:
		parts := make([]string, 0)
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := block["text"].(string)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		joined := strings.Join(parts, " ")
		if len(joined) > 200 {
			return joined[:200]
		}
		return joined
	default:
		return ""
	}
}

func analyticsTags(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		tags := make([]string, 0, len(typed))
		for _, item := range typed {
			tag, _ := item.(string)
			if strings.TrimSpace(tag) != "" {
				tags = append(tags, tag)
			}
		}
		return tags
	default:
		return []string{}
	}
}

func analyticsRatingValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func fmtSscanfInt64(value string, target *int64) (int, error) {
	var parsed int64
	n, err := fmt.Sscanf(value, "%d", &parsed)
	if err != nil {
		return n, err
	}
	*target = parsed
	return n, nil
}
