package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V38D63C18F30FRevision     = "38d63c18f30f"
	V38D63C18F30FDownRevision = "3af16a1c9fb6"
)

func UpgradeV38D63C18F30FAddOAuthSessionTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "oauth_session")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if _, err := db.DB.ExecContext(ctx, `
		CREATE TABLE oauth_session (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			token TEXT NOT NULL,
			expires_at BIGINT NOT NULL,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)
	`); err != nil {
		return err
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_oauth_session_user_id ON oauth_session (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_session_expires_at ON oauth_session (expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_session_user_provider ON oauth_session (user_id, provider)`,
	}
	for _, stmt := range indexes {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
