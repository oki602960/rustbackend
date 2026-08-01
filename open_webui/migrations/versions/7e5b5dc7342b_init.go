package versions

import (
	"context"
	"fmt"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

const (
	V7E5B5DC7342BRevision     = "7e5b5dc7342b"
	V7E5B5DC7342BDownRevision = ""
)

func UpgradeV7E5B5DC7342BInit(ctx context.Context, db *dbinternal.Handle) error {
	statements, err := initStatements(db.Dialect)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if _, err := db.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func initStatements(dialect dbinternal.Dialect) ([]string, error) {
	switch dialect {
	case dbinternal.DialectSQLite:
		return []string{
			`CREATE TABLE IF NOT EXISTS auth (
				id TEXT PRIMARY KEY,
				email TEXT,
				password TEXT,
				active BOOLEAN
			)`,
			`CREATE TABLE IF NOT EXISTS chat (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				title TEXT,
				chat TEXT,
				created_at BIGINT,
				updated_at BIGINT,
				share_id TEXT UNIQUE,
				archived BOOLEAN
			)`,
			`CREATE TABLE IF NOT EXISTS chatidtag (
				id TEXT PRIMARY KEY,
				tag_name TEXT,
				chat_id TEXT,
				user_id TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS document (
				collection_name TEXT PRIMARY KEY,
				name TEXT UNIQUE,
				title TEXT,
				filename TEXT,
				content TEXT,
				user_id TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS file (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				filename TEXT,
				meta JSON,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS function (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				name TEXT,
				type TEXT,
				content TEXT,
				meta JSON,
				valves JSON,
				is_active BOOLEAN,
				is_global BOOLEAN,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS memory (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				content TEXT,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS model (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				base_model_id TEXT,
				name TEXT,
				params JSON,
				meta JSON,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS prompt (
				command TEXT PRIMARY KEY,
				user_id TEXT,
				title TEXT,
				content TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS tag (
				id TEXT PRIMARY KEY,
				name TEXT,
				user_id TEXT,
				data TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS tool (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				name TEXT,
				content TEXT,
				specs JSON,
				meta JSON,
				valves JSON,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS "user" (
				id TEXT PRIMARY KEY,
				name TEXT,
				email TEXT,
				role TEXT,
				profile_image_url TEXT,
				last_active_at BIGINT,
				updated_at BIGINT,
				created_at BIGINT,
				api_key TEXT UNIQUE,
				settings JSON,
				info JSON,
				oauth_sub TEXT UNIQUE
			)`,
		}, nil
	case dbinternal.DialectPostgres:
		return []string{
			`CREATE TABLE IF NOT EXISTS auth (
				id TEXT PRIMARY KEY,
				email TEXT,
				password TEXT,
				active BOOLEAN
			)`,
			`CREATE TABLE IF NOT EXISTS chat (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				title TEXT,
				chat TEXT,
				created_at BIGINT,
				updated_at BIGINT,
				share_id TEXT UNIQUE,
				archived BOOLEAN
			)`,
			`CREATE TABLE IF NOT EXISTS chatidtag (
				id TEXT PRIMARY KEY,
				tag_name TEXT,
				chat_id TEXT,
				user_id TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS document (
				collection_name TEXT PRIMARY KEY,
				name TEXT UNIQUE,
				title TEXT,
				filename TEXT,
				content TEXT,
				user_id TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS file (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				filename TEXT,
				meta JSON,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS function (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				name TEXT,
				type TEXT,
				content TEXT,
				meta JSON,
				valves JSON,
				is_active BOOLEAN,
				is_global BOOLEAN,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS memory (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				content TEXT,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS model (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				base_model_id TEXT,
				name TEXT,
				params JSON,
				meta JSON,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS prompt (
				command TEXT PRIMARY KEY,
				user_id TEXT,
				title TEXT,
				content TEXT,
				timestamp BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS tag (
				id TEXT PRIMARY KEY,
				name TEXT,
				user_id TEXT,
				data TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS tool (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				name TEXT,
				content TEXT,
				specs JSON,
				meta JSON,
				valves JSON,
				updated_at BIGINT,
				created_at BIGINT
			)`,
			`CREATE TABLE IF NOT EXISTS "user" (
				id TEXT PRIMARY KEY,
				name TEXT,
				email TEXT,
				role TEXT,
				profile_image_url TEXT,
				last_active_at BIGINT,
				updated_at BIGINT,
				created_at BIGINT,
				api_key TEXT UNIQUE,
				settings JSON,
				info JSON,
				oauth_sub TEXT UNIQUE
			)`,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}
