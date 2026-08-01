package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	B2C3D4E5F6A7Revision     = "b2c3d4e5f6a7"
	B2C3D4E5F6A7DownRevision = "a1b2c3d4e5f6"
)

func UpgradeB2C3D4E5F6A7AddScimColumnToUserTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.HasColumn(ctx, "user", "scim")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `ALTER TABLE "user" ADD COLUMN scim JSON`)
	return err
}
