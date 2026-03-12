package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	mgcrdb "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go/modules/cockroachdb"
)

// setupTestDB spawns a CockroachDB container and applies migrations.
// It returns the *sql.DB connection and a teardown function.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    t.Helper()
    ctx := context.Background()

    // 1. Start Container (Uses the Testcontainers package)
    cockroachContainer, err := cockroachdb.Run(ctx, "cockroachdb/cockroach:latest-v23.1")
    if err != nil {
        t.Fatalf("failed to start container: %s", err)
    }

    connStr, err := cockroachContainer.ConnectionString(ctx)
    if err != nil {
        t.Fatalf("failed to get connection string: %s", err)
    }

    // 2. Connect
    db, err := sql.Open("pgx", connStr)
    if err != nil {
        t.Fatalf("failed to open db: %s", err)
    }

    // 3. Apply Migrations
    _, b, _, _ := runtime.Caller(0)
    basepath := filepath.Dir(b)
    migrationsPath := filepath.Join(basepath, "..", "..", "..", "deployments", "db", "migrations")
    absPath, _ := filepath.Abs(migrationsPath)
    migrationSource := fmt.Sprintf("file://%s", filepath.ToSlash(absPath))

    // Use the ALIASED migration driver here: mg_crdb
    driver, err := mgcrdb.WithInstance(db, &mgcrdb.Config{})
    if err != nil {
        t.Fatalf("failed to create migration driver: %s", err)
    }

    m, err := migrate.NewWithDatabaseInstance(migrationSource, "cockroachdb", driver)
    if err != nil {
        t.Fatalf("failed to init migration: %s", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("failed to apply migrations: %s", err)
    }

    teardown := func() {
        _ = db.Close()
        _ = cockroachContainer.Terminate(ctx)
    }

    return db, teardown
}

// truncateTables clears data between tests to ensure isolation.
func truncateTables(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{"audit_logs", "rules", "sources"}
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("failed to truncate table %s: %s", table, err)
		}
	}
}
