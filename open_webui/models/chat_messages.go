package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type ChatMessage struct {
	ID        string         `json:"id"`
	ChatID    string         `json:"chat_id"`
	UserID    string         `json:"user_id"`
	Role      string         `json:"role"`
	ParentID  string         `json:"parent_id,omitempty"`
	Content   any            `json:"content,omitempty"`
	ModelID   string         `json:"model_id,omitempty"`
	Usage     map[string]any `json:"usage,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

type ChatMessagesTable struct {
	db *dbinternal.Handle
}

func NewChatMessagesTable(db *dbinternal.Handle) *ChatMessagesTable {
	return &ChatMessagesTable{db: db}
}

func (t *ChatMessagesTable) GetMessagesByChatID(ctx context.Context, chatID string) ([]ChatMessage, error) {
	chat, err := t.getChatByID(ctx, chatID)
	if err != nil || chat == nil {
		return []ChatMessage{}, err
	}
	messages := parseChatMessages(*chat)
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].CreatedAt == messages[j].CreatedAt {
			return messages[i].ID < messages[j].ID
		}
		return messages[i].CreatedAt < messages[j].CreatedAt
	})
	return messages, nil
}

func (t *ChatMessagesTable) GetMessagesByUserID(ctx context.Context, userID string, skip int, limit int) ([]ChatMessage, error) {
	messages, err := t.loadMessages(ctx, "")
	if err != nil {
		return nil, err
	}
	filtered := make([]ChatMessage, 0)
	for _, message := range messages {
		if message.UserID == userID {
			filtered = append(filtered, message)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt == filtered[j].CreatedAt {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt > filtered[j].CreatedAt
	})
	return paginateMessages(filtered, skip, limit), nil
}

func (t *ChatMessagesTable) GetMessagesByModelID(ctx context.Context, modelID string, startDate *int64, endDate *int64, skip int, limit int) ([]ChatMessage, error) {
	messages, err := t.loadMessages(ctx, "")
	if err != nil {
		return nil, err
	}
	filtered := make([]ChatMessage, 0)
	for _, message := range messages {
		if message.ModelID != modelID {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		filtered = append(filtered, message)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt == filtered[j].CreatedAt {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].CreatedAt > filtered[j].CreatedAt
	})
	return paginateMessages(filtered, skip, limit), nil
}

func (t *ChatMessagesTable) GetChatIDsByModelID(ctx context.Context, modelID string, startDate *int64, endDate *int64, skip int, limit int) ([]string, error) {
	messages, err := t.loadMessages(ctx, "")
	if err != nil {
		return nil, err
	}
	lastMessageAtByChat := map[string]int64{}
	for _, message := range messages {
		if message.Role != "assistant" || message.ModelID != modelID {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		if current, ok := lastMessageAtByChat[message.ChatID]; !ok || message.CreatedAt > current {
			lastMessageAtByChat[message.ChatID] = message.CreatedAt
		}
	}
	type pair struct {
		ChatID    string
		CreatedAt int64
	}
	items := make([]pair, 0, len(lastMessageAtByChat))
	for chatID, createdAt := range lastMessageAtByChat {
		items = append(items, pair{ChatID: chatID, CreatedAt: createdAt})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ChatID < items[j].ChatID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	start, end := paginateBounds(len(items), skip, limit)
	result := make([]string, 0, end-start)
	for _, item := range items[start:end] {
		result = append(result, item.ChatID)
	}
	return result, nil
}

func (t *ChatMessagesTable) GetMessageCountByModel(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, message := range messages {
		if message.Role != "assistant" || message.ModelID == "" || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		counts[message.ModelID]++
	}
	return counts, nil
}

func (t *ChatMessagesTable) GetTokenUsageByModel(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	usage := map[string]map[string]int{}
	for _, message := range messages {
		if message.Role != "assistant" || message.ModelID == "" || message.Usage == nil || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		inputTokens := intValue(message.Usage["input_tokens"])
		outputTokens := intValue(message.Usage["output_tokens"])
		item := usage[message.ModelID]
		if item == nil {
			item = map[string]int{}
			usage[message.ModelID] = item
		}
		item["input_tokens"] += inputTokens
		item["output_tokens"] += outputTokens
		item["total_tokens"] += inputTokens + outputTokens
		item["message_count"]++
	}
	return usage, nil
}

func (t *ChatMessagesTable) GetTokenUsageByUser(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	usage := map[string]map[string]int{}
	for _, message := range messages {
		if message.Role != "assistant" || message.UserID == "" || message.Usage == nil || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		inputTokens := intValue(message.Usage["input_tokens"])
		outputTokens := intValue(message.Usage["output_tokens"])
		item := usage[message.UserID]
		if item == nil {
			item = map[string]int{}
			usage[message.UserID] = item
		}
		item["input_tokens"] += inputTokens
		item["output_tokens"] += outputTokens
		item["total_tokens"] += inputTokens + outputTokens
		item["message_count"]++
	}
	return usage, nil
}

func (t *ChatMessagesTable) GetMessageCountByUser(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, message := range messages {
		if message.UserID == "" || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		counts[message.UserID]++
	}
	return counts, nil
}

func (t *ChatMessagesTable) GetMessageCountByChat(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, message := range messages {
		if message.UserID == "" || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		counts[message.ChatID]++
	}
	return counts, nil
}

func (t *ChatMessagesTable) GetDailyMessageCountsByModel(ctx context.Context, startDate *int64, endDate *int64, groupID string) (map[string]map[string]int, error) {
	messages, err := t.loadMessages(ctx, groupID)
	if err != nil {
		return nil, err
	}
	counts := map[string]map[string]int{}
	for _, message := range messages {
		if message.Role != "assistant" || message.ModelID == "" || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		dateKey := normalizeTimestamp(message.CreatedAt).Format("2006-01-02")
		if counts[dateKey] == nil {
			counts[dateKey] = map[string]int{}
		}
		counts[dateKey][message.ModelID]++
	}
	fillMissingDays(counts, startDate, endDate)
	return counts, nil
}

func (t *ChatMessagesTable) GetHourlyMessageCountsByModel(ctx context.Context, startDate *int64, endDate *int64) (map[string]map[string]int, error) {
	messages, err := t.loadMessages(ctx, "")
	if err != nil {
		return nil, err
	}
	counts := map[string]map[string]int{}
	for _, message := range messages {
		if message.Role != "assistant" || message.ModelID == "" || strings.HasPrefix(message.UserID, "shared-") {
			continue
		}
		if !messageInRange(message.CreatedAt, startDate, endDate) {
			continue
		}
		dateKey := normalizeTimestamp(message.CreatedAt).Format("2006-01-02 15:00")
		if counts[dateKey] == nil {
			counts[dateKey] = map[string]int{}
		}
		counts[dateKey][message.ModelID]++
	}
	fillMissingHours(counts, startDate, endDate)
	return counts, nil
}

func (t *ChatMessagesTable) loadMessages(ctx context.Context, groupID string) ([]ChatMessage, error) {
	chats, err := t.loadChats(ctx, groupID)
	if err != nil {
		return nil, err
	}
	messages := make([]ChatMessage, 0)
	for _, chat := range chats {
		messages = append(messages, parseChatMessages(chat)...)
	}
	return messages, nil
}

func (t *ChatMessagesTable) loadChats(ctx context.Context, groupID string) ([]Chat, error) {
	query := `SELECT id, user_id, title, chat, created_at, updated_at, share_id, archived, folder_id FROM chat`
	args := []any{}
	if groupID != "" {
		query += ` WHERE user_id IN (SELECT user_id FROM group_member WHERE group_id = ?)`
		args = append(args, groupID)
	}
	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chats := make([]Chat, 0)
	for rows.Next() {
		chat, err := scanChatRows(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, *chat)
	}
	return chats, rows.Err()
}

func (t *ChatMessagesTable) getChatByID(ctx context.Context, chatID string) (*Chat, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, title, chat, created_at, updated_at, share_id, archived, folder_id FROM chat WHERE id = ? LIMIT 1`, t.db.Dialect),
		chatID,
	)
	return scanChatRow(row)
}

