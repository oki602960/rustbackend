package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Memory struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedAt int64  `json:"created_at"`
}

type MemoriesTable struct {
	db *dbinternal.Handle
}

func NewMemoriesTable(db *dbinternal.Handle) *MemoriesTable {
	return &MemoriesTable{db: db}
}

func (t *MemoriesTable) InsertNewMemory(ctx context.Context, userID string, content string) (*Memory, error) {
	now := time.Now().Unix()
	id := uuid.NewString()
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO memory (id, user_id, content, updated_at, created_at) VALUES (?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		userID,
		content,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetMemoryByID(ctx, id)
}

func (t *MemoriesTable) GetMemoriesByUserID(ctx context.Context, userID string) ([]Memory, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, content, updated_at, created_at FROM memory WHERE user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var memory Memory
		if err := rows.Scan(&memory.ID, &memory.UserID, &memory.Content, &memory.UpdatedAt, &memory.CreatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func (t *MemoriesTable) GetMemoryByID(ctx context.Context, id string) (*Memory, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, content, updated_at, created_at FROM memory WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	var memory Memory
	err := row.Scan(&memory.ID, &memory.UserID, &memory.Content, &memory.UpdatedAt, &memory.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &memory, nil
}

func (t *MemoriesTable) UpdateMemoryByIDAndUserID(ctx context.Context, id string, userID string, content string) (*Memory, error) {
	result, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE memory SET content = ?, updated_at = ? WHERE id = ? AND user_id = ?`, t.db.Dialect),
		content,
		time.Now().Unix(),
		id,
		userID,
	)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, nil
	}
	return t.GetMemoryByID(ctx, id)
}

func (t *MemoriesTable) DeleteMemoryByIDAndUserID(ctx context.Context, id string, userID string) (bool, error) {
	result, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM memory WHERE id = ? AND user_id = ?`, t.db.Dialect),
		id,
		userID,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *MemoriesTable) DeleteMemoriesByUserID(ctx context.Context, userID string) (bool, error) {
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM memory WHERE user_id = ?`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return false, err
	}
	return true, nil
}
