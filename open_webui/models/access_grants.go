package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

type AccessGrant struct {
	ID            string `json:"id"`
	ResourceType  string `json:"resource_type"`
	ResourceID    string `json:"resource_id"`
	PrincipalType string `json:"principal_type"`
	PrincipalID   string `json:"principal_id"`
	Permission    string `json:"permission"`
	CreatedAt     int64  `json:"created_at"`
}

type AccessGrantsTable struct {
	db *dbinternal.Handle
}

func NewAccessGrantsTable(db *dbinternal.Handle) *AccessGrantsTable {
	return &AccessGrantsTable{db: db}
}

func (t *AccessGrantsTable) SetAccessGrants(ctx context.Context, resourceType string, resourceID string, grants []map[string]any) error {
	if _, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM access_grant WHERE resource_type = ? AND resource_id = ?`, t.db.Dialect),
		resourceType,
		resourceID,
	); err != nil {
		return err
	}
	for _, grant := range normalizeAccessGrants(grants) {
		if _, err := t.db.DB.ExecContext(
			ctx,
			rebindPlaceholders(`INSERT INTO access_grant (id, resource_type, resource_id, principal_type, principal_id, permission, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, t.db.Dialect),
			grant.ID,
			resourceType,
			resourceID,
			grant.PrincipalType,
			grant.PrincipalID,
			grant.Permission,
			grant.CreatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func (t *AccessGrantsTable) GetGrantsByResource(ctx context.Context, resourceType string, resourceID string) ([]AccessGrant, error) {
	rows, err := t.db.DB.QueryContext(
		ctx,
		rebindPlaceholders(`SELECT id, resource_type, resource_id, principal_type, principal_id, permission, created_at FROM access_grant WHERE resource_type = ? AND resource_id = ? ORDER BY created_at ASC`, t.db.Dialect),
		resourceType,
		resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []AccessGrant
	for rows.Next() {
		var grant AccessGrant
		if err := rows.Scan(&grant.ID, &grant.ResourceType, &grant.ResourceID, &grant.PrincipalType, &grant.PrincipalID, &grant.Permission, &grant.CreatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (t *AccessGrantsTable) RevokeAllAccess(ctx context.Context, resourceType string, resourceID string) error {
	_, err := t.db.DB.ExecContext(
		ctx,
		rebindPlaceholders(`DELETE FROM access_grant WHERE resource_type = ? AND resource_id = ?`, t.db.Dialect),
		resourceType,
		resourceID,
	)
	return err
}

func (t *AccessGrantsTable) HasAccess(ctx context.Context, userID string, resourceType string, resourceID string, permission string, userGroupIDs []string) (bool, error) {
	args := []any{resourceType, resourceID, permission, userID}
	query := `SELECT COUNT(*) FROM access_grant WHERE resource_type = ? AND resource_id = ? AND permission = ? AND ((principal_type = 'user' AND (principal_id = ? OR principal_id = '*'))`
	if len(userGroupIDs) > 0 {
		query += ` OR (principal_type = 'group' AND principal_id IN (` + placeholdersForUsers(len(userGroupIDs), t.db.Dialect) + `))`
		for _, id := range userGroupIDs {
			args = append(args, id)
		}
	}
	query += `)`

	var count int
	if err := t.db.DB.QueryRowContext(ctx, rebindPlaceholders(query, t.db.Dialect), args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func normalizeAccessGrants(grants []map[string]any) []AccessGrant {
	result := make([]AccessGrant, 0, len(grants))
	now := time.Now().Unix()
	for _, grant := range grants {
		principalType, _ := grant["principal_type"].(string)
		principalID, _ := grant["principal_id"].(string)
		permission, _ := grant["permission"].(string)
		if principalType == "" || principalID == "" || permission == "" {
			continue
		}
		id, _ := grant["id"].(string)
		if id == "" {
			id = uuid.NewString()
		}
		result = append(result, AccessGrant{
			ID:            id,
			PrincipalType: principalType,
			PrincipalID:   principalID,
			Permission:    permission,
			CreatedAt:     now,
		})
	}
	return result
}

func grantsToJSON(grants []AccessGrant) (string, error) {
	body, err := json.Marshal(grants)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func scanAccessGrantRow(row *sql.Row) (*AccessGrant, error) {
	var grant AccessGrant
	err := row.Scan(&grant.ID, &grant.ResourceType, &grant.ResourceID, &grant.PrincipalType, &grant.PrincipalID, &grant.Permission, &grant.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &grant, nil
}