func parseChatMessages(chat Chat) []ChatMessage {
	history, _ := chat.Chat["history"].(map[string]any)
	messageMap, _ := history["messages"].(map[string]any)
	messages := make([]ChatMessage, 0, len(messageMap))
	for messageID, raw := range messageMap {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := stringValueFromAny(item["id"])
		if id == "" {
			id = messageID
		}
		usage := map[string]any{}
		if rawUsage := cloneArbitraryMap(item["usage"]); rawUsage != nil {
			usage = rawUsage
		} else if info := cloneArbitraryMap(item["info"]); info != nil {
			if infoUsage := cloneArbitraryMap(info["usage"]); infoUsage != nil {
				usage = infoUsage
			}
		}
		userID := stringValueFromAny(item["user_id"])
		if userID == "" {
			userID = chat.UserID
		}
		createdAt := int64Value(item["timestamp"])
		if createdAt == 0 {
			createdAt = chat.UpdatedAt
		}
		if createdAt == 0 {
			createdAt = chat.CreatedAt
		}
		messages = append(messages, ChatMessage{
			ID:        id,
			ChatID:    chat.ID,
			UserID:    userID,
			Role:      firstNonEmptyString(stringValueFromAny(item["role"]), "user"),
			ParentID:  firstNonEmptyString(stringValueFromAny(item["parent_id"]), stringValueFromAny(item["parentId"])),
			Content:   item["content"],
			ModelID:   firstNonEmptyString(stringValueFromAny(item["model_id"]), stringValueFromAny(item["model"])),
			Usage:     usage,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		})
	}
	return messages
}

