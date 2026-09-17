package repository

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFetchArticleRelatedColumnsIncludesOfflineAndRemovedRelations(t *testing.T) {
	for _, empty := range []bool{false, true} {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatal(err)
		}
		rows := sqlmock.NewRows([]string{"id", "name", "code", "page_id", "page_name", "parent_id", "description", "sort"})
		if !empty {
			rows.AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
				AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20)
		}
		// Deliberately no article join or acp.deleted_at filter: relationships
		// must remain discoverable after unpublishing or deleting the article.
		mock.ExpectQuery("SELECT DISTINCT c.id,c.name,c.code,c.page_id,p.name,c.parent_id,COALESCE(c.description,''),c.sort FROM article_column_publish acp INNER JOIN `column` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL WHERE acp.article_id=? ORDER BY c.id").
			WithArgs(int64(1001)).WillReturnRows(rows)
		columns, err := New(db, 1).FetchArticleRelatedColumns(context.Background(), 1001)
		if err != nil {
			t.Fatal(err)
		}
		if empty && len(columns) != 0 || !empty && (len(columns) != 2 || columns[1].PageName != "关于我们") {
			t.Fatalf("columns = %+v", columns)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestFetchColumnsIncludesPageNamesForHistoricalRefresh(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("(?s)SELECT c.id,c.name,c.code,c.page_id,p.name.*INNER JOIN page p.*p.status=1.*c.status=1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "page_id", "page_name", "parent_id", "description", "sort"}).
			AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
			AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20))
	columns, err := New(db, 1).FetchColumns(context.Background())
	if err != nil || len(columns) != 2 {
		t.Fatalf("columns = %+v, err = %v", columns, err)
	}
	if !slices.Equal([]string{columns[0].PageName, columns[1].PageName}, []string{"资讯动态", "关于我们"}) {
		t.Fatalf("missing page names: %+v", columns)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleRelatedColumnsPropagatesDatabaseFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := errors.New("database unavailable")
	mock.ExpectQuery("SELECT DISTINCT c.id").WithArgs(int64(1001)).WillReturnError(want)
	if _, err := New(db, 1).FetchArticleRelatedColumns(context.Background(), 1001); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePageIDRejectsDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM page WHERE name=? AND status=1 AND deleted_at IS NULL ORDER BY id LIMIT 2")).WithArgs("资讯动态").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	if _, err := ResolvePageID(context.Background(), db, "资讯动态"); err != ErrPageNotUnique {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchArticleColumnsUsesPublishableTypeAndReturnsPageName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("(?s)SELECT DISTINCT c.id.*a.status=1.*a.audit_status=2.*a.deleted_at IS NULL.*a.type IN \\(1,2\\)").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "page_id", "page_name", "parent_id", "description", "sort"}).
			AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
			AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20))
	columns, err := New(db, 1).FetchArticleColumns(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0].PageName != "资讯动态" || columns[1].PageName != "关于我们" {
		t.Fatalf("unexpected columns: %+v", columns)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchByColumnUsesPublishedFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT id,name,code,page_id,parent_id").WithArgs(int64(7), "中心动态").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "page_id", "parent_id", "description", "sort"}).AddRow(21, "中心动态", "center", 7, 0, "", 10))
	mock.ExpectQuery("(?s)FROM article_column_publish acp.*a.status=1.*a.audit_status=2.*a.deleted_at IS NULL.*a.type IN \\(1,2\\).*ORDER BY acp.is_top DESC").WithArgs(int64(21), 10).WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "is_top", "default_color", "url", "publish_time"}).AddRow(1001, 1, "标题", "摘要", "正文", "", "", "中心", 0, 1, "", "", time.Now()))
	_, items, err := New(db, 7).FetchByColumnName(context.Background(), "中心动态", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ColumnID != 21 {
		t.Fatalf("unexpected items: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
