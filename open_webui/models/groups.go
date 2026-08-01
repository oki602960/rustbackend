package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type Group struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Data        map[string]any
	Meta        map[string]any
	Permissions map[string]any
	CreatedAt   int64
	UpdatedAt   int64
}

type GroupCreateParams struct {
	UserID      string
	Name        string
	Description string
	Data        map[string]any
	Meta        map[string]any
	Permissions map[string]any
}

type GroupUpdateParams struct {
	Name        *string
	Description *string
	Data        map[string]any
	Meta        map[string]any
	Permissions map[string]any
}

type GroupsTable struct {
	db *dbinternal.Handle
}

func NewGroupsTable(db *dbinternal.Handle) *GroupsTable {
	return &GroupsTable{db: db}
}

func (t *GroupsTable) InsertNewGroup(ctx context.Context, params GroupCreateParams) (*Group, error) {
	now := time.Now().Unix()
	dataJSON, err := marshalJSONMap(params.Data)
	if err != nil {
		return nil, err
	}
	metaJSON, err := marshalJSONMap(params.Meta)
	if err != nil {
		return nil, err
	}
	permissionsJSON, err := marshalJSONMap(params.Permissions)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	_, err = t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`INSERT INTO "group" (id, user_id, name, description, data, meta, permissions, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
		id,
		params.UserID,
		params.Name,
		params.Description,
		dataJSON,
		metaJSON,
		permissionsJSON,
		now,
		now,
	)
	if err != nil {
		return nil, err
	}
	return t.GetGroupByID(ctx, id)
}

func (t *GroupsTable) GetGroups(ctx context.Context, memberID string) ([]Group, error) {
	query := `SELECT g.id, g.user_id, g.name, g.description, g.data, g.meta, g.permissions, g.created_at, g.updated_at FROM "group" g`
	args := []any{}
	if memberID != "" {
		query += ` JOIN group_member gm ON gm.group_id = g.id WHERE gm.user_id = ?`
		args = append(args, memberID)
	}
	query += ` ORDER BY g.updated_at DESC`

	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}
	return groups, rows.Err()
}

func (t *GroupsTable) GetGroupsByMemberID(ctx context.Context, userID string) ([]Group, error) {
	return t.GetGroups(ctx, userID)
}

func (t *GroupsTable) GetGroupByID(ctx context.Context, id string) (*Group, error) {
	row := t.db.DB.QueryRowContext(
		ctx,
		rebindPlaceholders(`SELECT id, user_id, name, description, data, meta, permissions, created_at, updated_at FROM "group" WHERE id = ? LIMIT 1`, t.db.Dialect),
		id,
	)
	return scanGroupRow(row)
}

func (t *GroupsTable) UpdateGroupByID(ctx context.Context, id string, params GroupUpdateParams) (*Group, error) {
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
	if params.Permissions != nil {
		permissionsJSON, err := marshalJSONMap(params.Permissions)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, "permissions = ?")
		args = append(args, permissionsJSON)
	}
	if len(assignments) == 0 {
		return t.GetGroupByID(ctx, id)
	}

	assignments = append(assignments, "updated_at = ?")
	args = append(args, time.Now().Unix(), id)
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`UPDATE "group" SET `+joinAssignments(assignments)+` WHERE id = ?`, t.db.Dialect),
		args...,
	)
	if err != nil {
		return nil, err
	}
	return t.GetGroupByID(ctx, id)
}

func (t *GroupsTable) DeleteGroupByID(ctx context.Context, id string) (bool, error) {
	if _, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM group_member WHERE group_id = ?`, t.db.Dialect), id); err != nil {
		return false, err
	}
	result, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`DELETE FROM "group" WHERE id = ?`, t.db.Dialect), id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *GroupsTable) AddUsersToGroup(ctx context.Context, id string, userIDs []string) (*Group, error) {
	now := time.Now().Unix()
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		query := `INSERT OR IGNORE INTO group_member (id, group_id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
		if t.db.Dialect == dbinternal.DialectPostgres {
			query = `INSERT INTO group_member (id, group_id, user_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (group_id, user_id) DO NOTHING`
		}
		if _, err := t.db.DB.ExecContext(ctx, query, uuid.NewString(), id, userID, now, now); err != nil {
			return nil, err
		}
	}
	_, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`UPDATE "group" SET updated_at = ? WHERE id = ?`, t.db.Dialect), now, id)
	if err != nil {
		return nil, err
	}
	return t.GetGroupByID(ctx, id)
}

func (t *GroupsTable) RemoveUsersFromGroup(ctx context.Context, id string, userIDs []string) (*Group, error) {
	if len(userIDs) > 0 {
		query := `DELETE FROM group_member WHERE group_id = ? AND user_id IN (` + placeholders(len(userIDs), t.db.Dialect, 2) + `)`
		args := []any{id}
		for _, userID := range userIDs {
			args = append(args, userID)
		}
		if _, err := t.db.DB.ExecContext(ctx, query, args...); err != nil {
			return nil, err
		}
	}
	_, err := t.db.DB.ExecContext(ctx, rebindPlaceholders(`UPDATE "group" SET updated_at = ? WHERE id = ?`, t.db.Dialect), time.Now().Unix(), id)
	if err != nil {
		return nil, err
	}
	return t.GetGroupByID(ctx, id)
}

func (t *GroupsTable) GetGroupMemberCountByID(ctx context.Context, id string) (int, error) {
	row := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(`SELECT COUNT(*) FROM group_member WHERE group_id = ?`, t.db.Dialect), id)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (t *GroupsTable) GetGroupUserIDsByID(ctx context.Context, id string) ([]string, error) {
	rows, err := t.db.DB.QueryContext(ctx, rebindPlaceholders(`SELECT user_id FROM group_member WHERE group_id = ? ORDER BY created_at ASC`, t.db.Dialect), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		ids = append(ids, userID)
	}
	return ids, rows.Err()
}

func scanGroup(rows *sql.Rows) (*Group, error) {
	var group Group
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	var permissionsRaw sql.NullString
	if err := rows.Scan(&group.ID, &group.UserID, &group.Name, &group.Description, &dataRaw, &metaRaw, &permissionsRaw, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return nil, err
	}
	var err error
	group.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	group.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	group.Permissions, err = unmarshalJSONMap(permissionsRaw)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func scanGroupRow(row *sql.Row) (*Group, error) {
	var group Group
	var dataRaw sql.NullString
	var metaRaw sql.NullString
	var permissionsRaw sql.NullString
	err := row.Scan(&group.ID, &group.UserID, &group.Name, &group.Description, &dataRaw, &metaRaw, &permissionsRaw, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	group.Data, err = unmarshalJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	group.Meta, err = unmarshalJSONMap(metaRaw)
	if err != nil {
		return nil, err
	}
	group.Permissions, err = unmarshalJSONMap(permissionsRaw)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func joinAssignments(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ", "
		}
		result += part
	}
	return result
}

func placeholders(count int, dialect dbinternal.Dialect, start int) string {
	out := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			out += ", "
		}
		if dialect == dbinternal.DialectPostgres {
			out += "$" + itoa(start+i)
		} else {
			out += "?"
		}
	}
	return out
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