func scanChatRows(rows *sql.Rows) (*Chat, error) {
	var chat Chat
	var titleRaw sql.NullString
	var chatRaw sql.NullString
	var shareIDRaw sql.NullString
	var archivedRaw sql.NullBool
	var folderIDRaw sql.NullString
	if err := rows.Scan(&chat.ID, &chat.UserID, &titleRaw, &chatRaw, &chat.CreatedAt, &chat.UpdatedAt, &shareIDRaw, &archivedRaw, &folderIDRaw); err != nil {
		return nil, err
	}
	if titleRaw.Valid {
		chat.Title = titleRaw.String
	}
	payload, err := unmarshalJSONMap(chatRaw)
	if err != nil {
		return nil, err
	}
	chat.Chat = payload
	chat.Meta = extractChatMeta(payload)
	if shareIDRaw.Valid {
		chat.ShareID = shareIDRaw.String
	}
	if archivedRaw.Valid {
		chat.Archived = archivedRaw.Bool
	}
	if folderIDRaw.Valid {
		chat.FolderID = folderIDRaw.String
	}
	return &chat, nil
}

func paginateMessages(messages []ChatMessage, skip int, limit int) []ChatMessage {
	start, end := paginateBounds(len(messages), skip, limit)
	if start >= end {
		return []ChatMessage{}
	}
	return messages[start:end]
}

func paginateBounds(total int, skip int, limit int) (int, int) {
	if skip < 0 {
		skip = 0
	}
	if limit <= 0 || skip+limit > total {
		limit = total - skip
	}
	if skip > total {
		skip = total
	}
	end := skip + limit
	if end > total {
		end = total
	}
	return skip, end
}

func messageInRange(createdAt int64, startDate *int64, endDate *int64) bool {
	if startDate != nil && createdAt < *startDate {
		return false
	}
	if endDate != nil && createdAt > *endDate {
		return false
	}
	return true
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	default:
		return 0
	}
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		number, _ := typed.Int64()
		return number
	default:
		return 0
	}
}

func stringValueFromAny(value any) string {
	text, _ := value.(string)
	return text
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cloneArbitraryMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

func normalizeTimestamp(timestamp int64) time.Time {
	now := time.Now().Unix()
	if timestamp > 10_000_000_000 {
		timestamp /= 1000
	}
	if timestamp < 1577836800 || timestamp > now+86400 {
		timestamp = now
	}
	return time.Unix(timestamp, 0).UTC()
}

func fillMissingDays(counts map[string]map[string]int, startDate *int64, endDate *int64) {
	if startDate == nil || endDate == nil {
		return
	}
	current := normalizeTimestamp(*startDate)
	end := normalizeTimestamp(*endDate)
	for !current.After(end) {
		key := current.Format("2006-01-02")
		if counts[key] == nil {
			counts[key] = map[string]int{}
		}
		current = current.Add(24 * time.Hour)
	}
}

func fillMissingHours(counts map[string]map[string]int, startDate *int64, endDate *int64) {
	if startDate == nil || endDate == nil {
		return
	}
	current := normalizeTimestamp(*startDate).Truncate(time.Hour)
	end := normalizeTimestamp(*endDate)
	for !current.After(end) {
		key := current.Format("2006-01-02 15:00")
		if counts[key] == nil {
			counts[key] = map[string]int{}
		}
		current = current.Add(time.Hour)
	}
}
