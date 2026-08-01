package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Prompt struct {
	ID        string         `json:"id"`
	Command   string         `json:"command"`
	UserID    string         `json:"user_id"`
	Name      string         `json:"name"`
	Content   string         `json:"content"`
	Data      map[string]any `json:"data,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	IsActive  bool           `json:"is_active"`
	VersionID string         `json:"version_id,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

type PromptCreateParams struct {
	UserID  string
	Command string
	Name    string
	Content string
	Data    map[string]any
	Meta    map[string]any
	Tags    []string
}

type PromptUpdateParams struct {
	Command *string
	Name    *string
	Content *string
	Data    map[string]any
	Meta    map[string]any
	Tags    []string
}

type PromptSearchOptions struct {
	Query string
	Skip  int
	Limit int
}

type PromptsTable struct {
	db *dbinternal.Handle
}

func NewPromptsTable(db *dbinternal.Handle) *PromptsTable {
	return &PromptsTable{db: db}
}

func (t *PromptsTable) InsertNewPrompt(ctx context.Context, params PromptCreateParams) (*Prompt, error) {
	now := time.Now().Unix()
	id := uuid.NewString()
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := marshalJSONStringSlice(params.Tags)
	if err != nil {
		return nil, err
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO prompt (id, command, user_id, name, content, data, meta, tags, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.Command,
		params.UserID,
		params.Name,
		params.Content,
		dataJSON,
		metaJSON,
		tagsJSON,
		true,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetPromptByID(ctx, id)
}

func (t *PromptsTable) GetPromptByID(ctx context.Context, id string) (*Prompt, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, command, user_id, name, content, data, meta, tags, is_active, version_id, created_at, updated_at FROM prompt WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanPromptRow(row)
}

func (t *PromptsTable) GetPromptByCommand(ctx context.Context, command string) (*Prompt, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, command, user_id, name, content, data, meta, tags, is_active, version_id, created_at, updated_at FROM prompt WHERE command = ? LIMIT 1`, t.db.Dialect),
		command,
	)
	return scanPromptRow(row)
}

func (t *PromptsTable) GetPrompts(ctx context.Context) ([]Prompt, error) {
	rows, err := t.db.DB.QueryContext(ctx, `SELECT id, command, user_id, name, content, data, meta, tags, is_active, version_id, created_at, updated_at FROM prompt WHERE is_active = 1 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (t *PromptsTable) GetPromptsByUserID(ctx context.Context, userID string) ([]Prompt, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, command, user_id, name, content, data, meta, tags, is_active, version_id, created_at, updated_at FROM prompt WHERE is_active = 1 AND user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (t *PromptsTable) SearchPrompts(ctx context.Context, userID string, options PromptSearchOptions) ([]Prompt, int, error) {
	query := `SELECT id, command, user_id, name, content, data, meta, tags, is_active, version_id, created_at, updated_at FROM prompt WHERE is_active = 1 AND user_id = ?`
	countQuery := `SELECT COUNT(*) FROM prompt WHERE is_active = 1 AND user_id = ?`
	args := []any{userID}
	if options.Query != "" {
		query += ` AND (LOWER(command) LIKE LOWER(?) OR LOWER(name) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?))`
		countQuery += ` AND (LOWER(command) LIKE LOWER(?) OR LOWER(name) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?))`
		value := "%" + options.Query + "%"
		args = append(args, value, value, value)
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
	items, err := scanPrompts(rows)
	return items, total, err
}

func (t *PromptsTable) UpdatePromptByID(ctx context.Context, id string, params PromptUpdateParams) (*Prompt, error) {
	assignments := []string{}
	args := []any{}
	if params.Command != nil {
		assignments = append(assignments, "command = ?")
		args = append(args, *params.Command)
	}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Content != nil {
		assignments = append(assignments, "content = ?")
		args = append(args, *params.Content)
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
	if params.Tags != nil {
		tagsJSON, err := marshalJSONStringSlice(params.Tags)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "tags = ?")
		args = append(args, tagsJSON)
	}
	if len(assignments) == 0 {
		return t.GetPromptByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE prompt SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetPromptByID(ctx, id)
}

func (t *PromptsTable) DeletePromptByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM prompt WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *PromptsTable) GetTags(ctx context.Context, userID string) ([]string, error) {
	prompts, err := t.GetPromptsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	tagSet := map[string]struct{}{}
	for _, prompt := range prompts {
		for _, tag := range prompt.Tags {
			tagSet[tag] = struct{}{}
		}
	}
	result := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		result = append(result, tag)
	}
	slices.Sort(result)
	return result, nil
}

func scanPromptRow(row *sql.Row) (*Prompt, error) {
	var prompt Prompt
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	var tagsRaw sql.NullString
	var versionIDRaw sql.NullString
	err := row.Scan(&prompt.ID, &prompt.Command, &prompt.UserID, &prompt.Name, &prompt.Content, &dataRaw, &metaRaw, &tagsRaw, &prompt.IsActive, &versionIDRaw, &prompt.CreatedAt, &prompt.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if versionIDRaw.Valid {
		prompt.VersionID = versionIDRaw.String
	}
	var err2 error
	prompt.Data, err2 = unmarshalJSONMap(dataRaw)
	if err2 != nil {
		return nil, err2
	}
	prompt.Meta, err2 = unmarshalJSONMap(metaRaw)
	if err2 != nil {
		return nil, err2
	}
	prompt.Tags, err2 = unmarshalJSONStringSlice(tagsRaw)
	if err2 != nil {
		return nil, err2
	}
	return &prompt, nil
}

func scanPrompts(rows *sql.Rows) ([]Prompt, error) {
	var prompts []Prompt
	for rows.Next() {
		var prompt Prompt
		var dataRaw sql.NullString
		var metaRaw sql.NullString
		var tagsRaw sql.NullString
		var versionIDRaw sql.NullString
		if err := rows.Scan(&prompt.ID, &prompt.Command, &prompt.UserID, &prompt.Name, &prompt.Content, &dataRaw, &metaRaw, &tagsRaw, &prompt.IsActive, &versionIDRaw, &prompt.CreatedAt, &prompt.UpdatedAt); err != nil {
			return nil, err
		}
		if versionIDRaw.Valid {
			prompt.VersionID = versionIDRaw.String
		}
		var err error
		prompt.Data, err = unmarshalJSONMap(dataRaw)
		if err != nil {
			return nil, err
		}
		prompt.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		prompt.Tags, err = unmarshalJSONStringSlice(tagsRaw)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	return prompts, rows.Err()
}

func marshalJSONStringSlice(values []string) (any, error) {
	if values == nil {
		return nil, nil
	}
	body, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return string(body), nil
}

func unmarshalJSONStringSlice(value sql.NullString) ([]string, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(value.String), &items); err != nil {
		return nil, err
	}
	return items, nil
}
