package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type OAuthSession struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Provider  string         `json:"provider"`
	Token     map[string]any `json:"token"`
	ExpiresAt int64          `json:"expires_at"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

type OAuthSessionsTable struct {
	db *dbinternal.Handle
}

func NewOAuthSessionsTable(db *dbinternal.Handle) *OAuthSessionsTable {
	return &OAuthSessionsTable{db: db}
}

func (t *OAuthSessionsTable) CreateSession(ctx context.Context, userID string, provider string, token map[string]any) (*OAuthSession, error) {
	body, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	expiresAt := int64(0)
	switch value := token["expires_at"].(type) {
	case float64:
		expiresAt = int64(value)
	case int64:
		expiresAt = value
	case int:
		expiresAt = int64(value)
	}
	id := uuid.NewString()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO oauth_session (id, user_id, provider, token, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		userID,
		provider,
		string(body),
		expiresAt,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetSessionByID(ctx, id)
}

func (t *OAuthSessionsTable) GetSessionByID(ctx context.Context, sessionID string) (*OAuthSession, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, provider, token, expires_at, created_at, updated_at FROM oauth_session WHERE id = ? LIMIT 1`, t.db.Dialect),
		sessionID,
	)
	return scanOAuthSessionRow(row)
}

func (t *OAuthSessionsTable) GetSessionsByUserID(ctx context.Context, userID string) ([]OAuthSession, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, provider, token, expires_at, created_at, updated_at FROM oauth_session WHERE user_id = ? ORDER BY created_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []OAuthSession
	for rows.Next() {
		session, err := scanOAuthSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, rows.Err()
}

func scanOAuthSessionRow(row *sql.Row) (*OAuthSession, error) {
	var session OAuthSession
	var tokenRaw string
	err := row.Scan(&session.ID, &session.UserID, &session.Provider, &tokenRaw, &session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(tokenRaw), &session.Token); err != nil {
		return nil, err
	}
	return &session, nil
}

func scanOAuthSessionRows(rows *sql.Rows) (*OAuthSession, error) {
	var session OAuthSession
	var tokenRaw string
	if err := rows.Scan(&session.ID, &session.UserID, &session.Provider, &tokenRaw, &session.ExpiresAt, &session.CreatedAt, &session.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(tokenRaw), &session.Token); err != nil {
		return nil, err
	}
	return &session, nil
}
