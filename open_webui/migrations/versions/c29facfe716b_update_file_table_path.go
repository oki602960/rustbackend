package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VC29FACFE716BRevision     = "c29facfe716b"
	VC29FACFE716BDownRevision = "c69f45358db4"
)

func UpgradeVC29FACFE716BUpdateFileTablePath(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "file")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	hasColumn, err := db.HasColumn(ctx, "file", "path")
	if err != nil {
		return err
	}
	if hasColumn {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `ALTER TABLE file ADD COLUMN path TEXT`)
	return err
}
