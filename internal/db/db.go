package db

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens GORM from a DSN like:
//
//	sqlite://./data/linkgear.db
//	sqlite://:memory:
//	postgres://user:pass@host:5432/db?sslmode=disable
func Open(dsn string) (*gorm.DB, string, error) {
	driver, conn, err := ParseDSN(dsn)
	if err != nil {
		return nil, "", err
	}
	var dialector gorm.Dialector
	switch driver {
	case "sqlite":
		if conn != ":memory:" && !strings.HasPrefix(conn, "file:") {
			dir := filepath.Dir(conn)
			if dir != "." && dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return nil, "", fmt.Errorf("create sqlite dir: %w", err)
				}
			}
		}
		if conn == ":memory:" {
			conn = "file:linkgear_mem?mode=memory&cache=shared"
		}
		dialector = sqlite.Open(conn)
	case "postgres", "postgresql":
		dialector = postgres.Open(conn)
		driver = "postgres"
	default:
		return nil, "", fmt.Errorf("unsupported driver %q", driver)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, "", err
	}
	return gdb, driver, nil
}

// ParseDSN splits scheme and connection string.
func ParseDSN(dsn string) (driver, conn string, err error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", "", fmt.Errorf("empty DSN")
	}
	if strings.HasPrefix(dsn, "sqlite://") {
		return "sqlite", strings.TrimPrefix(dsn, "sqlite://"), nil
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if _, err := url.Parse(dsn); err != nil {
			return "", "", err
		}
		return "postgres", dsn, nil
	}
	if !strings.Contains(dsn, "://") {
		return "sqlite", dsn, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", err
	}
	return u.Scheme, dsn, nil
}
