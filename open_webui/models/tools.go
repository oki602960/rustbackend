package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Tool struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Name      string           `json:"name"`
	Content   string           `json:"content"`
	Specs     []map[string]any `json:"specs,omitempty"`
	Meta      map[string]any   `json:"meta,omitempty"`
	Valves    map[string]any   `json:"valves,omitempty"`
	UpdatedAt int64            `json:"updated_at"`
	CreatedAt int64            `json:"created_at"`
}

type ToolCreateParams struct {
	ID      string
	UserID  string
	Name    string
	Content string
	Specs   []map[string]any
	Meta    map[string]any
	Valves  map[string]any
}

type ToolUpdateParams struct {
	Name    *string
	Content *string
	Specs   []map[string]any
	Meta    map[string]any
	Valves  map[string]any
}

type ToolsTable struct {
	db *dbinternal.Handle
}

func NewToolsTable(db *dbinternal.Handle) *ToolsTable {
	return &ToolsTable{db: db}
}

func (t *ToolsTable) InsertNewTool(ctx context.Context, params ToolCreateParams) (*Tool, error) {
	specsJSON, err := marshalJSONMapList(params.Specs)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	valvesJSON, err := marshalJSONMap(params.Valves)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO tool (id, user_id, name, content, specs, meta, valves, updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.UserID,
		params.Name,
		params.Content,
		specsJSON,
		metaJSON,
		valvesJSON,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetToolByID(ctx, params.ID)
}

func (t *ToolsTable) GetToolByID(ctx context.Context, id string) (*Tool, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, content, specs, meta, valves, updated_at, created_at FROM tool WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanToolRow(row)
}

func (t *ToolsTable) GetTools(ctx context.Context) ([]Tool, error) {
	rows, err := t.db.DB.QueryContext(ctx, `SELECT id, user_id, name, content, specs, meta, valves, updated_at, created_at FROM tool ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTools(rows)
}

func (t *ToolsTable) GetToolsByUserID(ctx context.Context, userID string) ([]Tool, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, content, specs, meta, valves, updated_at, created_at FROM tool WHERE user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTools(rows)
}

func (t *ToolsTable) UpdateToolByID(ctx context.Context, id string, params ToolUpdateParams) (*Tool, error) {
	assignments := []string{}
	args := []any{}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Content != nil {
		assignments = append(assignments, "content = ?")
		args = append(args, *params.Content)
	}
	if params.Specs != nil {
		specsJSON, err := marshalJSONMapList(params.Specs)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "specs = ?")
		args = append(args, specsJSON)
	}
	if params.Meta != nil {
		metaJSON, err := marshalJSONMap(params.Meta)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "meta = ?")
		args = append(args, metaJSON)
	}
	if params.Valves != nil {
		valvesJSON, err := marshalJSONMap(params.Valves)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "valves = ?")
		args = append(args, valvesJSON)
	}
	if len(assignments) == 0 {
		return t.GetToolByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE tool SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetToolByID(ctx, id)
}

func (t *ToolsTable) DeleteToolByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM tool WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func scanToolRow(row *sql.Row) (*Tool, error) {
	var tool Tool
	var specsRaw sql.NullString
	var metaRaw sql.NullString
	var valvesRaw sql.NullString
	err := row.Scan(&tool.ID, &tool.UserID, &tool.Name, &tool.Content, &specsRaw, &metaRaw, &valvesRaw, &tool.UpdatedAt, &tool.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tool.Specs, err = unmarshalJSONMapList(specsRaw)
	if err != nil {
		return nil, err
	}
	tool.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	tool.Valves, err = unmarshalJSONMap(valvesRaw)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func scanTools(rows *sql.Rows) ([]Tool, error) {
	var tools []Tool
	for rows.Next() {
		var tool Tool
		var specsRaw sql.NullString
		var metaRaw sql.NullString
		var valvesRaw sql.NullString
		if err := rows.Scan(&tool.ID, &tool.UserID, &tool.Name, &tool.Content, &specsRaw, &metaRaw, &valvesRaw, &tool.UpdatedAt, &tool.CreatedAt); err != nil {
			return nil, err
		}
		var err error
		tool.Specs, err = unmarshalJSONMapList(specsRaw)
		if err != nil {
			return nil, err
		}
		tool.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		tool.Valves, err = unmarshalJSONMap(valvesRaw)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}

func marshalJSONMapList(items []map[string]any) (any, error) {
	if items == nil {
		return nil, nil
	}
	body, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return string(body), nil
}

func unmarshalJSONMapList(value sql.NullString) ([]map[string]any, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(value.String), &items); err != nil {
		return nil, err
	}
	return items, nil
}
