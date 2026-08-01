package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V37F288994C47Revision     = "37f288994c47"
	V37F288994C47DownRevision = "a5c220713937"
)

func UpgradeV37F288994C47AddGroupMemberTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "group_member")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = db.DB.ExecContext(ctx, `
		CREATE TABLE group_member (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at BIGINT,
			updated_at BIGINT,
			UNIQUE(group_id, user_id)
		)
	`)
	return err
}
