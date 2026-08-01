package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VC0FBF31CA0DBRevision     = "c0fbf31ca0db"
	VC0FBF31CA0DBDownRevision = "ca81bd47c050"
)

func UpgradeVC0FBF31CA0DBUpdateFileTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "file")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	specs := []struct {
		column string
		ddl    string
	}{
		{"hash", `ALTER TABLE file ADD COLUMN hash TEXT`},
		{"data", `ALTER TABLE file ADD COLUMN data JSON`},
		{"updated_at", `ALTER TABLE file ADD COLUMN updated_at BIGINT`},
	}
	for _, spec := range specs {
		hasColumn, err := db.HasColumn(ctx, "file", spec.column)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}
		if _, err := db.DB.ExecContext(ctx, spec.ddl); err != nil {
			return err
		}
	}
	return nil
}
