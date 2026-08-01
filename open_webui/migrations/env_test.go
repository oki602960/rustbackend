package migrations

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
)

func TestRunCreatesConfigTable(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	db, err := dbinternal.Open(context.Background(), dbinternal.Options{
		DatabaseURL:     "sqlite:///" + filepath.Join(tempDir, "migrations.db"),
		EnableSQLiteWAL: true,
		OpenTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	exists, err := db.TableExists(context.Background(), "config")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected config table to exist after migrations")
	}

	for _, table := range []string{"auth", "chat", "file", "user"} {
		exists, err := db.TableExists(context.Background(), table)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("expected %s table to exist after migrations", table)
		}
	}

	for _, column := range []string{
		"username",
		"bio",
		"gender",
		"date_of_birth",
		"profile_banner_image_url",
		"timezone",
		"presence_state",
		"status_emoji",
		"status_message",
		"status_expires_at",
		"oauth",
		"scim",
	} {
		exists, err := HasColumn(context.Background(), db, "user", column)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("expected user.%s to exist after migrations", column)
		}
	}

	exists, err = db.TableExists(context.Background(), "api_key")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected api_key table to exist after migrations")
	}
}
