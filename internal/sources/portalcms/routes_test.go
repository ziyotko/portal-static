package portalcms

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRoutePublisherPublishesConfiguredAliases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeFixture := func(relative, content string) string {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	writeFixture("about.html", "about")
	article := writeFixture(filepath.Join("article", "2026", "09", "42.html"), "article")
	listDirectory := filepath.Join(root, "list", "7")
	writeFixture(filepath.Join("list", "7", "1.html"), "list")

	publisher := RoutePublisher{
		Store:       store,
		AllowedRoot: root,
		Routes:      map[string]string{"about": "/company", "article": "/detail", "list": "/page"},
	}
	if err := publisher.PublishMain(context.Background(), "about", "about.html"); err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishArticle(context.Background(), 42, article); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(route_path,'') FROM portal.`column` WHERE id=? AND status=1")).
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"route_path"}).AddRow("/news"))
	if err := publisher.PublishList(context.Background(), 7, listDirectory); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(root, "company.html"):      "about",
		filepath.Join(root, "detail", "42.html"): "article",
		filepath.Join(root, "news", "page.html"): "list",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(data) != want {
			t.Errorf("%s=%q want %q", path, data, want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRoutePublisherRejectsDuplicateListRoutes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewStore(db, "portal", false)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,COALESCE(route_path,'') FROM portal.`column` WHERE status=1 ORDER BY id")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "route_path"}).AddRow(1, "/news").AddRow(2, "/news"))
	publisher := RoutePublisher{Store: store, AllowedRoot: root, Routes: map[string]string{"list": "/page"}}
	if err := publisher.PublishAllLists(context.Background()); err == nil {
		t.Fatal("expected duplicate route error")
	}
}

func TestRoutePublisherRejectsRouteTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "about.html"), []byte("about"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher := RoutePublisher{AllowedRoot: root, Routes: map[string]string{"about": "/../escape"}}
	if err := publisher.PublishMain(context.Background(), "about", "about.html"); err == nil {
		t.Fatal("expected invalid route error")
	}
}
