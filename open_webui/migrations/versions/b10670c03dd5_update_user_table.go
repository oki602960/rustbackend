package versions

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	B10670C03DD5Revision     = "b10670c03dd5"
	B10670C03DD5DownRevision = "2f1211949ecc"
)

func UpgradeB10670C03DD5UpdateUserTable(ctx context.Context, db *dbinternal.Handle) error {
	type columnDef struct {
		name string
		ddl  string
	}

	defs := []columnDef{
		{name: "profile_banner_image_url", ddl: `ALTER TABLE "user" ADD COLUMN profile_banner_image_url TEXT`},
		{name: "timezone", ddl: `ALTER TABLE "user" ADD COLUMN timezone TEXT`},
		{name: "presence_state", ddl: `ALTER TABLE "user" ADD COLUMN presence_state TEXT`},
		{name: "status_emoji", ddl: `ALTER TABLE "user" ADD COLUMN status_emoji TEXT`},
		{name: "status_message", ddl: `ALTER TABLE "user" ADD COLUMN status_message TEXT`},
		{name: "status_expires_at", ddl: `ALTER TABLE "user" ADD COLUMN status_expires_at BIGINT`},
		{name: "oauth", ddl: `ALTER TABLE "user" ADD COLUMN oauth JSON`},
	}

	for _, def := range defs {
		exists, err := db.HasColumn(ctx, "user", def.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.DB.ExecContext(ctx, def.ddl); err != nil {
			return err
		}
	}

	if exists, err := db.TableExists(ctx, "api_key"); err != nil {
		return err
	} else if !exists {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE api_key (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				key TEXT UNIQUE NOT NULL,
				data JSON,
				expires_at BIGINT,
				last_used_at BIGINT,
				created_at BIGINT NOT NULL,
				updated_at BIGINT NOT NULL
			)
		`); err != nil {
			return err
		}
	}

	hasOAuthSub, err := db.HasColumn(ctx, "user", "oauth_sub")
	if err != nil {
		return err
	}
	if hasOAuthSub {
		rows, err := db.DB.QueryContext(ctx, `SELECT id, oauth_sub FROM "user" WHERE oauth_sub IS NOT NULL`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			var oauthSub sql.NullString
			if err := rows.Scan(&id, &oauthSub); err != nil {
				return err
			}
			if !oauthSub.Valid || strings.TrimSpace(oauthSub.String) == "" {
				continue
			}

			provider := "oidc"
			sub := oauthSub.String
			if strings.Contains(sub, "@") {
				parts := strings.SplitN(sub, "@", 2)
				provider = parts[0]
				sub = parts[1]
			}

			oauthJSON, err := json.Marshal(map[string]any{
				provider: map[string]any{
					"sub": sub,
				},
			})
			if err != nil {
				return err
			}

			if _, err := db.DB.ExecContext(ctx, `UPDATE "user" SET oauth = ? WHERE id = ?`, string(oauthJSON), id); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	hasLegacyAPIKey, err := db.HasColumn(ctx, "user", "api_key")
	if err != nil {
		return err
	}
	if hasLegacyAPIKey {
		rows, err := db.DB.QueryContext(ctx, `SELECT id, api_key FROM "user" WHERE api_key IS NOT NULL`)
		if err != nil {
			return err
		}
		defer rows.Close()

		now := time.Now().Unix()
		for rows.Next() {
			var id string
			var apiKey sql.NullString
			if err := rows.Scan(&id, &apiKey); err != nil {
				return err
			}
			if !apiKey.Valid || strings.TrimSpace(apiKey.String) == "" {
				continue
			}

			_, err := db.DB.ExecContext(
				ctx,
				insertAPIKeyQuery(db.Dialect),
				insertAPIKeyArgs(db.Dialect, "key_"+id, id, apiKey.String, now, now)...,
			)
			if err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}

func insertAPIKeyQuery(dialect dbinternal.Dialect) string {
	if dialect == dbinternal.DialectPostgres {
		return `INSERT INTO api_key (id, user_id, key, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (key) DO NOTHING`
	}
	return `INSERT OR IGNORE INTO api_key (id, user_id, key, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
}

func insertAPIKeyArgs(dialect dbinternal.Dialect, id string, userID string, key string, createdAt int64, updatedAt int64) []any {
	return []any{id, userID, key, createdAt, updatedAt}
}
