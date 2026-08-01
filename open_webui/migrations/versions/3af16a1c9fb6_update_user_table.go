package versions

import (
	"context"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V3AF16A1C9FB6Revision     = "3af16a1c9fb6"
	V3AF16A1C9FB6DownRevision = "018012973d35"
)

func UpgradeV3AF16A1C9FB6UpdateUserTable(ctx context.Context, db *dbinternal.Handle) error {
	type columnDef struct {
		name string
		ddl  string
	}

	defs := []columnDef{
		{name: "username", ddl: `ALTER TABLE "user" ADD COLUMN username TEXT`},
		{name: "bio", ddl: `ALTER TABLE "user" ADD COLUMN bio TEXT`},
		{name: "gender", ddl: `ALTER TABLE "user" ADD COLUMN gender TEXT`},
		{name: "date_of_birth", ddl: `ALTER TABLE "user" ADD COLUMN date_of_birth TEXT`},
	}

	for _, def := range defs {
		exists, err := db.HasColumn(ctx, "user", def.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.DB.ExecContext(ctx, def.ddl); err != nil {
			return err
		}
	}

	return nil
}
