package db

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}

	return m, nil
}

func MigrateUp(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		if isDirty(err) {
			version, dirty, verr := m.Version()
			if verr == nil && dirty {
				if ferr := m.Force(int(version)); ferr != nil {
					return fmt.Errorf("migrate repair: %w", ferr)
				}
				if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
					return fmt.Errorf("migrate up após repair: %w", err)
				}
				return nil
			}
		}
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

func MigrateDown(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}

	return nil
}

func MigrateRepair(databaseURL string) (int, bool, error) {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}

	if !dirty {
		return int(version), false, nil
	}

	if err := m.Force(int(version)); err != nil {
		return 0, true, fmt.Errorf("migrate force: %w", err)
	}

	return int(version), true, nil
}

func isDirty(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Dirty database")
}
