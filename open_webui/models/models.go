package models

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Model struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	BaseModelID *string        `json:"base_model_id,omitempty"`
	Name        string         `json:"name"`
	Params      map[string]any `json:"params,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	IsActive    bool           `json:"is_active"`
	UpdatedAt   int64          `json:"updated_at"`
	CreatedAt   int64          `json:"created_at"`
}

type ModelCreateParams struct {
	ID          string
	UserID      string
	BaseModelID *string
	Name        string
	Params      map[string]any
	Meta        map[string]any
	IsActive    bool
}

type ModelUpdateParams struct {
	BaseModelID *string
	Name        *string
	Params      map[string]any
	Meta        map[string]any
	IsActive    *bool
}

type ModelSearchOptions struct {
	Query     string
	OrderBy   string
	Direction string
	Skip      int
	Limit     int
	BaseOnly  bool
}

type ModelsTable struct {
	db *dbinternal.Handle
}

func NewModelsTable(db *dbinternal.Handle) *ModelsTable {
	return &ModelsTable{db: db}
}

func (t *ModelsTable) InsertNewModel(ctx context.Context, params ModelCreateParams) (*Model, error) {
	now := time.Now().Unix()
	paramsJSON, err := marshalJSONMap(params.Params)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	baseModelID := any(nil)
	if params.BaseModelID != nil {
		baseModelID = *params.BaseModelID
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO model (id, user_id, base_model_id, name, params, meta, is_active, updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.UserID,
		baseModelID,
		params.Name,
		paramsJSON,
		metaJSON,
		params.IsActive,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetModelByID(ctx, params.ID)
}

func (t *ModelsTable) GetModelByID(ctx context.Context, id string) (*Model, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, base_model_id, name, params, meta, is_active, updated_at, created_at FROM model WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanModelRow(row)
}

func (t *ModelsTable) GetModels(ctx context.Context, baseOnly bool) ([]Model, error) {
	query := `SELECT id, user_id, base_model_id, name, params, meta, is_active, updated_at, created_at FROM model`
	if baseOnly {
		query += ` WHERE base_model_id IS NULL`
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := t.db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

func (t *ModelsTable) GetModelsByUserID(ctx context.Context, userID string) ([]Model, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, base_model_id, name, params, meta, is_active, updated_at, created_at FROM model WHERE user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

func (t *ModelsTable) SearchModels(ctx context.Context, userID string, options ModelSearchOptions) ([]Model, int, error) {
	query := `SELECT id, user_id, base_model_id, name, params, meta, is_active, updated_at, created_at FROM model`
	countQuery := `SELECT COUNT(*) FROM model`
	clauses := []string{}
	args := []any{}
	if !options.BaseOnly {
		clauses = append(clauses, `base_model_id IS NOT NULL`)
	}
	if userID != "" {
		clauses = append(clauses, `user_id = ?`)
		args = append(args, userID)
	}
	if options.Query != "" {
		clauses = append(clauses, `(LOWER(name) LIKE LOWER(?) OR LOWER(COALESCE(base_model_id, '')) LIKE LOWER(?))`)
		value := "%" + options.Query + "%"
		args = append(args, value, value)
	}
	if len(clauses) > 0 {
		where := " WHERE " + joinAssignments(clauses)
		where = replaceAndSeparator(where)
		query += where
		countQuery += where
	}
	orderBy := "created_at"
	switch options.OrderBy {
	case "name", "created_at", "updated_at":
		orderBy = options.OrderBy
	}
	direction := "DESC"
	if options.Direction == "asc" {
		direction = "ASC"
	}
	query += " ORDER BY " + orderBy + " " + direction
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
	items, err := scanModels(rows)
	return items, total, err
}

func (t *ModelsTable) UpdateModelByID(ctx context.Context, id string, params ModelUpdateParams) (*Model, error) {
	assignments := []string{}
	args := []any{}
	if params.BaseModelID != nil {
		assignments = append(assignments, "base_model_id = ?")
		args = append(args, *params.BaseModelID)
	}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Params != nil {
		paramsJSON, err := marshalJSONMap(params.Params)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "params = ?")
		args = append(args, paramsJSON)
	}
	if params.Meta != nil {
		metaJSON, err := marshalJSONMap(params.Meta)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "meta = ?")
		args = append(args, metaJSON)
	}
	if params.IsActive != nil {
		assignments = append(assignments, "is_active = ?")
		args = append(args, *params.IsActive)
	}
	if len(assignments) == 0 {
		return t.GetModelByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE model SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetModelByID(ctx, id)
}

func (t *ModelsTable) ToggleModelByID(ctx context.Context, id string) (*Model, error) {
	model, err := t.GetModelByID(ctx, id)
	if err != nil || model == nil {
		return model, err
	}
	next := !model.IsActive
	return t.UpdateModelByID(ctx, id, ModelUpdateParams{IsActive: &next})
}

func (t *ModelsTable) DeleteModelByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM model WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *ModelsTable) GetTags(ctx context.Context, userID string) ([]string, error) {
	models, err := t.GetModelsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	tagsSet := map[string]struct{}{}
	for _, model := range models {
		rawTags, ok := model.Meta["tags"]
		if !ok {
			continue
		}
		switch tags := rawTags.(type) {
		case []any:
			for _, tag := range tags {
				switch value := tag.(type) {
				case string:
					tagsSet[value] = struct{}{}
				case map[string]any:
					if name, ok := value["name"].(string); ok {
						tagsSet[name] = struct{}{}
					}
				}
			}
		}
	}
	result := make([]string, 0, len(tagsSet))
	for tag := range tagsSet {
		result = append(result, tag)
	}
	slices.Sort(result)
	return result, nil
}

func scanModelRow(row *sql.Row) (*Model, error) {
	var model Model
	var baseModelIDRaw sql.NullString
	var paramsRaw sql.NullString
	var metaRaw sql.NullString
	err := row.Scan(&model.ID, &model.UserID, &baseModelIDRaw, &model.Name, &paramsRaw, &metaRaw, &model.IsActive, &model.UpdatedAt, &model.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if baseModelIDRaw.Valid {
		model.BaseModelID = &baseModelIDRaw.String
	}
	model.Params, err = unmarshalJSONMap(paramsRaw)
	if err != nil {
		return nil, err
	}
	model.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	return &model, nil
}

func scanModels(rows *sql.Rows) ([]Model, error) {
	var models []Model
	for rows.Next() {
		var model Model
		var baseModelIDRaw sql.NullString
		var paramsRaw sql.NullString
		var metaRaw sql.NullString
		if err := rows.Scan(&model.ID, &model.UserID, &baseModelIDRaw, &model.Name, &paramsRaw, &metaRaw, &model.IsActive, &model.UpdatedAt, &model.CreatedAt); err != nil {
			return nil, err
		}
		if baseModelIDRaw.Valid {
			model.BaseModelID = &baseModelIDRaw.String
		}
		var err error
		model.Params, err = unmarshalJSONMap(paramsRaw)
		if err != nil {
			return nil, err
		}
		model.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func replaceAndSeparator(where string) string {
	return strings.ReplaceAll(where, ", ", " AND ")
}
