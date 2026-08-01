package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Folder struct {
	ID         string         `json:"id"`
	ParentID   string         `json:"parent_id,omitempty"`
	UserID     string         `json:"user_id"`
	Name       string         `json:"name"`
	Items      map[string]any `json:"items,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	IsExpanded bool           `json:"is_expanded"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
}

type FolderCreateParams struct {
	UserID     string
	ParentID   *string
	Name       string
	Data       map[string]any
	Meta       map[string]any
	Items      map[string]any
	IsExpanded bool
}

type FolderUpdateParams struct {
	Name       *string
	Data       map[string]any
	Meta       map[string]any
	ParentID   *string
	IsExpanded *bool
}

type FoldersTable struct {
	db *dbinternal.Handle
}

func NewFoldersTable(db *dbinternal.Handle) *FoldersTable {
	return &FoldersTable{db: db}
}

func (t *FoldersTable) InsertNewFolder(ctx context.Context, params FolderCreateParams) (*Folder, error) {
	now := time.Now().Unix()
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	itemsJSON, err := marshalJSONMap(params.Items)
	if err != nil {
		return nil, err
	}
	parentID := any(nil)
	if params.ParentID != nil {
		parentID = *params.ParentID
	}
	id := uuid.NewString()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO folder (id, parent_id, user_id, name, items, meta, data, is_expanded, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		parentID,
		params.UserID,
		params.Name,
		itemsJSON,
		metaJSON,
		dataJSON,
		params.IsExpanded,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFolderByIDAndUserID(ctx, id, params.UserID)
}

func (t *FoldersTable) GetFoldersByUserID(ctx context.Context, userID string) ([]Folder, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, parent_id, user_id, name, items, meta, data, is_expanded, created_at, updated_at FROM folder WHERE user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFolders(rows)
}

func (t *FoldersTable) GetFolderByIDAndUserID(ctx context.Context, id string, userID string) (*Folder, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, parent_id, user_id, name, items, meta, data, is_expanded, created_at, updated_at FROM folder WHERE id = ? AND user_id = ? LIMIT 1`, t.db.Dialect),
		id,
		userID,
	)
	return scanFolderRow(row)
}

func (t *FoldersTable) GetFolderByParentIDAndUserIDAndName(ctx context.Context, parentID *string, userID string, name string) (*Folder, error) {
	query := `SELECT id, parent_id, user_id, name, items, meta, data, is_expanded, created_at, updated_at FROM folder WHERE user_id = ? AND name = ? AND `
	args := []any{userID, name}
	if parentID == nil {
		query += `parent_id IS NULL LIMIT 1`
	} else {
		query += `parent_id = ? LIMIT 1`
		args = append(args, *parentID)
	}
	row := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...)
	return scanFolderRow(row)
}

func (t *FoldersTable) UpdateFolderByIDAndUserID(ctx context.Context, id string, userID string, params FolderUpdateParams) (*Folder, error) {
	assignments := []string{}
	args := []any{}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Data != nil {
		dataJSON, err := marshalJSONMap(params.Data)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "data = ?")
		args = append(args, dataJSON)
	}
	if params.Meta != nil {
		metaJSON, err := marshalJSONMap(params.Meta)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "meta = ?")
		args = append(args, metaJSON)
	}
	if params.ParentID != nil {
		assignments = append(assignments, "parent_id = ?")
		args = append(args, *params.ParentID)
	}
	if params.IsExpanded != nil {
		assignments = append(assignments, "is_expanded = ?")
		args = append(args, *params.IsExpanded)
	}
	if len(assignments) == 0 {
		return t.GetFolderByIDAndUserID(ctx, id, userID)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id, userID)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE folder SET `+joinAssignments(assignments)+` WHERE id = ? AND user_id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFolderByIDAndUserID(ctx, id, userID)
}

func (t *FoldersTable) DeleteFolderByIDAndUserID(ctx context.Context, id string, userID string) (bool, error) {
	result, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM folder WHERE id = ? AND user_id = ?`, t.db.Dialect),
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

func scanFolderRow(row *sql.Row) (*Folder, error) {
	var folder Folder
	var parentIDRaw sql.NullString
	var itemsRaw sql.NullString
	var metaRaw sql.NullString
	var dataRaw sql.NullString
	err := row.Scan(&folder.ID, &parentIDRaw, &folder.UserID, &folder.Name, &itemsRaw, &metaRaw, &dataRaw, &folder.IsExpanded, &folder.CreatedAt, &folder.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if parentIDRaw.Valid {
		folder.ParentID = parentIDRaw.String
	}
	folder.Items, err = unmarshalJSONMap(itemsRaw)
	if err != nil {
		return nil, err
	}
	folder.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	folder.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func scanFolders(rows *sql.Rows) ([]Folder, error) {
	var folders []Folder
	for rows.Next() {
		var folder Folder
		var parentIDRaw sql.NullString
		var itemsRaw sql.NullString
		var metaRaw sql.NullString
		var dataRaw sql.NullString
		if err := rows.Scan(&folder.ID, &parentIDRaw, &folder.UserID, &folder.Name, &itemsRaw, &metaRaw, &dataRaw, &folder.IsExpanded, &folder.CreatedAt, &folder.UpdatedAt); err != nil {
			return nil, err
		}
		if parentIDRaw.Valid {
			folder.ParentID = parentIDRaw.String
		}
		var err error
		folder.Items, err = unmarshalJSONMap(itemsRaw)
		if err != nil {
			return nil, err
		}
		folder.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		folder.Data, err = unmarshalJSONMap(dataRaw)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}
