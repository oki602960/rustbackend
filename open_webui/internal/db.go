package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

type Options struct {
	DatabaseURL     string
	DatabaseSchema  string
	EnableSQLiteWAL bool
	PoolSize        int
	PoolRecycle     time.Duration
	OpenTimeout     time.Duration
}

type Handle struct {
	DB          *sql.DB
	Dialect     Dialect
	DatabaseURL string
	Schema      string
}

func Open(ctx context.Context, opts Options) (*Handle, error) {
	if strings.TrimSpace(opts.DatabaseURL) == "" {
		return nil, errors.New("database url is required")
	}

	spec, err := ResolveConnection(opts.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if spec.Dialect == DialectSQLite {
		if err := os.MkdirAll(filepath.Dir(spec.DSN), 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open(spec.DriverName, spec.DSN)
	if err != nil {
		return nil, err
	}

	if opts.PoolSize > 0 {
		db.SetMaxOpenConns(opts.PoolSize)
		db.SetMaxIdleConns(opts.PoolSize)
	}
	if opts.PoolRecycle > 0 {
		db.SetConnMaxLifetime(opts.PoolRecycle)
	}

	pingCtx := ctx
	if opts.OpenTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, opts.OpenTimeout)
		defer cancel()
	}

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}

	if spec.Dialect == DialectSQLite {
		journalMode := "DELETE"
		if opts.EnableSQLiteWAL {
			journalMode = "WAL"
		}
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode="+journalMode); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return &Handle{
		DB:          db,
		Dialect:     spec.Dialect,
		DatabaseURL: opts.DatabaseURL,
		Schema:      opts.DatabaseSchema,
	}, nil
}

func (h *Handle) Close() error {
	if h == nil || h.DB == nil {
		return nil
	}
	return h.DB.Close()
}

func (h *Handle) TableExists(ctx context.Context, table string) (bool, error) {
	switch h.Dialect {
	case DialectSQLite:
		var exists int
		err := h.DB.QueryRowContext(
			ctx,
			"SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1",
			table,
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	case DialectPostgres:
		schema := h.Schema
		query := "SELECT 1 FROM information_schema.tables WHERE table_name = $1 AND table_schema = current_schema() LIMIT 1"
		args := []any{table}
		if schema != "" {
			query = "SELECT 1 FROM information_schema.tables WHERE table_name = $1 AND table_schema = $2 LIMIT 1"
			args = append(args, schema)
		}
		var exists int
		err := h.DB.QueryRowContext(ctx, query, args...).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported dialect: %s", h.Dialect)
	}
}

func (h *Handle) ExistingColumns(ctx context.Context, table string) ([]string, error) {
	switch h.Dialect {
	case DialectSQLite:
		rows, err := h.DB.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var columns []string
		for rows.Next() {
			var cid int
			var name string
			var dataType string
			var notNull int
			var defaultValue any
			var pk int
			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
				return nil, err
			}
			columns = append(columns, name)
		}
		return columns, rows.Err()
	case DialectPostgres:
		query := `SELECT column_name FROM information_schema.columns WHERE table_name = $1 AND table_schema = current_schema() ORDER BY ordinal_position`
		args := []any{table}
		if h.Schema != "" {
			query = `SELECT column_name FROM information_schema.columns WHERE table_name = $1 AND table_schema = $2 ORDER BY ordinal_position`
			args = append(args, h.Schema)
		}
		rows, err := h.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var columns []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			columns = append(columns, name)
		}
		return columns, rows.Err()
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", h.Dialect)
	}
}

func (h *Handle) HasColumn(ctx context.Context, table string, column string) (bool, error) {
	columns, err := h.ExistingColumns(ctx, table)
	if err != nil {
		return false, err
	}
	for _, existing := range columns {
		if existing == column {
			return true, nil
		}
	}
	return false, nil
}
