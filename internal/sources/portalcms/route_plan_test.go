package portalcms

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestValidateRoutePlanRejectsMainTemplateCollision(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	records := map[string]BoundTemplate{
		"home":    {RoutePath: "/same"},
		"about":   {RoutePath: "/same.html"},
		"list":    {RoutePath: "/list"},
		"article": {RoutePath: "/detail"},
	}
	if err := ValidateRoutePlan(context.Background(), store, records); err == nil || !strings.Contains(err.Error(), "static route conflict") {
		t.Fatalf("expected route conflict, got %v", err)
	}
}

func TestValidateRoutePlanChecksColumnsArticlesAndTopics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,COALESCE(route_path,'') FROM portal.`column` WHERE status=1 ORDER BY id")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "route_path"}).AddRow(7, "/news"))
	mock.ExpectQuery(`(?s)SELECT DISTINCT a\.id.*article_column_publish.*c\.template_id=acp\.template_id.*a\.status=1.*a\.audit_status=2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,COALESCE(route_path,'') FROM portal.template WHERE type='special' AND status=1 ORDER BY id")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "route_path"}).AddRow(88, "/topic"))
	records := map[string]BoundTemplate{
		"home":    {RoutePath: "/"},
		"list":    {RoutePath: "/page"},
		"article": {RoutePath: "/detail"},
	}
	if err := ValidateRoutePlan(context.Background(), store, records); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
