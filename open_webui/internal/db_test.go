package internal

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenSQLiteAndTableExists(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	handle, err := Open(context.Background(), Options{
		DatabaseURL:     "sqlite:///" + filepath.Join(tempDir, "test.db"),
		EnableSQLiteWAL: true,
		OpenTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	_, err = handle.DB.ExecContext(context.Background(), "CREATE TABLE config (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatal(err)
	}

	exists, err := handle.TableExists(context.Background(), "config")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected table to exist")
	}
}
