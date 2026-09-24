package portalcms

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var templateColumns = []string{"id", "name", "code", "type", "route_path", "status", "source_code", "layout"}

func TestLoadBoundTemplatesUsesDirectTemplateBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery(`(?s)SELECT id,name,COALESCE\(code,''\),type.*FROM portal\.template.*ORDER BY id`).
		WillReturnRows(sqlmock.NewRows(templateColumns).
			AddRow(2, "首页", "home", "home", "/", 1, "<html>{{.Title}}</html>", ""))
	records, err := store.LoadBoundTemplates(context.Background(), []TemplateBinding{{Key: "home", Code: "home", Type: "home"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := records["home"]; got.ID != 2 || got.Code != "home" || got.RoutePath != "/" || got.Source == "" {
		t.Fatalf("unexpected binding: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBoundTemplatesReadsOneSnapshotForAllBindings(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery(`(?s)FROM portal\.template.*ORDER BY id`).WillReturnRows(sqlmock.NewRows(templateColumns).
		AddRow(11, "首页", "home", "home", "/", 1, "<html>home</html>", "").
		AddRow(12, "栏目", "list", "column", "/list", 1, "<html>list</html>", ""))
	records, err := store.LoadBoundTemplates(context.Background(), []TemplateBinding{
		{Key: "home", Name: "首页", Type: "home"},
		{Key: "list", Code: "list", Type: "column"},
	})
	if err != nil || len(records) != 2 || records["home"].ID != 11 || records["list"].ID != 12 {
		t.Fatalf("records=%#v err=%v", records, err)
	}
}

func TestLoadBoundTemplatesRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name   string
		rows   *sqlmock.Rows
		bind   TemplateBinding
		needle string
	}{
		{"disabled", sqlmock.NewRows(templateColumns).AddRow(9, "详情", "detail", "detail", "/article", 0, "<html></html>", ""), TemplateBinding{Key: "article", Code: "detail", Type: "detail"}, "disabled"},
		{"empty", sqlmock.NewRows(templateColumns).AddRow(9, "详情", "detail", "detail", "/article", 1, "", ""), TemplateBinding{Key: "article", Code: "detail", Type: "detail"}, "empty source_code"},
		{"wrong type", sqlmock.NewRows(templateColumns).AddRow(9, "详情", "detail", "home", "/", 1, "<html></html>", ""), TemplateBinding{Key: "article", Code: "detail", Type: "detail"}, "type is"},
		{"ambiguous name", sqlmock.NewRows(templateColumns).AddRow(9, "详情", "a", "detail", "/a", 1, "<html>A</html>", "").AddRow(10, "详情", "b", "detail", "/b", 1, "<html>B</html>", ""), TemplateBinding{Key: "article", Name: "详情", Type: "detail"}, "ambiguous"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			store, _ := NewStore(db, "portal", false)
			mock.ExpectQuery(`(?s)FROM portal\.template.*ORDER BY id`).WillReturnRows(tc.rows)
			_, err := store.LoadBoundTemplates(context.Background(), []TemplateBinding{tc.bind})
			if err == nil || !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMaterializeTemplatesValidatesWritesAndCleansUp(t *testing.T) {
	paths, cleanup, err := MaterializeTemplates(map[string]BoundTemplate{
		"home": {Key: "home", Name: "首页", Source: "<html>{{.Title}}</html>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := paths["home"]
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "<html>{{.Title}}</html>" {
		t.Fatalf("runtime template = %q, %v", data, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("runtime template survived cleanup: %v", err)
	}
}

func TestMaterializeTemplatesRejectsInvalidSyntax(t *testing.T) {
	_, _, err := MaterializeTemplates(map[string]BoundTemplate{"home": {Key: "home", Name: "首页", Source: "{{if}}"}})
	if err == nil || !strings.Contains(err.Error(), "parse database template") {
		t.Fatalf("unexpected error: %v", err)
	}
}
