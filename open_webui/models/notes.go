package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Note struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	Title         string         `json:"title"`
	Data          map[string]any `json:"data,omitempty"`
	Meta          map[string]any `json:"meta,omitempty"`
	AccessControl map[string]any `json:"access_control,omitempty"`
	CreatedAt     int64          `json:"created_at"`
	UpdatedAt     int64          `json:"updated_at"`
}

type NoteCreateParams struct {
	UserID string
	Title  string
	Data   map[string]any
	Meta   map[string]any
}

type NoteUpdateParams struct {
	Title *string
	Data  map[string]any
	Meta  map[string]any
}

type NoteListOptions struct {
	Query string
	Skip  int
	Limit int
}

type NotesTable struct {
	db *dbinternal.Handle
}

func NewNotesTable(db *dbinternal.Handle) *NotesTable {
	return &NotesTable{db: db}
}

func (t *NotesTable) InsertNewNote(ctx context.Context, params NoteCreateParams) (*Note, error) {
	now := time.Now().UnixNano()
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO note (id, user_id, title, data, meta, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.UserID,
		params.Title,
		dataJSON,
		metaJSON,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetNoteByID(ctx, id)
}

func (t *NotesTable) GetNoteByID(ctx context.Context, id string) (*Note, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, title, data, meta, access_control, created_at, updated_at FROM note WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanNoteRow(row)
}

func (t *NotesTable) GetNotesByUserID(ctx context.Context, userID string, skip int, limit int) ([]Note, error) {
	query := `SELECT id, user_id, title, data, meta, access_control, created_at, updated_at FROM note WHERE user_id = ? ORDER BY updated_at DESC`
	args := []any{userID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	if skip > 0 {
		query += ` OFFSET ?`
		args = append(args, skip)
	}
	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

func (t *NotesTable) SearchNotes(ctx context.Context, userID string, options NoteListOptions) ([]Note, int, error) {
	query := `SELECT id, user_id, title, data, meta, access_control, created_at, updated_at FROM note WHERE user_id = ?`
	countQuery := `SELECT COUNT(*) FROM note WHERE user_id = ?`
	args := []any{userID}
	if options.Query != "" {
		query += ` AND LOWER(title) LIKE LOWER(?)`
		countQuery += ` AND LOWER(title) LIKE LOWER(?)`
		args = append(args, "%"+options.Query+"%")
	}
	query += ` ORDER BY updated_at DESC`

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
	notes, err := scanNotes(rows)
	return notes, total, err
}

func (t *NotesTable) UpdateNoteByID(ctx context.Context, id string, params NoteUpdateParams) (*Note, error) {
	assignments := []string{}
	args := []any{}
	if params.Title != nil {
		assignments = append(assignments, "title = ?")
		args = append(args, *params.Title)
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
	if len(assignments) == 0 {
		return t.GetNoteByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().UnixNano(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE note SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetNoteByID(ctx, id)
}

func (t *NotesTable) DeleteNoteByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM note WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func scanNoteRow(row *sql.Row) (*Note, error) {
	var note Note
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	var accessControlRaw sql.NullString
	err := row.Scan(&note.ID, &note.UserID, &note.Title, &dataRaw, &metaRaw, &accessControlRaw, &note.CreatedAt, &note.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	note.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	note.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	note.AccessControl, err = unmarshalJSONMap(accessControlRaw)
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func scanNotes(rows *sql.Rows) ([]Note, error) {
	var notes []Note
	for rows.Next() {
		var note Note
		var dataRaw sql.NullString
		var metaRaw sql.NullString
		var accessControlRaw sql.NullString
		if err := rows.Scan(&note.ID, &note.UserID, &note.Title, &dataRaw, &metaRaw, &accessControlRaw, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		var err error
		note.Data, err = unmarshalJSONMap(dataRaw)
		if err != nil {
			return nil, err
		}
		note.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		note.AccessControl, err = unmarshalJSONMap(accessControlRaw)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}
