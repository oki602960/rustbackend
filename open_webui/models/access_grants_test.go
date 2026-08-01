package models

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
)

func TestAccessGrantsTableLifecycle(t *testing.T) {
	t.Parallel()

	db, err := dbinternal.Open(context.Background(), dbinternal.Options{
		DatabaseURL:     "sqlite:///" + filepath.Join(t.TempDir(), "access_grants.db"),
		EnableSQLiteWAL: true,
		OpenTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	table := NewAccessGrantsTable(db)
	err = table.SetAccessGrants(context.Background(), "model", "model-1", []map[string]any{
		{"principal_type": "user", "principal_id": "user-1", "permission": "read"},
		{"principal_type": "group", "principal_id": "group-1", "permission": "write"},
	})
	if err != nil {
		t.Fatal(err)
	}

	grants, err := table.GetGrantsByResource(context.Background(), "model", "model-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 {
		t.Fatalf("unexpected grants count: %d", len(grants))
	}

	allowed, err := table.HasAccess(context.Background(), "user-1", "model", "model-1", "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected read access")
	}

	allowed, err = table.HasAccess(context.Background(), "user-2", "model", "model-1", "write", []string{"group-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected group write access")
	}

	if err := table.RevokeAllAccess(context.Background(), "model", "model-1"); err != nil {
		t.Fatal(err)
	}
}
