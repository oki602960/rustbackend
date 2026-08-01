package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V922E7A387820Revision     = "922e7a387820"
	V922E7A387820DownRevision = "4ace53fd72c8"
)

func UpgradeV922E7A387820AddGroupTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "group")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE "group" (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				name TEXT,
				description TEXT,
				data JSON,
				meta JSON,
				permissions JSON,
				created_at BIGINT,
				updated_at BIGINT
			)
		`); err != nil {
			return err
		}
	}

	type alterSpec struct {
		table  string
		column string
		ddl    string
	}

	specs := []alterSpec{
		{table: "model", column: "access_control", ddl: `ALTER TABLE model ADD COLUMN access_control JSON`},
		{table: "model", column: "is_active", ddl: `ALTER TABLE model ADD COLUMN is_active BOOLEAN DEFAULT 1 NOT NULL`},
		{table: "prompt", column: "access_control", ddl: `ALTER TABLE prompt ADD COLUMN access_control JSON`},
		{table: "tool", column: "access_control", ddl: `ALTER TABLE tool ADD COLUMN access_control JSON`},
		{table: "knowledge", column: "access_control", ddl: `ALTER TABLE knowledge ADD COLUMN access_control JSON`},
	}

	for _, spec := range specs {
		tableExists, err := db.TableExists(ctx, spec.table)
		if err != nil {
			return err
		}
		if !tableExists {
			continue
		}
		hasColumn, err := db.HasColumn(ctx, spec.table, spec.column)
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
