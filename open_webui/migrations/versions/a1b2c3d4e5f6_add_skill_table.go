package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VA1B2C3D4E5F6Revision     = "a1b2c3d4e5f6"
	VA1B2C3D4E5F6DownRevision = "f1e2d3c4b5a6"
)

func UpgradeVA1B2C3D4E5F6AddSkillTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "skill")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if _, err := db.DB.ExecContext(ctx, `
		CREATE TABLE skill (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			content TEXT NOT NULL,
			meta JSON,
			is_active BOOLEAN NOT NULL,
			updated_at BIGINT NOT NULL,
			created_at BIGINT NOT NULL
		)
	`); err != nil {
		return err
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_skill_user_id ON skill (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_updated_at ON skill (updated_at)`,
	}
	for _, stmt := range indexes {
		if _, err := db.DB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
