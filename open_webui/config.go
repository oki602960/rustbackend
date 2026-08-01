package openwebui

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
)

var defaultConfigData = map[string]any{
	"version": 0,
	"ui":      map[string]any{},
}

type ConfigRecord struct {
	ID        int64
	Data      map[string]any
	Version   int
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type ConfigStore struct {
	runtime RuntimeConfig
	db      *dbinternal.Handle
	ownsDB  bool
}

func NewConfigStore(ctx context.Context, runtime RuntimeConfig) (*ConfigStore, error) {
	if err := os.MkdirAll(runtime.DataDir, 0o755); err != nil {
		return nil, err
	}

	dbHandle, err := dbinternal.Open(ctx, dbinternal.Options{
		DatabaseURL:     runtime.DatabaseURL,
		DatabaseSchema:  runtime.DatabaseSchema,
		EnableSQLiteWAL: runtime.DatabaseEnableSQLiteWAL,
		PoolSize:        runtime.DatabasePoolSize,
		PoolRecycle:     runtime.DatabasePoolRecycle,
		OpenTimeout:     runtime.DatabasePoolTimeout,
	})
	if err != nil {
		return nil, err
	}

	return newConfigStore(ctx, runtime, dbHandle, true, runtime.EnableDBMigrations)
}

func NewConfigStoreWithHandle(ctx context.Context, runtime RuntimeConfig, dbHandle *dbinternal.Handle) (*ConfigStore, error) {
	if err := os.MkdirAll(runtime.DataDir, 0o755); err != nil {
		return nil, err
	}
	return newConfigStore(ctx, runtime, dbHandle, false, false)
}

func newConfigStore(ctx context.Context, runtime RuntimeConfig, dbHandle *dbinternal.Handle, ownsDB bool, runMigrations bool) (*ConfigStore, error) {
	store := &ConfigStore{
		runtime: runtime,
		db:      dbHandle,
		ownsDB:  ownsDB,
	}

	if runMigrations {
		if err := migrations.Run(ctx, dbHandle); err != nil {
			_ = store.Close()
			return nil, err
		}
	}

	if err := store.migrateLegacyConfigJSON(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}

	return store, nil
}

func (s *ConfigStore) Close() error {
	if s == nil || s.db == nil || !s.ownsDB {
		return nil
	}
	return s.db.Close()
}

func (s *ConfigStore) Load(ctx context.Context) (map[string]any, error) {
	record, err := s.loadRecord(ctx)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Data == nil {
		return cloneConfigData(defaultConfigData), nil
	}
	return cloneConfigData(record.Data), nil
}

func (s *ConfigStore) Save(ctx context.Context, data map[string]any) error {
	payload := cloneConfigData(data)

	hasTable, err := s.hasConfigTable(ctx)
	if err != nil {
		return err
	}
	if !hasTable {
		return writeJSONFile(s.legacyConfigPath(), payload)
	}

	if payload["version"] == nil {
		payload["version"] = 0
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	record, err := s.loadRecord(ctx)
	if err != nil {
		return err
	}

	switch s.db.Dialect {
	case dbinternal.DialectSQLite:
		if record == nil {
			_, err = s.db.DB.ExecContext(ctx, "INSERT INTO config (data, version) VALUES (?, ?)", string(body), payload["version"])
		} else {
			_, err = s.db.DB.ExecContext(ctx, "UPDATE config SET data = ?, version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(body), payload["version"], record.ID)
		}
	case dbinternal.DialectPostgres:
		if record == nil {
			_, err = s.db.DB.ExecContext(ctx, "INSERT INTO config (data, version) VALUES ($1::json, $2)", string(body), payload["version"])
		} else {
			_, err = s.db.DB.ExecContext(ctx, "UPDATE config SET data = $1::json, version = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3", string(body), payload["version"], record.ID)
		}
	default:
		err = errors.New("unsupported database dialect")
	}
	return err
}

func (s *ConfigStore) Reset(ctx context.Context) error {
	hasTable, err := s.hasConfigTable(ctx)
	if err != nil {
		return err
	}
	if hasTable {
		if _, err := s.db.DB.ExecContext(ctx, "DELETE FROM config"); err != nil {
			return err
		}
	}
	if err := os.Remove(s.legacyConfigPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func GetConfigValue(config map[string]any, configPath string) (any, bool) {
	current := any(config)
	for _, part := range strings.Split(configPath, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := node[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func (s *ConfigStore) migrateLegacyConfigJSON(ctx context.Context) error {
	legacyPath := s.legacyConfigPath()
	if _, err := os.Stat(legacyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	hasTable, err := s.hasConfigTable(ctx)
	if err != nil {
		return err
	}
	if !hasTable {
		return nil
	}

	record, err := s.loadRecord(ctx)
	if err != nil {
		return err
	}
	if record != nil {
		return nil
	}

	body, err := os.ReadFile(legacyPath)
	if err != nil {
		return err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if err := s.Save(ctx, payload); err != nil {
		return err
	}

	return os.Rename(legacyPath, s.oldLegacyConfigPath())
}

func (s *ConfigStore) loadRecord(ctx context.Context) (*ConfigRecord, error) {
	hasTable, err := s.hasConfigTable(ctx)
	if err != nil {
		return nil, err
	}
	if !hasTable {
		body, err := os.ReadFile(s.legacyConfigPath())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, err
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		return &ConfigRecord{Data: payload}, nil
	}

	row := s.db.DB.QueryRowContext(ctx, "SELECT id, data, version, created_at, updated_at FROM config ORDER BY id DESC LIMIT 1")
	record := &ConfigRecord{}
	var rawData []byte
	err = row.Scan(&record.ID, &rawData, &record.Version, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if len(rawData) == 0 {
		record.Data = cloneConfigData(defaultConfigData)
		return record, nil
	}
	if err := json.Unmarshal(rawData, &record.Data); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *ConfigStore) hasConfigTable(ctx context.Context) (bool, error) {
	if s.db == nil {
		return false, nil
	}
	return s.db.TableExists(ctx, "config")
}

func (s *ConfigStore) legacyConfigPath() string {
	return filepath.Join(s.runtime.DataDir, "config.json")
}

func (s *ConfigStore) oldLegacyConfigPath() string {
	return filepath.Join(s.runtime.DataDir, "old_config.json")
}

func cloneConfigData(source map[string]any) map[string]any {
	body, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	var target map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return map[string]any{}
	}
	return target
}

func writeJSONFile(path string, payload map[string]any) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
