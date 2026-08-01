package internal

import "testing"

func TestResolveConnectionSQLite(t *testing.T) {
	t.Parallel()

	spec, err := ResolveConnection("sqlite:////tmp/openwebui.db")
	if err != nil {
		t.Fatal(err)
	}
	if spec.DriverName != "sqlite" {
		t.Fatalf("unexpected driver: %s", spec.DriverName)
	}
	if spec.Dialect != DialectSQLite {
		t.Fatalf("unexpected dialect: %s", spec.Dialect)
	}
}

func TestResolveConnectionPostgres(t *testing.T) {
	t.Parallel()

	spec, err := ResolveConnection("postgres://user:pass@localhost:5432/openwebui")
	if err != nil {
		t.Fatal(err)
	}
	if spec.DriverName != "pgx" {
		t.Fatalf("unexpected driver: %s", spec.DriverName)
	}
	if spec.Dialect != DialectPostgres {
		t.Fatalf("unexpected dialect: %s", spec.Dialect)
	}
	if spec.DSN != "postgresql://user:pass@localhost:5432/openwebui" {
		t.Fatalf("unexpected dsn: %s", spec.DSN)
	}
}
