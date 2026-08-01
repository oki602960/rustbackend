package models

import (
	"context"
	"database/sql"
	"errors"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Chat struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Title     string         `json:"title"`
	Chat      map[string]any `json:"chat"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
	ShareID   string         `json:"share_id,omitempty"`
	Archived  bool           `json:"archived"`
	FolderID  string         `json:"folder_id,omitempty"`
}

type ChatsTable struct {
	db *dbinternal.Handle
}

func NewChatsTable(db *dbinternal.Handle) *ChatsTable {
	return &ChatsTable{db: db}
}

func (t *ChatsTable) GetChatByID(ctx context.Context, id string) (*Chat, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, title, chat, created_at, updated_at, share_id, archived, folder_id FROM chat WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanChatRow(row)
}

func scanChatRow(row *sql.Row) (*Chat, error) {
	var chat Chat
	var titleRaw sql.NullString
	var chatRaw sql.NullString
	var shareIDRaw sql.NullString
	var archivedRaw sql.NullBool
	var folderIDRaw sql.NullString
	err := row.Scan(&chat.ID, &chat.UserID, &titleRaw, &chatRaw, &chat.CreatedAt, &chat.UpdatedAt, &shareIDRaw, &archivedRaw, &folderIDRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
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

func extractChatMeta(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	if meta, ok := payload["meta"].(map[string]any); ok {
		cloned := cloneArbitraryMap(meta)
		if cloned != nil {
			return cloned
		}
		return map[string]any{}
	}
	if tags, ok := payload["tags"]; ok {
		return map[string]any{"tags": tags}
	}
	return map[string]any{}
}
