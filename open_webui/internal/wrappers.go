package internal

import (
	"fmt"
	"strings"
)

type ConnectionSpec struct {
	DriverName string
	DSN        string
	Dialect    Dialect
}

func ResolveConnection(databaseURL string) (ConnectionSpec, error) {
	normalized := strings.TrimSpace(databaseURL)
	if strings.HasPrefix(normalized, "postgres://") {
		normalized = "postgresql://" + strings.TrimPrefix(normalized, "postgres://")
	}

	switch {
	case strings.HasPrefix(normalized, "sqlite+sqlcipher://"):
		return ConnectionSpec{}, fmt.Errorf("unsupported database url: %s", normalized)
	case strings.HasPrefix(normalized, "sqlite:///"):
		return ConnectionSpec{
			DriverName: "sqlite",
			DSN:        strings.TrimPrefix(normalized, "sqlite:///"),
			Dialect:    DialectSQLite,
		}, nil
	case strings.HasPrefix(normalized, "sqlite://"):
		return ConnectionSpec{
			DriverName: "sqlite",
			DSN:        strings.TrimPrefix(normalized, "sqlite://"),
			Dialect:    DialectSQLite,
		}, nil
	case strings.HasPrefix(normalized, "postgresql://"):
		return ConnectionSpec{
			DriverName: "pgx",
			DSN:        normalized,
			Dialect:    DialectPostgres,
		}, nil
	default:
		return ConnectionSpec{}, fmt.Errorf("unsupported database url: %s", normalized)
	}
}
