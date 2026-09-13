package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("sqlite DSN must not be empty")
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection keeps PRAGMA state deterministic and avoids multiple
	// isolated databases when tests use :memory:.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration: %w", err)
	}
	defer tx.Rollback()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id TEXT PRIMARY KEY,
			protocol TEXT NOT NULL,
			base_url TEXT NOT NULL,
			enabled INTEGER NOT NULL CHECK (enabled IN (0, 1))
		)`,
		`CREATE TABLE IF NOT EXISTS protocol_defaults (
			protocol TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS model_routes (
			model TEXT NOT NULL,
			position INTEGER NOT NULL CHECK (position >= 0),
			provider_id TEXT NOT NULL,
			upstream_model TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (model, position),
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS model_routes_provider_idx ON model_routes(provider_id)`,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite migration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration: %w", err)
	}
	return nil
}

func (s *Store) Load(ctx context.Context) (domaincatalog.Snapshot, error) {
	snapshot := domaincatalog.Snapshot{
		Defaults: make(map[provider.Protocol]string),
		Routes:   make(map[string][]domaincatalog.RouteTarget),
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, protocol, base_url, enabled FROM providers ORDER BY id`)
	if err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("load providers: %w", err)
	}
	for rows.Next() {
		var item domaincatalog.Provider
		var protocolName string
		var enabled int
		if err := rows.Scan(&item.ID, &protocolName, &item.BaseURL, &enabled); err != nil {
			rows.Close()
			return domaincatalog.Snapshot{}, fmt.Errorf("scan provider: %w", err)
		}
		item.Protocol = provider.Protocol(protocolName)
		item.Enabled = enabled != 0
		snapshot.Providers = append(snapshot.Providers, item)
	}
	if err := rows.Close(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("close provider rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("iterate providers: %w", err)
	}

	defaultRows, err := s.db.QueryContext(ctx, `SELECT protocol, provider_id FROM protocol_defaults ORDER BY protocol`)
	if err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("load protocol defaults: %w", err)
	}
	for defaultRows.Next() {
		var protocolName string
		var providerID string
		if err := defaultRows.Scan(&protocolName, &providerID); err != nil {
			defaultRows.Close()
			return domaincatalog.Snapshot{}, fmt.Errorf("scan protocol default: %w", err)
		}
		snapshot.Defaults[provider.Protocol(protocolName)] = providerID
	}
	if err := defaultRows.Close(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("close protocol default rows: %w", err)
	}
	if err := defaultRows.Err(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("iterate protocol defaults: %w", err)
	}

	routeRows, err := s.db.QueryContext(ctx, `SELECT model, provider_id, upstream_model FROM model_routes ORDER BY model, position`)
	if err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("load model routes: %w", err)
	}
	for routeRows.Next() {
		var model string
		var target domaincatalog.RouteTarget
		if err := routeRows.Scan(&model, &target.ProviderID, &target.Model); err != nil {
			routeRows.Close()
			return domaincatalog.Snapshot{}, fmt.Errorf("scan model route: %w", err)
		}
		snapshot.Routes[model] = append(snapshot.Routes[model], target)
	}
	if err := routeRows.Close(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("close model route rows: %w", err)
	}
	if err := routeRows.Err(); err != nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("iterate model routes: %w", err)
	}
	if err := snapshot.Validate(); err != nil && (len(snapshot.Providers) != 0 || len(snapshot.Defaults) != 0 || len(snapshot.Routes) != 0) {
		return domaincatalog.Snapshot{}, fmt.Errorf("validate persisted catalog: %w", err)
	}
	return snapshot, nil
}

func (s *Store) Replace(ctx context.Context, snapshot domaincatalog.Snapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("validate catalog snapshot: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog replace: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM model_routes`); err != nil {
		return fmt.Errorf("clear model routes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM protocol_defaults`); err != nil {
		return fmt.Errorf("clear protocol defaults: %w", err)
	}
	providerStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO providers(id, protocol, base_url, enabled) VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			protocol = excluded.protocol,
			base_url = excluded.base_url,
			enabled = excluded.enabled
	`)
	if err != nil {
		return fmt.Errorf("prepare provider upsert: %w", err)
	}
	defer providerStmt.Close()
	providerIDs := make([]any, 0, len(snapshot.Providers))
	for _, item := range snapshot.Providers {
		enabled := 0
		if item.Enabled {
			enabled = 1
		}
		if _, err := providerStmt.ExecContext(ctx, item.ID, string(item.Protocol), item.BaseURL, enabled); err != nil {
			return fmt.Errorf("upsert provider %q: %w", item.ID, err)
		}
		providerIDs = append(providerIDs, item.ID)
	}
	if len(providerIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM providers`); err != nil {
			return fmt.Errorf("delete stale providers: %w", err)
		}
	} else {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(providerIDs)), ",")
		if _, err := tx.ExecContext(ctx, `DELETE FROM providers WHERE id NOT IN (`+placeholders+`)`, providerIDs...); err != nil {
			return fmt.Errorf("delete stale providers: %w", err)
		}
	}
	defaultStmt, err := tx.PrepareContext(ctx, `INSERT INTO protocol_defaults(protocol, provider_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare protocol default insert: %w", err)
	}
	defer defaultStmt.Close()
	for protocolName, providerID := range snapshot.Defaults {
		if _, err := defaultStmt.ExecContext(ctx, string(protocolName), providerID); err != nil {
			return fmt.Errorf("insert default %q: %w", protocolName, err)
		}
	}
	routeStmt, err := tx.PrepareContext(ctx, `INSERT INTO model_routes(model, position, provider_id, upstream_model) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare route insert: %w", err)
	}
	defer routeStmt.Close()
	for model, targets := range snapshot.Routes {
		for position, target := range targets {
			if _, err := routeStmt.ExecContext(ctx, model, position, target.ProviderID, target.Model); err != nil {
				return fmt.Errorf("insert route %q position %d: %w", model, position, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog replace: %w", err)
	}
	return nil
}
