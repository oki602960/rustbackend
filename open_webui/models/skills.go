package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Skill struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Content     string         `json:"content"`
	Meta        map[string]any `json:"meta,omitempty"`
	IsActive    bool           `json:"is_active"`
	UpdatedAt   int64          `json:"updated_at"`
	CreatedAt   int64          `json:"created_at"`
}

type SkillCreateParams struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Content     string
	Meta        map[string]any
	IsActive    bool
}

type SkillUpdateParams struct {
	Name        *string
	Description *string
	Content     *string
	Meta        map[string]any
	IsActive    *bool
}

type SkillSearchOptions struct {
	Query string
	Skip  int
	Limit int
}

type SkillsTable struct {
	db *dbinternal.Handle
}

func NewSkillsTable(db *dbinternal.Handle) *SkillsTable {
	return &SkillsTable{db: db}
}

func (t *SkillsTable) InsertNewSkill(ctx context.Context, params SkillCreateParams) (*Skill, error) {
	now := time.Now().Unix()
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO skill (id, user_id, name, description, content, meta, is_active, updated_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		params.ID,
		params.UserID,
		params.Name,
		params.Description,
		params.Content,
		metaJSON,
		params.IsActive,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetSkillByID(ctx, params.ID)
}

func (t *SkillsTable) GetSkillByID(ctx context.Context, id string) (*Skill, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, description, content, meta, is_active, updated_at, created_at FROM skill WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanSkillRow(row)
}

func (t *SkillsTable) GetSkills(ctx context.Context) ([]Skill, error) {
	rows, err := t.db.DB.QueryContext(ctx, `SELECT id, user_id, name, description, content, meta, is_active, updated_at, created_at FROM skill ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSkills(rows)
}

func (t *SkillsTable) GetSkillsByUserID(ctx context.Context, userID string) ([]Skill, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, description, content, meta, is_active, updated_at, created_at FROM skill WHERE user_id = ? ORDER BY updated_at DESC`, t.db.Dialect),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSkills(rows)
}

func (t *SkillsTable) SearchSkills(ctx context.Context, userID string, options SkillSearchOptions) ([]Skill, int, error) {
	query := `SELECT id, user_id, name, description, content, meta, is_active, updated_at, created_at FROM skill WHERE user_id = ?`
	countQuery := `SELECT COUNT(*) FROM skill WHERE user_id = ?`
	args := []any{userID}
	if options.Query != "" {
		query += ` AND (LOWER(name) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?) OR LOWER(id) LIKE LOWER(?))`
		countQuery += ` AND (LOWER(name) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?) OR LOWER(id) LIKE LOWER(?))`
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
	items, err := scanSkills(rows)
	return items, total, err
}

func (t *SkillsTable) UpdateSkillByID(ctx context.Context, id string, params SkillUpdateParams) (*Skill, error) {
	assignments := []string{}
	args := []any{}
	if params.Name != nil {
		assignments = append(assignments, "name = ?")
		args = append(args, *params.Name)
	}
	if params.Description != nil {
		assignments = append(assignments, "description = ?")
		args = append(args, *params.Description)
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
	if params.IsActive != nil {
		assignments = append(assignments, "is_active = ?")
		args = append(args, *params.IsActive)
	}
	if len(assignments) == 0 {
		return t.GetSkillByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE skill SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetSkillByID(ctx, id)
}

func (t *SkillsTable) DeleteSkillByID(ctx context.Context, id string) (bool, error) {
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM skill WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func scanSkillRow(row *sql.Row) (*Skill, error) {
	var skill Skill
	var metaRaw sql.NullString
	err := row.Scan(&skill.ID, &skill.UserID, &skill.Name, &skill.Description, &skill.Content, &metaRaw, &skill.IsActive, &skill.UpdatedAt, &skill.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	skill.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	return &skill, nil
}

func scanSkills(rows *sql.Rows) ([]Skill, error) {
	var skills []Skill
	for rows.Next() {
		var skill Skill
		var metaRaw sql.NullString
		if err := rows.Scan(&skill.ID, &skill.UserID, &skill.Name, &skill.Description, &skill.Content, &metaRaw, &skill.IsActive, &skill.UpdatedAt, &skill.CreatedAt); err != nil {
			return nil, err
		}
		var err error
		skill.Meta, err = unmarshalJSONMap(metaRaw)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}
