package versions

import "context"

import dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"

const (
	VAF906E964978Revision     = "af906e964978"
	VAF906E964978DownRevision = "c29facfe716b"
)

func UpgradeVAF906E964978AddFeedbackTable(ctx context.Context, db *dbinternal.Handle) error {
	exists, err := db.TableExists(ctx, "feedback")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.DB.ExecContext(ctx, `
		CREATE TABLE feedback (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			version BIGINT,
			type TEXT,
			data JSON,
			meta JSON,
			snapshot JSON,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)
	`)
	return err
}
