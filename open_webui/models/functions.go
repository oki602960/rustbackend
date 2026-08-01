package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Function struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Content   string         `json:"content"`
	Meta      map[string]any `json:"meta,omitempty"`
	Valves    map[string]any `json:"valves,omitempty"`
	IsActive  bool           `json:"is_active"`
	IsGlobal  bool           `json:"is_global"`
	UpdatedAt int64          `json:"updated_at"`
	CreatedAt int64          `json:"created_at"`
}

type FunctionCreateParams struct {
	ID      string
	UserID  string
	Name    string
	Type    string
	Content string
	Meta    map[string]any
	Valves  map[string]any
}

type FunctionUpdateParams struct {
	Name     *string
	Type     *string
	Content  *string
	Meta     map[string]any
	Valves   map[string]any
	IsActive *bool
	IsGlobal *bool
}

type FunctionsTable struct {
	db *dbinternal.Handle
}

func NewFunctionsTable(db *dbinternal.Handle) *FunctionsTable {
	return &FunctionsTable{db: db}
}

func (t *FunctionsTable) InsertNewFunction(ctx context.Context, params FunctionCreateParams) (*Function, error) {
	now := time.Now().Unix()
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	valvesJSON, err := marshalJSONMap(params.Valves)
	if err != nil {
		return nil, err
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO "function" (id, user_id, name, type, content, meta, valves, is_active, is_global, updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.UserID,
		params.Name,
		params.Type,
		params.Content,
		metaJSON,
		valvesJSON,
		true,
		false,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFunctionByID(ctx, params.ID)
}

func (t *FunctionsTable) GetFunctionByID(ctx context.Context, id string) (*Function, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, type, content, meta, valves, is_active, is_global, updated_at, created_at FROM "function" WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanFunctionRow(row)
}

func (t *FunctionsTable) GetFunctions(ctx context.Context, includeValves bool) ([]Function, error) {
	query := `SELECT id, user_id, name, type, content, meta, valves, is_active, is_global, updated_at, created_at FROM "function" ORDER BY updated_at DESC`
	rows, err := t.db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFunctions(rows)
}

func (t *FunctionsTable) UpdateFunctionByID(ctx context.Context, id string, params FunctionUpdateParams) (*Function, error) {
	assignments := []string{}
	args := []any{}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Type != nil {
		assignments = append(assignments, "type = ?")
		args = append(args, *params.Type)
	}
	if params.Content != nil {
		assignments = append(assignments, "content = ?")
		args = append(args, *params.Content)
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
	if params.IsActive != nil {
		assignments = append(assignments, "is_active = ?")
		args = append(args, *params.IsActive)
	}
	if params.IsGlobal != nil {
		assignments = append(assignments, "is_global = ?")
		args = append(args, *params.IsGlobal)
	}
	if len(assignments) == 0 {
		return t.GetFunctionByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE "function" SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetFunctionByID(ctx, id)
}

func (t *FunctionsTable) DeleteFunctionByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM "function" WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func scanFunctionRow(row *sql.Row) (*Function, error) {
	var function Function
	var metaRaw sql.NullString
	var valvesRaw sql.NullString
	err := row.Scan(&function.ID, &function.UserID, &function.Name, &function.Type, &function.Content, &metaRaw, &valvesRaw, &function.IsActive, &function.IsGlobal, &function.UpdatedAt, &function.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	function.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	function.Valves, err = unmarshalJSONMap(valvesRaw)
	if err != nil {
		return nil, err
	}
	return &function, nil
}

func scanFunctions(rows *sql.Rows) ([]Function, error) {
	var functions []Function
	for rows.Next() {
		var function Function
		var metaRaw sql.NullString
		var valvesRaw sql.NullString
		if err := rows.Scan(&function.ID, &function.UserID, &function.Name, &function.Type, &function.Content, &metaRaw, &valvesRaw, &function.IsActive, &function.IsGlobal, &function.UpdatedAt, &function.CreatedAt); err != nil {
			return nil, err
		}
		var err error
		function.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		function.Valves, err = unmarshalJSONMap(valvesRaw)
		if err != nil {
			return nil, err
		}
		functions = append(functions, function)
	}
	return functions, rows.Err()
}
