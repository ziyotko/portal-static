package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/sources/portalcms"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDatabaseOperationTimeoutIsBoundedAndPreservesShorterCallerDeadline(t *testing.T) {
	ctx, cancel := withDatabaseOperationTimeout(context.Background())
	deadline, ok := ctx.Deadline()
	cancel()
	if !ok || time.Until(deadline) < 29*time.Second || time.Until(deadline) > databaseOperationTimeout {
		t.Fatalf("unexpected database deadline: %v", deadline)
	}

	parent, parentCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer parentCancel()
	parentDeadline, _ := parent.Deadline()
	child, childCancel := withDatabaseOperationTimeout(parent)
	defer childCancel()
	childDeadline, _ := child.Deadline()
	if !childDeadline.Equal(parentDeadline) {
		t.Fatalf("database timeout extended caller deadline: parent=%v child=%v", parentDeadline, childDeadline)
	}
}

func TestResolvePageIDUsesActivePageName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT id.*FROM caam_portal\.page.*name = \?.*status = 1.*deleted_at IS NULL.*LIMIT 2`).
		WithArgs("首页").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	id, err := ResolvePageID(context.Background(), db, " 首页 ")
	if err != nil || id != 3 {
		t.Fatalf("ResolvePageID() = %d, %v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePageIDUsesConfiguredSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := portalcms.NewStore(db, "tenant_portal", false)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`(?s)SELECT id.*FROM tenant_portal\.page.*name = \?.*status = 1.*LIMIT 2`).
		WithArgs("首页").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	if id, err := ResolvePageIDWithStore(context.Background(), store, "首页"); err != nil || id != 3 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePageIDRejectsMissingOrDuplicatePage(t *testing.T) {
	for _, test := range []struct {
		name string
		ids  []int
		want error
		text string
	}{
		{name: "missing", want: ErrPageNotFound, text: "页面不存在：“首页”"},
		{name: "duplicate", ids: []int{3, 12}, want: ErrPageNotUnique, text: "匹配到 2 条记录"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"id"})
			for _, id := range test.ids {
				rows.AddRow(id)
			}
			mock.ExpectQuery(`(?s)SELECT id.*FROM caam_portal\.page.*name = \?.*status = 1.*deleted_at IS NULL.*LIMIT 2`).
				WithArgs("首页").WillReturnRows(rows)
			_, err = ResolvePageID(context.Background(), db, "首页")
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
			if !strings.Contains(err.Error(), test.text) {
				t.Fatalf("expected Chinese error containing %q, got %v", test.text, err)
			}
		})
	}
}

func TestFetchByColumnUsesPublishedFiltersAndBoundArguments(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	slot := config.SlotConfig{Key: "industry-news", Name: "行业要闻", Limit: 5, Types: []int{1, 2}}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM caam_portal.`column` WHERE page_id = ? AND name = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 2")).
		WithArgs(12, slot.Name).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(91))
	published := time.Date(2026, 7, 16, 9, 30, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type.*FROM \(.*FROM caam_portal\.article candidate.*INNER JOIN caam_portal\.article_column_publish acp.*acp\.column_id = \?.*acp\.article_id = candidate\.id.*candidate\.status = 1.*candidate\.audit_status = 2.*candidate\.deleted_at IS NULL.*candidate\.type IN \(\?,\?\).*ORDER BY candidate\.is_top DESC, candidate\.publish_time DESC, candidate\.id DESC.*LIMIT \?.*INNER JOIN caam_portal\.article a.*ORDER BY a\.is_top DESC, a\.publish_time DESC, a\.id DESC`).
		WithArgs(int64(91), 1, 2, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow("42", 1, "测试新闻", nil, nil, "/cover.jpg", nil, "协会", 1, "#0055AA", "https://example.com/42", published))

	articles, err := NewArticleRepository(db, 12).FetchByColumn(context.Background(), slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].ID != "42" || !articles[0].IsBold || articles[0].Summary != "" {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if !articles[0].PublishTime.Equal(published) {
		t.Fatalf("unexpected publish time: %v", articles[0].PublishTime)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchByColumnRejectsMissingOrDuplicateColumn(t *testing.T) {
	for _, test := range []struct {
		count int
		want  error
		text  string
	}{{count: 0, want: ErrColumnNotFound, text: "栏目不存在：“重复栏目”"}, {count: 2, want: ErrColumnNotUnique, text: "匹配到 2 条记录"}} {
		t.Run(string(rune('0'+test.count)), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"id"})
			for i := 0; i < test.count; i++ {
				rows.AddRow(i + 1)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM caam_portal.`column` WHERE page_id = ? AND name = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 2")).
				WithArgs(12, "重复栏目").
				WillReturnRows(rows)
			_, err = NewArticleRepository(db, 12).FetchByColumn(context.Background(), config.SlotConfig{Name: "重复栏目", Limit: 1, Types: []int{1}})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
			if !strings.Contains(err.Error(), test.text) {
				t.Fatalf("expected Chinese error containing %q, got %v", test.text, err)
			}
		})
	}
}

