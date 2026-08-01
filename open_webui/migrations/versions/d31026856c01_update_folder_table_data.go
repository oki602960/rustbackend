package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VD31026856C01Revision     = "d31026856c01"
	VD31026856C01DownRevision = "9f0c9cd09105"
)

func UpgradeVD31026856C01UpdateFolderTableData(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "folder")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	hasColumn, err := db.HasColumn(ctx, "folder", "data")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `ALTER TABLE folder ADD COLUMN data JSON`)
	return err
}
