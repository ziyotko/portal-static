package portalcms

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"portal-static/internal/contracts"
)

func TestTopicServiceGenerateAndDeleteByTemplateID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	root := t.TempDir()
	query := regexp.QuoteMeta("SELECT id,name,COALESCE(code,''),type,COALESCE(route_path,''),status,COALESCE(source_code,''),COALESCE(layout,'') FROM portal.template WHERE id=?")
	row := func() *sqlmock.Rows {
		return sqlmock.NewRows(templateColumns).AddRow(10, "专题", "special", "special", "/topics/launch", 1, `<html>{{.Template.Name}} {{.GeneratedAt}}</html>`, "")
	}
	mock.ExpectQuery(query).WithArgs(int64(10)).WillReturnRows(row())
	service := TopicService{Store: store, AllowedRoot: root}
	result, err := service.Generate(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "topics", "launch.html")
	if result.Output != want || result.GeneratedFiles != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	data, err := os.ReadFile(want)
	if err != nil || !strings.Contains(string(data), "专题") {
		t.Fatalf("generated topic=%q err=%v", data, err)
	}
	mock.ExpectQuery(query).WithArgs(int64(10)).WillReturnRows(row())
	deleted, err := service.Delete(context.Background(), 10)
	if err != nil || !deleted.Deleted || len(deleted.DeletedPaths) != 1 {
		t.Fatalf("delete=%#v err=%v", deleted, err)
	}
	if _, err := os.Stat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("topic survived deletion: %v", err)
	}
}

func TestTopicServiceRejectsUnsafeRouteAndOutputRoot(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	store, _ := NewStore(db, "portal", false)
	root := t.TempDir()
	query := regexp.QuoteMeta("SELECT id,name,COALESCE(code,''),type,COALESCE(route_path,''),status,COALESCE(source_code,''),COALESCE(layout,'') FROM portal.template WHERE id=?")
	mock.ExpectQuery(query).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows(templateColumns).
		AddRow(10, "专题", "special", "special", "/../../escape", 1, `<html></html>`, ""))
	service := TopicService{Store: store, AllowedRoot: root}
	if _, err := service.Generate(context.Background(), 10); !errors.Is(err, contracts.ErrInvalidOutputPath) {
		t.Fatalf("route error=%v", err)
	}

	mock.ExpectQuery(query).WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows(templateColumns).
		AddRow(11, "专题", "special-2", "special", "/safe", 1, `<html></html>`, ""))
	outside := filepath.Join(t.TempDir(), "outside")
	ctx := contracts.WithOptions(context.Background(), outside, false)
	if _, err := service.Generate(ctx, 11); !errors.Is(err, contracts.ErrInvalidOutputPath) {
		t.Fatalf("output error=%v", err)
	}
}

func TestRouteOutputFile(t *testing.T) {
	for input, want := range map[string]string{
		"/": "index.html", "/topic": "topic.html", "/topic/": "topic/index.html", "/topic/index.html": "topic/index.html",
	} {
		got, err := routeOutputFile(input)
		if err != nil || got != want {
			t.Errorf("routeOutputFile(%q)=%q,%v want %q", input, got, err, want)
		}
	}
}
