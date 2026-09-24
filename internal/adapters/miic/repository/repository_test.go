package repository

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"portal-static/internal/sources/portalcms"
)

func TestFetchArticleRelatedColumnsIncludesOfflineAndRemovedRelations(t *testing.T) {
	for _, empty := range []bool{false, true} {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatal(err)
		}
		rows := sqlmock.NewRows([]string{"id", "name", "code", "template_id", "template_name", "parent_id", "description", "sort"})
		if !empty {
			rows.AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
				AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20)
		}
		mock.ExpectQuery("SELECT DISTINCT c.id,c.name,c.code,c.template_id,t.name,c.parent_id,COALESCE(c.description,''),c.sort FROM article_column_publish acp INNER JOIN `column` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id INNER JOIN template t ON t.id=c.template_id AND t.status=1 WHERE acp.article_id=? ORDER BY c.id").
			WithArgs(int64(1001)).WillReturnRows(rows)
		columns, err := New(db, 1).FetchArticleRelatedColumns(context.Background(), 1001)
		if err != nil {
			t.Fatal(err)
		}
		if empty && len(columns) != 0 || !empty && (len(columns) != 2 || columns[1].TemplateName != "关于我们") {
			t.Fatalf("columns = %+v", columns)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestFetchColumnsIncludesTemplateNamesForHistoricalRefresh(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("(?s)SELECT c.id,c.name,c.code,c.template_id,t.name.*INNER JOIN template t.*t.status=1.*c.status=1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "template_id", "template_name", "parent_id", "description", "sort"}).
			AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
			AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20))
	columns, err := New(db, 1).FetchColumns(context.Background())
	if err != nil || len(columns) != 2 {
		t.Fatalf("columns = %+v, err = %v", columns, err)
	}
	if !slices.Equal([]string{columns[0].TemplateName, columns[1].TemplateName}, []string{"资讯动态", "关于我们"}) {
		t.Fatalf("missing template names: %+v", columns)
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

func TestResolveTemplateIDRejectsDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM template WHERE name=? AND type='home' AND status=1 ORDER BY id LIMIT 2")).WithArgs("资讯动态").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	if _, err := ResolveTemplateID(context.Background(), db, "资讯动态"); err != ErrTemplateNotUnique {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveTemplateIDUsesConfiguredSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := portalcms.NewStore(db, "tenant_portal", false)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM tenant_portal.template WHERE name=? AND type='home' AND status=1 ORDER BY id LIMIT 2")).
		WithArgs("资讯动态").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	if id, err := ResolveTemplateIDWithStore(context.Background(), store, "资讯动态"); err != nil || id != 7 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleColumnsUsesPublishableTypeAndReturnsTemplateName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("(?s)SELECT DISTINCT c.id.*a.status=1.*a.audit_status=2.*a.type IN \\(1,2\\)").
		WithArgs(int64(1001)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "template_id", "template_name", "parent_id", "description", "sort"}).
			AddRow(21, "中心动态", "center", 1, "资讯动态", 0, "", 10).
			AddRow(41, "招聘信息", "recruitment", 4, "关于我们", 0, "", 20))
	columns, err := New(db, 1).FetchArticleColumns(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0].TemplateName != "资讯动态" || columns[1].TemplateName != "关于我们" {
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
	mock.ExpectQuery("SELECT id,name,code,template_id,parent_id").WithArgs(int64(7), "中心动态").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "template_id", "parent_id", "description", "sort"}).AddRow(21, "中心动态", "center", 7, 0, "", 10))
	mock.ExpectQuery("(?s)FROM article_column_publish acp.*a.status=1.*a.audit_status=2.*a.type IN \\(1,2\\).*ORDER BY acp.is_top DESC").WithArgs(int64(21), 10).WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "is_top", "default_color", "url", "publish_time"}).AddRow(1001, 1, "标题", "摘要", "正文", "", "", "中心", 0, 1, "", "", time.Now()))
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
