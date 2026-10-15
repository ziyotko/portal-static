package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFetchHomeHeroArticlesUsesCurrentManagerAssignments(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)FROM `column` c INNER JOIN template t.*WHERE c.id=\\? AND c.status=1").
		WithArgs(int64(66)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "template_id", "parent_id", "description", "sort"}).
			AddRow(66, "轮播图", "home-hero", 9, 65, "", 1))
	mock.ExpectQuery("(?s)FROM article a.*c.id=\\? AND c.code='home-hero'.*WHERE a.status=1.*AND EXISTS .*FROM article_column ac.*ORDER BY a.is_top DESC").
		WithArgs(int64(66)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "is_top", "color", "url", "publish_time"}).
			AddRow(1144, 1, "2026 年通知", "", "", "", "", "", 0, 0, "", "", time.Date(2026, 3, 24, 9, 55, 32, 0, time.Local)))

	column, articles, err := New(db, 9).FetchColumnArticles(context.Background(), 66)
	if err != nil {
		t.Fatal(err)
	}
	if column.Code != "home-hero" || len(articles) != 1 || articles[0].ID != 1144 || articles[0].ColumnID != 66 {
		t.Fatalf("column = %+v, articles = %+v", column, articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchOtherColumnsUsesCurrentArticleStyles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)FROM `column` c INNER JOIN template t.*WHERE c.id=\\? AND c.status=1").
		WithArgs(int64(37)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "template_id", "parent_id", "description", "sort"}).
			AddRow(37, "国际交流与合作", "training-international", 9, 11, "", 2))
	mock.ExpectQuery("(?s)WITH RECURSIVE descendants AS.*COALESCE\\(a.is_bold,0\\),COALESCE\\(a.is_top,0\\),COALESCE\\(a.default_color,''\\).*FROM article_column_publish acp INNER JOIN article a.*WHERE acp.column_id IN \\(SELECT id FROM descendants\\).*ORDER BY a.is_top DESC").
		WithArgs(int64(37)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "is_top", "color", "url", "publish_time"}).
			AddRow(20038, 1, "国际交流栏目示例", "", "", "", "", "", 1, 1, "#008000", "", time.Date(2026, 3, 24, 9, 55, 32, 0, time.Local)).
			AddRow(20038, 1, "国际交流栏目示例", "", "", "", "", "", 1, 1, "#008000", "", time.Date(2026, 3, 24, 9, 55, 32, 0, time.Local)))

	column, articles, err := New(db, 9).FetchColumnArticles(context.Background(), 37)
	if err != nil {
		t.Fatal(err)
	}
	if column.Code != "training-international" || len(articles) != 1 || articles[0].ID != 20038 || articles[0].ColumnID != 37 || !articles[0].IsTop || !articles[0].IsBold || articles[0].DefaultColor != "#008000" {
		t.Fatalf("column = %+v, articles = %+v", column, articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
