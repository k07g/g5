package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

// Migrate applies every embedded *.up.sql migration, in filename order.
// Each migration is written to be idempotent (CREATE ... IF NOT EXISTS)
// since no migration-tracking table is used; this lets it run safely on
// every application startup, including against environments (like ECS)
// where the database isn't otherwise reachable to apply migrations by hand.
func Migrate(ctx context.Context, conn *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		contents, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			return fmt.Errorf("applying migration %s: %w", name, err)
		}
	}
	return nil
}
