package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type File struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Hash      string         `json:"hash,omitempty"`
	Filename  string         `json:"filename"`
	Path      string         `json:"path,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

type FileCreateParams struct {
	ID       string
	UserID   string
	Hash     string
	Filename string
	Path     string
	Data     map[string]any
	Meta     map[string]any
}

type FileListOptions struct {
	UserID string
	Query  string
	Skip   int
	Limit  int
}

type FilesTable struct {
	db *dbinternal.Handle
}

func NewFilesTable(db *dbinternal.Handle) *FilesTable {
	return &FilesTable{db: db}
}

func (t *FilesTable) InsertNewFile(ctx context.Context, params FileCreateParams) (*File, error) {
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO file (id, user_id, hash, filename, path, data, meta, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.UserID,
		nilIfEmptyString(params.Hash),
		params.Filename,
		nilIfEmptyString(params.Path),
		dataJSON,
		metaJSON,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFileByID(ctx, params.ID)
}

func (t *FilesTable) GetFileByID(ctx context.Context, id string) (*File, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, hash, filename, path, data, meta, created_at, updated_at FROM file WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanFileRow(row)
}

func (t *FilesTable) GetFileByIDAndUserID(ctx context.Context, id string, userID string) (*File, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, hash, filename, path, data, meta, created_at, updated_at FROM file WHERE id = ? AND user_id = ? LIMIT 1`, t.db.Dialect),
		id,
		userID,
	)
	return scanFileRow(row)
}

func (t *FilesTable) GetFileList(ctx context.Context, options FileListOptions) ([]File, int, error) {
	query := `SELECT id, user_id, hash, filename, path, data, meta, created_at, updated_at FROM file`
	countQuery := `SELECT COUNT(*) FROM file`
	args := []any{}
	clauses := []string{}
	if options.UserID != "" {
		clauses = append(clauses, `user_id = ?`)
		args = append(args, options.UserID)
	}
	if options.Query != "" {
		clauses = append(clauses, `LOWER(filename) LIKE LOWER(?)`)
		args = append(args, "%"+options.Query+"%")
	}
	if len(clauses) > 0 {
		where := " WHERE " + joinAssignments(clauses)
		where = replaceAndSeparator(where)
		query += where
		countQuery += where
	}
	query += ` ORDER BY updated_at DESC, id DESC`
	var total int
	if err := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(countQuery, t.db.Dialect), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	pagedArgs := append([]any{}, args...)
	if options.Limit > 0 {
		query += ` LIMIT ?`
		pagedArgs = append(pagedArgs, options.Limit)
	}
	if options.Skip > 0 {
		query += ` OFFSET ?`
		pagedArgs = append(pagedArgs, options.Skip)
	}
	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanFiles(rows)
	return items, total, err
}

func (t *FilesTable) DeleteFileByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM file WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func scanFileRow(row *sql.Row) (*File, error) {
	var file File
	var hashRaw sql.NullString
	var pathRaw sql.NullString
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	err := row.Scan(&file.ID, &file.UserID, &hashRaw, &file.Filename, &pathRaw, &dataRaw, &metaRaw, &file.CreatedAt, &file.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	file.Hash = hashRaw.String
	file.Path = pathRaw.String
	file.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	file.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func scanFiles(rows *sql.Rows) ([]File, error) {
	var files []File
	for rows.Next() {
		var file File
		var hashRaw sql.NullString
		var pathRaw sql.NullString
		var dataRaw sql.NullString
		var metaRaw sql.NullString
		if err := rows.Scan(&file.ID, &file.UserID, &hashRaw, &file.Filename, &pathRaw, &dataRaw, &metaRaw, &file.CreatedAt, &file.UpdatedAt); err != nil {
			return nil, err
		}
		file.Hash = hashRaw.String
		file.Path = pathRaw.String
		var err error
		file.Data, err = unmarshalJSONMap(dataRaw)
		if err != nil {
			return nil, err
		}
		file.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func nilIfEmptyString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
