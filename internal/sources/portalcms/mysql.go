package portalcms

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"portal-static/internal/core/config"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const SchemaPlaceholder = "{{schema}}."

// Store owns a Portal CMS MySQL connection and applies the configured schema
// to every query containing SchemaPlaceholder. Adapters keep their own result
// models while sharing connection, schema and lifecycle rules.
type Store struct {
	db        *sql.DB
	schemaRef string
	close     bool
}

func Open(database config.DatabaseConfig, timezone string) (*Store, error) {
	if err := database.ValidateMySQL(); err != nil {
		return nil, err
	}
	dsn := strings.TrimSpace(os.Getenv(database.DSNEnv))
	if dsn == "" {
		return nil, fmt.Errorf("database DSN environment variable %q is empty", database.DSNEnv)
	}
	mysqlConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	if mysqlConfig.DBName != "" && mysqlConfig.DBName != database.Schema {
		return nil, fmt.Errorf("database DSN selects schema %q but database.schema is %q", mysqlConfig.DBName, database.Schema)
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load database timezone: %w", err)
	}
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = location
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(database.MaxOpenConns)
	db.SetMaxIdleConns(database.MaxIdleConns)
	lifetime, _ := time.ParseDuration(database.ConnMaxLifetime)
	db.SetConnMaxLifetime(lifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return NewStore(db, database.Schema, true)
}

// NewStore wraps an existing connection. It is useful to adapters and tests;
// closeDB controls whether Close also owns the supplied connection.
func NewStore(db *sql.DB, schema string, closeDB bool) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("portal CMS database is nil")
	}
	if schema != "" {
		candidate := config.DatabaseConfig{DSNEnv: "IGNORED", Schema: schema, MaxOpenConns: 1, ConnMaxLifetime: "1m"}
		if err := candidate.ValidateMySQL(); err != nil {
			return nil, err
		}
	}
	ref := ""
	if schema != "" {
		ref = schema + "."
	}
	return &Store{db: db, schemaRef: ref, close: closeDB}, nil
}

func (s *Store) SQL(query string) string {
	return strings.ReplaceAll(query, SchemaPlaceholder, s.schemaRef)
}

func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.SQL(query), args...)
}

func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.SQL(query), args...)
}

func (s *Store) Close() error {
	if s == nil || !s.close {
		return nil
	}
	return s.db.Close()
}
