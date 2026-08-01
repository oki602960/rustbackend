package openwebui

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigStoreSaveAndLoad(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, err := NewConfigStore(context.Background(), RuntimeConfig{
		DataDir:                 tempDir,
		DatabaseURL:             "sqlite:///" + filepath.Join(tempDir, "webui.db"),
		DatabasePoolTimeout:     5 * time.Second,
		DatabaseEnableSQLiteWAL: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = store.db.DB.Exec(`
		CREATE TABLE config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSON NOT NULL,
			version INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"version": 3,
		"ui": map[string]any{
			"theme": "light",
		},
	}
	if err := store.Save(context.Background(), payload); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	value, ok := GetConfigValue(loaded, "ui.theme")
	if !ok {
		t.Fatal("missing config path")
	}
	if value != "light" {
		t.Fatalf("unexpected config value: %v", value)
	}
}
