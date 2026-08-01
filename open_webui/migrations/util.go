package migrations

import (
	"context"
	"fmt"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

func GetExistingTables(ctx context.Context, db *dbinternal.Handle) ([]string, error) {
	switch db.Dialect {
	case dbinternal.DialectSQLite:
		rows, err := db.DB.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
		return tables, rows.Err()
	case dbinternal.DialectPostgres:
		query := "SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() ORDER BY table_name"
		args := []any{}
		if db.Schema != "" {
			query = "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name"
			args = append(args, db.Schema)
		}

		rows, err := db.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tables = append(tables, name)
		}
		return tables, rows.Err()
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", db.Dialect)
	}
}

func GetExistingColumns(ctx context.Context, db *dbinternal.Handle, table string) ([]string, error) {
	return db.ExistingColumns(ctx, table)
}

func HasColumn(ctx context.Context, db *dbinternal.Handle, table string, column string) (bool, error) {
	return db.HasColumn(ctx, table, column)
}
