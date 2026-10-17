package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrateDSN opens a dedicated connection for migrations so migrate.Close
// does not tear down the caller's GORM pool.
func MigrateDSN(dsn string) error {
	driver, conn, err := ParseDSN(dsn)
	if err != nil {
		return err
	}
	if conn == ":memory:" {
		conn = "file:linkgear_mem?mode=memory&cache=shared"
	}
	var sqlDB *sql.DB
	switch driver {
	case "sqlite":
		// Same path/name as GORM so shared in-memory DBs see the schema.
		sqlDB, err = sql.Open("sqlite3", conn)
	case "postgres", "postgresql":
		sqlDB, err = sql.Open("postgres", conn)
		driver = "postgres"
	default:
		return fmt.Errorf("unsupported migrate driver %q", driver)
	}
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	return migrateUp(sqlDB, driver)
}

// MigrateUp runs migrations on an existing *sql.DB.
// Warning: migrate.Close closes this DB — prefer MigrateDSN.
func MigrateUp(sqlDB *sql.DB, driver string) error {
	return migrateUp(sqlDB, driver)
}

func migrateUp(sqlDB *sql.DB, driver string) error {
	src, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	var m *migrate.Migrate
	switch strings.ToLower(driver) {
	case "sqlite":
		dbDriver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
		if err != nil {
			return err
		}
		m, err = migrate.NewWithInstance("iofs", src, "sqlite3", dbDriver)
		if err != nil {
			return err
		}
	case "postgres", "postgresql":
		dbDriver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
		if err != nil {
			return err
		}
		m, err = migrate.NewWithInstance("iofs", src, "postgres", dbDriver)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported migrate driver %q", driver)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
