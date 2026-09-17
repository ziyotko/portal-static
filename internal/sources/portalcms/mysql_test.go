package portalcms

import (
	"context"
	"os"
	"strings"
	"testing"

	"portal-static/internal/core/config"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestStoreQualifiesConfiguredSchema(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "tenant_portal", false)
	if err != nil {
		t.Fatal(err)
	}
	got := store.SQL("SELECT * FROM {{schema}}.article JOIN {{schema}}.`column`")
	if got != "SELECT * FROM tenant_portal.article JOIN tenant_portal.`column`" {
		t.Fatalf("qualified SQL = %q", got)
	}
}

func TestStoreRejectsUnsafeSchema(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := NewStore(db, "portal; DROP DATABASE portal", false); err == nil {
		t.Fatal("expected unsafe schema rejection")
	}
}

func TestOpenRejectsDSNSchemaMismatchBeforeConnecting(t *testing.T) {
	const env = "PORTAL_STATIC_TEST_DSN"
	t.Setenv(env, "user:pass@tcp(127.0.0.1:3306)/wrong_schema")
	_, err := Open(config.DatabaseConfig{
		DSNEnv: env, Schema: "expected_schema", MaxOpenConns: 1, MaxIdleConns: 0, ConnMaxLifetime: "1m",
	}, "Asia/Shanghai")
	if err == nil || !strings.Contains(err.Error(), "wrong_schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenRequiresConfiguredEnvironmentVariable(t *testing.T) {
	const env = "PORTAL_STATIC_MISSING_DSN"
	_ = os.Unsetenv(env)
	_, err := Open(config.DatabaseConfig{
		DSNEnv: env, Schema: "portal", MaxOpenConns: 1, MaxIdleConns: 0, ConnMaxLifetime: "1m",
	}, "Asia/Shanghai")
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreDelegatesQueries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT 1 FROM portal\\.page").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(1))
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.QueryContext(context.Background(), "SELECT 1 FROM {{schema}}.page")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
