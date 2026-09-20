package portalcms

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadBoundTemplatesUsesPageBindingAndActiveTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE(t.source_code,'')") + ".*FROM portal\\.page p.*ORDER BY p\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"page_id", "page_name", "page_type", "template_id", "template_name", "template_type", "source_code"}).
			AddRow(3, "首页", "home", 2, "首页", "home", "<html>{{.Title}}</html>"))

	records, err := store.LoadBoundTemplates(context.Background(), []TemplateBinding{{
		Key: "home", PageName: "首页", PageType: "home", TemplateType: "home",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := records["home"]; got.PageID != 3 || got.TemplateID != 2 || got.Source == "" {
		t.Fatalf("unexpected binding: %#v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBoundTemplatesReadsOneSnapshotForAllBindings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery("SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE.*FROM portal\\.page p.*ORDER BY p\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"page_id", "page_name", "page_type", "template_id", "template_name", "template_type", "source_code"}).
			AddRow(1, "首页", "home", 11, "首页模板", "home", "<html>home</html>").
			AddRow(2, "通用栏目", "column", 12, "栏目模板", "column", "<html>list</html>"))

	records, err := store.LoadBoundTemplates(context.Background(), []TemplateBinding{
		{Key: "home", PageName: "首页", PageType: "home", TemplateType: "home"},
		{Key: "list", PageType: "column", TemplateType: "column"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records["home"].TemplateID != 11 || records["list"].TemplateID != 12 {
		t.Fatalf("unexpected records: %#v", records)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBoundTemplatesRejectsEmptySource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery("SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE.*FROM portal\\.page p.*ORDER BY p\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"page_id", "page_name", "page_type", "template_id", "template_name", "template_type", "source_code"}).
			AddRow(2, "通用详情", "detail", 9, "详情", "detail", ""))
	_, err = store.LoadBoundTemplates(context.Background(), []TemplateBinding{{Key: "article", PageType: "detail", TemplateType: "detail"}})
	if err == nil || !strings.Contains(err.Error(), "empty source_code") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadBoundTemplatesRejectsWrongTemplateType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery("SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE.*FROM portal\\.page p.*ORDER BY p\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"page_id", "page_name", "page_type", "template_id", "template_name", "template_type", "source_code"}).
			AddRow(2, "通用栏目", "column", 9, "栏目", "home", "<html></html>"))
	_, err = store.LoadBoundTemplates(context.Background(), []TemplateBinding{{Key: "list", PageType: "column", TemplateType: "column"}})
	if err == nil || !strings.Contains(err.Error(), `type is "home", want "column"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadBoundTemplatesRejectsAmbiguousPageType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	mock.ExpectQuery("SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE.*FROM portal\\.page p.*ORDER BY p\\.id").
		WillReturnRows(sqlmock.NewRows([]string{"page_id", "page_name", "page_type", "template_id", "template_name", "template_type", "source_code"}).
			AddRow(2, "通用详情 A", "detail", 9, "详情 A", "detail", "<html>A</html>").
			AddRow(3, "通用详情 B", "detail", 10, "详情 B", "detail", "<html>B</html>"))
	_, err = store.LoadBoundTemplates(context.Background(), []TemplateBinding{{Key: "article", PageType: "detail", TemplateType: "detail"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaterializeTemplatesValidatesWritesAndCleansUp(t *testing.T) {
	paths, cleanup, err := MaterializeTemplates(map[string]BoundTemplate{
		"home": {Key: "home", TemplateName: "首页", Source: "<html>{{.Title}}</html>"},
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
	_, _, err := MaterializeTemplates(map[string]BoundTemplate{
		"home": {Key: "home", TemplateName: "首页", Source: "{{if}}"},
	})
	if err == nil || !strings.Contains(err.Error(), "parse database template") {
		t.Fatalf("unexpected error: %v", err)
	}
}
