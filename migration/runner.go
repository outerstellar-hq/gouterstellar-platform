// Package migration runs consumer-owned embedded migrations without owning an
// application schema or executable.
package migration

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Runner applies one consumer-owned migration set through golang-migrate.
type Runner struct {
	migrate *migrate.Migrate
}

// New validates an embedded migration directory and combines it with the
// consumer's already-open database driver. This repository never supplies or
// owns a schema.
func New(files fs.FS, path, databaseName string, driver database.Driver) (*Runner, error) {
	if files == nil {
		return nil, errors.New("migration filesystem is required")
	}
	if path == "" || databaseName == "" || driver == nil {
		return nil, errors.New("migration path, database name and driver are required")
	}
	source, err := iofs.New(files, path)
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	migrator, err := migrate.NewWithInstance("iofs", source, databaseName, driver)
	if err != nil {
		_ = source.Close()
		return nil, fmt.Errorf("construct migrator: %w", err)
	}
	return &Runner{migrate: migrator}, nil
}

// Up applies all pending migrations. An already-current schema is success.
func (r *Runner) Up() error {
	if err := r.migrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// Version reports the current version and dirty state.
func (r *Runner) Version() (uint, bool, error) {
	version, dirty, err := r.migrate.Version()
	if err != nil {
		return 0, false, fmt.Errorf("read migration version: %w", err)
	}
	return version, dirty, nil
}

// Close releases source and database resources owned by golang-migrate.
func (r *Runner) Close() error {
	sourceErr, databaseErr := r.migrate.Close()
	return errors.Join(sourceErr, databaseErr)
}
