package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	V374D2F66AF06Revision     = "374d2f66af06"
	V374D2F66AF06DownRevision = "90ef40d4714e"
)

func UpgradeV374D2F66AF06AddPromptHistoryTable(ctx context.Context, db *dbinternal.Handle) error {
	promptExists, err := db.TableExists(ctx, "prompt")
	if err != nil {
		return err
	}
	if promptExists {
		type alterSpec struct {
			column string
			ddl    string
		}
		specs := []alterSpec{
			{column: "id", ddl: `ALTER TABLE prompt ADD COLUMN id TEXT`},
			{column: "name", ddl: `ALTER TABLE prompt ADD COLUMN name TEXT`},
			{column: "data", ddl: `ALTER TABLE prompt ADD COLUMN data JSON`},
			{column: "meta", ddl: `ALTER TABLE prompt ADD COLUMN meta JSON`},
			{column: "tags", ddl: `ALTER TABLE prompt ADD COLUMN tags JSON`},
			{column: "is_active", ddl: `ALTER TABLE prompt ADD COLUMN is_active BOOLEAN DEFAULT 1`},
			{column: "version_id", ddl: `ALTER TABLE prompt ADD COLUMN version_id TEXT`},
			{column: "created_at", ddl: `ALTER TABLE prompt ADD COLUMN created_at BIGINT`},
			{column: "updated_at", ddl: `ALTER TABLE prompt ADD COLUMN updated_at BIGINT`},
		}
		for _, spec := range specs {
			hasColumn, err := db.HasColumn(ctx, "prompt", spec.column)
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
	}

	exists, err := db.TableExists(ctx, "prompt_history")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `
		CREATE TABLE prompt_history (
			id TEXT PRIMARY KEY,
			prompt_id TEXT NOT NULL,
			user_id TEXT,
			snapshot JSON,
			parent_id TEXT,
			commit_message TEXT,
			created_at BIGINT
		)
	`)
	return err
}
