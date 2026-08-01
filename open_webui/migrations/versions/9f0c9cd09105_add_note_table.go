package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V9F0C9CD09105Revision     = "9f0c9cd09105"
	V9F0C9CD09105DownRevision = "3781e22d8b01"
)

func UpgradeV9F0C9CD09105AddNoteTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "note")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = db.DB.ExecContext(ctx, `
		CREATE TABLE note (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			title TEXT,
			data JSON,
			meta JSON,
			access_control JSON,
			created_at BIGINT,
			updated_at BIGINT
		)
	`)
	return err
}
