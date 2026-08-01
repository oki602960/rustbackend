package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	V7826AB40B532Revision     = "7826ab40b532"
	V7826AB40B532DownRevision = "57c599a3cb57"
)

func UpgradeV7826AB40B532UpdateFileTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "file")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	hasColumn, err := db.HasColumn(ctx, "file", "access_control")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `ALTER TABLE file ADD COLUMN access_control JSON`)
	return err
}