func TestResolveGlobalColumnIDUsesChineseNameAndRejectsAmbiguity(t *testing.T) {
	for _, test := range []struct {
		name string
		ids  []int64
		want int64
		err  error
	}{
		{name: "success", ids: []int64{42}, want: 42},
		{name: "missing", err: ErrColumnNotFound},
		{name: "duplicate", ids: []int64{42, 84}, err: ErrColumnNotUnique},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"id"})
			for _, id := range test.ids {
				rows.AddRow(id)
			}
			mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM caam_portal.`column` WHERE name = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 2")).
				WithArgs("行业要闻").WillReturnRows(rows)
			got, resolveErr := NewArticleRepository(db, 3).ResolveGlobalColumnID(context.Background(), " 行业要闻 ")
			if test.err != nil {
				if !errors.Is(resolveErr, test.err) {
					t.Fatalf("expected %v, got %v", test.err, resolveErr)
				}
			} else if resolveErr != nil || got != test.want {
				t.Fatalf("ResolveGlobalColumnID() = %d, %v; want %d", got, resolveErr, test.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFetchPublishedByColumnNameUsesNameAndPublicFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	name := "协会概况领导团队"
	published := time.Date(2026, 8, 5, 9, 30, 0, 0, time.Local)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE name = ? AND deleted_at IS NULL ORDER BY id ASC")).
		WithArgs(name).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(91, name))
	mock.ExpectQuery(`(?s)SELECT acp\.column_id, a\.id.*FROM caam_portal\.article_column_publish acp.*INNER JOIN caam_portal\.article a.*acp\.column_id IN \(\?\).*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*ORDER BY a\.is_top DESC, a\.publish_time DESC, a\.id DESC`).
		WithArgs(int64(91)).
		WillReturnRows(sqlmock.NewRows([]string{
			"column_id", "id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow(91, "1001", 1, "张三", "会长", "<p>简介</p>", "/uploads/leader.jpg", nil, "协会", 1, "#0055AA", nil, published))

	column, articles, err := NewArticleRepository(db, 12).FetchPublishedByColumnName(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != 91 || column.Name != name || len(articles) != 1 {
		t.Fatalf("unexpected result: %#v %#v", column, articles)
	}
	article := articles[0]
	if article.ColumnID != 91 || article.ID != "1001" || article.Content != "<p>简介</p>" || !article.IsBold {
		t.Fatalf("unexpected article: %#v", article)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchPublishedByColumnNameRejectsMissingColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE name = ? AND deleted_at IS NULL ORDER BY id ASC")).
		WithArgs("不存在栏目").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	_, _, err = NewArticleRepository(db, 12).FetchPublishedByColumnName(context.Background(), "不存在栏目")
	if !errors.Is(err, ErrColumnNotFound) {
		t.Fatalf("expected ErrColumnNotFound, got %v", err)
	}
}

func TestFetchPublishedByColumnNameMergesSameNameColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const name = "统计数据产销"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE name = ? AND deleted_at IS NULL ORDER BY id ASC")).
		WithArgs(name).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(44, name).AddRow(48, name))
	mock.ExpectQuery(`(?s)SELECT acp\.column_id, a\.id.*acp\.column_id IN \(\?,\?\).*ORDER BY a\.is_top DESC, a\.publish_time DESC, a\.id DESC`).
		WithArgs(int64(44), int64(48)).
		WillReturnRows(sqlmock.NewRows([]string{
			"column_id", "id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow(44, "2001", 1, "产销报告", nil, nil, nil, nil, nil, 0, nil, nil, time.Now()).
			AddRow(48, "2002", 3, "产销图表", nil, "100", nil, nil, nil, 0, nil, nil, time.Now()))
	column, articles, err := NewArticleRepository(db, 12).FetchPublishedByColumnName(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != 44 || len(articles) != 2 || articles[0].ColumnID != 44 || articles[1].ColumnID != 48 {
		t.Fatalf("unexpected merged result: %#v %#v", column, articles)
	}
}

func TestFetchPublishedByColumnNameLimitFiltersTypesAndUsesArticleColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	name := "统计数据产销"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE name = ? AND deleted_at IS NULL ORDER BY id ASC")).
		WithArgs(name).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(44, name).AddRow(48, name))
	published := time.Date(2026, 8, 10, 9, 0, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT selected\.column_id, a\.id.*article_column_publish acp.*acp\.column_id IN \(\?,\?\).*candidate\.type IN \(\?,\?\).*LIMIT \?.*INNER JOIN caam_portal\.article a`).
		WithArgs(int64(44), int64(48), 1, 2, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"column_id", "id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow(44, "901", 1, "产销报告", nil, "正文", nil, nil, nil, 0, nil, nil, published))

	column, articles, err := NewArticleRepository(db, 12).FetchPublishedByColumnNameLimit(context.Background(), name, []int{1, 2}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != 44 || len(articles) != 1 || articles[0].ColumnID != 44 || articles[0].ID != "901" {
		t.Fatalf("unexpected limited column result: %#v %#v", column, articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchPublishedByColumnIDDisambiguatesDuplicateNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const columnID int64 = 48
	const name = "统计数据产销"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE id = ?")).
		WithArgs(columnID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(columnID, name))
	mock.ExpectQuery(`(?s)FROM caam_portal\.article_column_publish acp.*INNER JOIN caam_portal\.article a.*acp\.column_id = \?.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL`).
		WithArgs(columnID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}))

	column, articles, err := NewArticleRepository(db, 12).FetchPublishedByColumnID(context.Background(), columnID)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != columnID || column.Name != name || len(articles) != 0 {
		t.Fatalf("unexpected result: %#v %#v", column, articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchLinksByColumnUsesLinkTableAndSortOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	slot := config.SlotConfig{Key: "friend-related", Name: "首页友链相关链接", Limit: 100, Types: []int{1, 2}}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM caam_portal.`column` WHERE page_id = ? AND name = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 2")).
		WithArgs(12, slot.Name).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(91))
	mock.ExpectQuery(`(?s)SELECT l\.id, l\.name, l\.url, l\.logo.*FROM caam_portal\.link l.*l\.deleted_at IS NULL.*l\.status = 1.*l\.page_id = \?.*c\.id = \?.*c\.deleted_at IS NULL.*ORDER BY l\.sort ASC, l\.id ASC.*LIMIT \?`).
		WithArgs(12, int64(91), 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "url", "logo"}).
			AddRow(37, "中国国际进口博览会", "https://www.ciie.org/", nil))

	links, err := NewArticleRepository(db, 12).FetchLinksByColumn(context.Background(), slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].ID != "37" || links[0].Title != "中国国际进口博览会" || links[0].URL != "https://www.ciie.org/" {
		t.Fatalf("unexpected links: %#v", links)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMonthlyStatisticsUsesTitleAndSelectedCutoff(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	title := "新能源汽车销量分析"
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*a\.type = 3.*a\.title = \?.*a\.summary LIKE \?.*a\.summary LIKE \?.*a\.summary <= \?.*ORDER BY a\.summary DESC`).
		WithArgs(title, "2025-%", "2026-%", "2026-07").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow("5237170", 3, title, "2026-07", "200", nil, nil, nil, 0, nil, nil, nil))

	articles, err := NewArticleRepository(db, 12).FetchMonthlyStatistics(context.Background(), title, "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Summary != "2026-07" || articles[0].Content != "200" {
		t.Fatalf("unexpected monthly statistics: %#v", articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMonthlyStatisticsWithoutCutoffUsesAllPeriodsForTitle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	title := "汽车月度销量分析"
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type.*a\.type = 3.*a\.title = \?.*ORDER BY a\.summary DESC`).
		WithArgs(title).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "summary", "content", "cover", "author", "source", "is_bold", "default_color", "url", "publish_time",
		}).AddRow("2", 3, title, "2026-07", "200", nil, nil, nil, 0, nil, nil, nil).
			AddRow("1", 3, title, "2025-07", "180", nil, nil, nil, 0, nil, nil, nil))

	articles, err := NewArticleRepository(db, 12).FetchMonthlyStatistics(context.Background(), title, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 || articles[0].Summary != "2026-07" || articles[1].Summary != "2025-07" {
		t.Fatalf("unexpected monthly statistics without cutoff: %#v", articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchStatisticsTitlesUsesGroupedPublishTitles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT article_title.*FROM caam_portal\\.article_column_publish.*WHERE column_id IN.*FROM caam_portal\\.`column`.*WHERE name = \\?.*status = 1.*GROUP BY article_title").
		WithArgs("首页统计数据").
		WillReturnRows(sqlmock.NewRows([]string{"article_title"}).
			AddRow("汽车月度销量分析").
			AddRow("新能源汽车月度销量分析").
			AddRow(" "))

	titles, err := NewArticleRepository(db, 12).FetchStatisticsTitles(context.Background(), "首页统计数据")
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 2 || titles[0] != "汽车月度销量分析" || titles[1] != "新能源汽车月度销量分析" {
		t.Fatalf("unexpected statistics titles: %#v", titles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchColumnArticlesUsesPublicFiltersAndStableOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE id = ? AND deleted_at IS NULL")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(42, "行业要闻"))
	published := time.Date(2026, 7, 24, 9, 0, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type.*FROM caam_portal\.article_column_publish acp.*INNER JOIN caam_portal\.article a.*a\.id = acp\.article_id.*acp\.column_id = \?.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*ORDER BY a\.is_top DESC, a\.publish_time DESC, a\.id DESC`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "url", "publish_time",
		}).AddRow("101", 1, "文章", nil, published))

	column, articles, err := NewArticleRepository(db, 12).FetchColumnArticles(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != 42 || column.Name != "行业要闻" || len(articles) != 1 || articles[0].ColumnID != 42 {
		t.Fatalf("unexpected result: %#v %#v", column, articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchColumnsReturnsEveryGlobalColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, name FROM caam_portal.`column` WHERE deleted_at IS NULL ORDER BY id ASC",
	)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(11, "首页头条").
			AddRow(14, "行业要闻").
			AddRow(31, "友情链接"))

	columns, err := NewArticleRepository(db, 12).FetchColumns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 3 || columns[0].ID != 11 || columns[2].Name != "友情链接" {
		t.Fatalf("unexpected columns: %#v", columns)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchAllArticlesUsesGlobalPublishedScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	published := time.Date(2026, 7, 24, 9, 0, 0, 0, time.Local)
	rows := sqlmock.NewRows([]string{
		"column_id", "id", "type", "title", "summary", "content", "cover",
		"author", "source", "is_bold", "default_color", "url", "publish_time",
	}).
		AddRow(14, "101", 1, "文章一", "摘要", "<p>正文</p>", nil, nil, "协会", 0, nil, nil, published).
		AddRow(15, "103", 2, "视频一", "摘要", "<p>正文</p>", nil, nil, "协会", 0, nil, nil, published)
	mock.ExpectQuery(`(?s)SELECT selected\.column_id,.*MIN\(acp\.column_id\).*GROUP BY acp\.article_id.*INNER JOIN caam_portal\.article a.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*a\.type IN \(1, 2\).*ORDER BY a\.is_top DESC, a\.publish_time DESC, a\.id DESC`).
		WillReturnRows(rows)

	articles, err := NewArticleRepository(db, 12).FetchAllArticles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 || articles[0].ID != "101" || articles[0].ColumnID != 14 || articles[1].Type != 2 {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleColumnMappingsBatchUsesPrimaryCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT id, column_id, article_id.*article_column_publish.*id > \?.*ORDER BY id ASC.*LIMIT \?`).
		WithArgs(int64(2000), 2000).
		WillReturnRows(sqlmock.NewRows([]string{"id", "column_id", "article_id"}).
			AddRow(2001, 14, 101).
			AddRow(2002, 62, 102))
	mappings, err := NewArticleRepository(db, 12).FetchArticleColumnMappingsBatch(context.Background(), 2000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 2 || mappings[0].ID != 2001 || mappings[1].ColumnID != 62 {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchListArticlesByIDsUsesPublishedFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	published := time.Date(2026, 8, 10, 9, 0, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type, a\.title, a\.url, a\.publish_time, a\.is_top.*FROM caam_portal\.article a.*a\.id IN \(\?,\?\).*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*a\.type IN \(1, 2, 3\)`).
		WithArgs(int64(101), int64(102)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "title", "url", "publish_time", "is_top"}).
			AddRow("101", 1, "置顶文章", nil, published, 1))
	articles, err := NewArticleRepository(db, 12).FetchListArticlesByIDs(context.Background(), []int64{101, 102})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].ID != "101" || !articles[0].IsTop {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleColumnsReturnsEverySupportedPublishingColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT DISTINCT c\.id, c\.name.*FROM caam_portal\.article a.*article_column_publish acp.*a\.id = \?.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*a\.type IN \(1, 2\).*c\.deleted_at IS NULL.*ORDER BY c\.id ASC`).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(40, "首页行业要闻").
			AddRow(42, "通知公告"))
	columns, err := NewArticleRepository(db, 12).FetchArticleColumns(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0].ID != 40 || columns[1].Name != "通知公告" {
		t.Fatalf("unexpected columns: %#v", columns)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleColumnsMapsEmptyResultToNotPublished(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`(?s)SELECT DISTINCT c\.id, c\.name.*a\.id = \?`).WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	_, err = NewArticleRepository(db, 12).FetchArticleColumns(context.Background(), 101)
	if !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("expected ErrArticleNotPublished, got %v", err)
	}
}

func TestFetchArticleRelatedColumnsIncludesOfflineRelationships(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"id", "name"})
			if !empty {
				rows.AddRow(40, "栏目一").AddRow(42, "栏目二")
			}
			mock.ExpectQuery("SELECT DISTINCT c.id, c.name FROM caam_portal.article_column_publish acp INNER JOIN caam_portal.`column` c ON c.id = acp.column_id WHERE acp.article_id = ? AND c.deleted_at IS NULL ORDER BY c.id ASC").
				WithArgs(int64(101)).WillReturnRows(rows)
			columns, err := NewArticleRepository(db, 12).FetchArticleRelatedColumns(context.Background(), 101)
			if err != nil {
				t.Fatal(err)
			}
			if empty && len(columns) != 0 || !empty && (len(columns) != 2 || columns[0].ID != 40 || columns[1].ID != 42) {
				t.Fatalf("unexpected columns: %#v", columns)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFetchDetailArticlesByIDsFiltersDetailTypes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	published := time.Date(2026, 8, 10, 9, 0, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT a\.id, a\.type, a\.title, a\.summary, a\.content.*a\.id IN \(\?\).*a\.type IN \(1, 2\)`).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "type", "title", "summary", "content", "cover", "author", "source",
			"is_bold", "default_color", "url", "publish_time", "is_top",
		}).AddRow("101", 1, "文章", "摘要", "<p>正文</p>", nil, nil, "协会", 1, "#123456", nil, published, 0))
	articles, err := NewArticleRepository(db, 12).FetchDetailArticlesByIDs(context.Background(), []int64{101})
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Content != "<p>正文</p>" || !articles[0].IsBold {
		t.Fatalf("unexpected articles: %#v", articles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArticleResolvesFirstPublishedColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	published := time.Date(2026, 8, 11, 9, 0, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT acp\.column_id,.*FROM caam_portal\.article a.*article_column_publish acp.*a\.id = \?.*a\.status = 1.*a\.audit_status = 2.*a\.deleted_at IS NULL.*a\.type IN \(1, 2\).*ORDER BY acp\.column_id ASC.*LIMIT 1`).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"column_id", "id", "type", "title", "summary", "content", "cover", "author", "source",
			"is_bold", "default_color", "url", "publish_time",
		}).AddRow(42, "101", 1, "文章", "摘要", "<p>正文</p>", nil, nil, "协会", 1, "#123456", nil, published))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM caam_portal.`column` WHERE id = ? AND deleted_at IS NULL")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(42, "行业要闻"))

	column, article, err := NewArticleRepository(db, 12).FetchArticle(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if column.ID != 42 || column.Name != "行业要闻" || article.ColumnID != 42 || article.ID != "101" || article.Content != "<p>正文</p>" {
		t.Fatalf("unexpected resolved article: %#v %#v", column, article)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
