package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VF1E2D3C4B5A6Revision     = "f1e2d3c4b5a6"
	VF1E2D3C4B5A6DownRevision = "8452d01d26d7"
)

func UpgradeVF1E2D3C4B5A6AddAccessGrantTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "access_grant")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if _, err := db.DB.ExecContext(ctx, `
		CREATE TABLE access_grant (
			id TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			principal_type TEXT NOT NULL,
			principal_id TEXT NOT NULL,
			permission TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			UNIQUE(resource_type, resource_id, principal_type, principal_id, permission)
		)
	`); err != nil {
		return err
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_access_grant_resource ON access_grant (resource_type, resource_id)`,
		`CREATE INDEX IF NOT EXISTS idx_access_grant_principal ON access_grant (principal_type, principal_id)`,
	}
	for _, stmt := range indexes {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
