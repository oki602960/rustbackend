package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	VC69F45358DB4Revision     = "c69f45358db4"
	VC69F45358DB4DownRevision = "3ab32c4b8f59"
)

func UpgradeVC69F45358DB4AddFolderTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "folder")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE folder (
				id TEXT PRIMARY KEY,
				parent_id TEXT,
				user_id TEXT NOT NULL,
				name TEXT NOT NULL,
				items JSON,
				meta JSON,
				is_expanded BOOLEAN NOT NULL DEFAULT 0,
				created_at BIGINT NOT NULL,
				updated_at BIGINT NOT NULL
			)
		`); err != nil {
			return err
		}
	}

	chatExists, err := db.TableExists(ctx, "chat")
	if err != nil {
		return err
	}
	if chatExists {
		hasColumn, err := db.HasColumn(ctx, "chat", "folder_id")
		if err != nil {
			return err
		}
		if !hasColumn {
			if _, err := db.DB.ExecContext(ctx, `ALTER TABLE chat ADD COLUMN folder_id TEXT`); err != nil {
				return err
			}
		}
	}

	return nil
}
