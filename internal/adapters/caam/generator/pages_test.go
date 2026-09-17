package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

type fakePageSource struct {
	column         model.Column
	articles       []model.Article
	detail         model.Article
	columnErr      error
	detailErr      error
	allColumns     []model.Column
	allArticles    []model.Article
	mappings       []model.ArticleColumnMapping
	batchList      []model.Article
	batchDetail    []model.Article
	relatedColumns []model.Column
	relatedErr     error
	articleColumns []model.Column
	columnIDs      map[string]int64
	homeBySlot     map[string][]model.Article
	footerLinks    map[string][]model.Article
	footerErr      error
}

func (f fakePageSource) ResolveColumnID(_ context.Context, slot config.SlotConfig) (int64, error) {
	if columnID := f.columnIDs[slot.Key]; columnID > 0 {
		return columnID, nil
	}
	return f.column.ID, f.columnErr
}

func (f fakePageSource) ResolveGlobalColumnID(_ context.Context, name string) (int64, error) {
	name = strings.TrimSpace(name)
	columns := append([]model.Column(nil), f.allColumns...)
	if f.column.ID > 0 {
		columns = append(columns, f.column)
	}
	ids := make([]int64, 0, 2)
	seen := make(map[int64]struct{})
	for _, column := range columns {
		if strings.TrimSpace(column.Name) != name {
			continue
		}
		if _, exists := seen[column.ID]; exists {
			continue
		}
		seen[column.ID] = struct{}{}
		ids = append(ids, column.ID)
	}
	if len(ids) == 0 {
		return 0, ErrColumnNotFound
	}
	if len(ids) > 1 {
		return 0, ErrColumnNotUnique
	}
	return ids[0], nil
}

func (f fakePageSource) FetchByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	return append([]model.Article(nil), f.homeBySlot[slot.Key]...), nil
}

func (f fakePageSource) FetchMonthlyStatistics(context.Context, string, string) ([]model.Article, error) {
	return nil, nil
}

func (f fakePageSource) FetchStatisticsTitles(context.Context, string) ([]string, error) {
	items := f.homeBySlot["stats"]
	titles := make([]string, 0, len(items))
	for _, item := range items {
		if title := strings.TrimSpace(item.Title); title != "" {
			titles = append(titles, title)
		}
	}
	return titles, nil
}

func (f fakePageSource) FetchColumns(context.Context) ([]model.Column, error) {
	columns := append([]model.Column(nil), f.allColumns...)
	seen := make(map[int64]struct{}, len(columns))
	for _, column := range columns {
		seen[column.ID] = struct{}{}
	}
	if len(columns) == 0 && f.column.ID > 0 {
		if _, exists := seen[f.column.ID]; !exists {
			columns = append(columns, f.column)
			seen[f.column.ID] = struct{}{}
		}
	}
	for _, article := range f.allArticles {
		if article.ColumnID <= 0 {
			continue
		}
		if _, exists := seen[article.ColumnID]; !exists {
			columns = append(columns, model.Column{ID: article.ColumnID, Name: "栏目"})
			seen[article.ColumnID] = struct{}{}
		}
	}
	return columns, nil
}

func (f fakePageSource) FetchAllArticles(context.Context) ([]model.Article, error) {
	return append([]model.Article(nil), f.allArticles...), nil
}

func (f fakePageSource) FetchArticleColumnMappingsBatch(_ context.Context, afterID int64, limit int) ([]model.ArticleColumnMapping, error) {
	mappings := append([]model.ArticleColumnMapping(nil), f.mappings...)
	if len(mappings) == 0 && len(f.allArticles) > 0 {
		for _, article := range f.allArticles {
			articleID, err := strconv.ParseInt(article.ID, 10, 64)
			if err == nil && article.ColumnID > 0 {
				mappings = append(mappings, model.ArticleColumnMapping{ID: int64(len(mappings) + 1), ColumnID: article.ColumnID, ArticleID: articleID})
			}
		}
	}
	if len(mappings) == 0 {
		for _, column := range f.allColumns {
			for _, article := range f.articles {
				articleID, err := strconv.ParseInt(article.ID, 10, 64)
				if err == nil {
					mappings = append(mappings, model.ArticleColumnMapping{ID: int64(len(mappings) + 1), ColumnID: column.ID, ArticleID: articleID})
				}
			}
		}
	}
	result := make([]model.ArticleColumnMapping, 0, limit)
	for _, mapping := range mappings {
		if mapping.ID > afterID {
			result = append(result, mapping)
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

func (f fakePageSource) FetchListArticlesByIDs(_ context.Context, ids []int64) ([]model.Article, error) {
	articles := f.batchList
	if len(articles) == 0 {
		articles = f.allArticles
	}
	if len(articles) == 0 {
		articles = f.articles
	}
	return filterFakeArticlesByIDs(articles, ids), nil
}

func (f fakePageSource) FetchDetailArticlesByIDs(_ context.Context, ids []int64) ([]model.Article, error) {
	articles := f.batchDetail
	if len(articles) == 0 {
		articles = f.allArticles
	}
	return filterFakeArticlesByIDs(articles, ids), nil
}

func (f fakePageSource) FetchArticleColumns(context.Context, int64) ([]model.Column, error) {
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	if len(f.articleColumns) > 0 {
		return append([]model.Column(nil), f.articleColumns...), nil
	}
	if f.column.ID > 0 {
		return []model.Column{f.column}, nil
	}
	return nil, ErrArticleNotPublished
}

func (f fakePageSource) FetchArticleRelatedColumns(context.Context, int64) ([]model.Column, error) {
	if f.relatedErr != nil {
		return nil, f.relatedErr
	}
	if f.relatedColumns != nil {
		return append([]model.Column(nil), f.relatedColumns...), nil
	}
	if len(f.articleColumns) > 0 {
		return append([]model.Column(nil), f.articleColumns...), nil
	}
	if f.column.ID > 0 {
		return []model.Column{f.column}, nil
	}
	return nil, nil
}

func filterFakeArticlesByIDs(articles []model.Article, ids []int64) []model.Article {
	wanted := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	result := make([]model.Article, 0, len(articles))
	seen := make(map[string]struct{})
	for _, article := range articles {
		id, err := strconv.ParseInt(article.ID, 10, 64)
		if err != nil {
			continue
		}
		if _, ok := wanted[id]; !ok {
			continue
		}
		if _, ok := seen[article.ID]; ok {
			continue
		}
		seen[article.ID] = struct{}{}
		result = append(result, article)
	}
	return result
}

func TestReplaceDirectoryPublishesWebReadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	parent := t.TempDir()
	staging, err := os.MkdirTemp(parent, ".staging-*")
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(staging, "article", "2026", "09")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(nested, "101.html")
	if err := os.WriteFile(page, []byte("page"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "site")
	if err := replaceDirectory(staging, target); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{target, filepath.Join(target, "article"), filepath.Join(target, "article", "2026"), filepath.Join(target, "article", "2026", "09")} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Fatalf("directory %s permissions = %04o, want 0755", directory, got)
		}
	}
	info, err := os.Stat(filepath.Join(target, "article", "2026", "09", "101.html"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("file permissions = %04o, want 0644", got)
	}
}

func (f fakePageSource) FetchColumnArticles(context.Context, int64) (model.Column, []model.Article, error) {
	return f.column, append([]model.Article(nil), f.articles...), f.columnErr
}

func (f fakePageSource) FetchArticle(context.Context, int64) (model.Column, model.Article, error) {
	return f.column, f.detail, f.detailErr
}

func (f fakePageSource) FetchLinksByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	return append([]model.Article(nil), f.footerLinks[slot.Key]...), f.footerErr
}

func TestGenerateListPublishesAllPagesAndRemovesObsoletePages(t *testing.T) {
	cfg := pageTestConfig(t)
	cfg.Site.PageSize = 10
	oldPage := filepath.Join(cfg.Site.OutputRoot, "list", "42", "3")
	if err := os.MkdirAll(oldPage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldPage, "index.html"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	articles := make([]model.Article, 0, 12)
	for i := 1; i <= 12; i++ {
		articles = append(articles, model.Article{
			ID: strconv.Itoa(i), ColumnID: 42, Type: model.ArticleTypeContent,
			Title: "文章" + strconv.Itoa(i), PublishTime: time.Date(2026, 7, i, 9, 0, 0, 0, time.UTC),
		})
	}
	g, err := NewPageGenerator(cfg, fakePageSource{
		column: model.Column{ID: 42, Name: "行业要闻"}, articles: articles,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateList(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalItems != 12 || result.TotalPages != 2 || result.PageSize != 10 ||
		result.GeneratedFiles != 2 || result.DurationSeconds <= 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	first, err := os.ReadFile(filepath.Join(cfg.Site.OutputRoot, "list", "42", "1.html"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(cfg.Site.OutputRoot, "list", "42", "2.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "../../article/2026/07/1.html") ||
		!strings.Contains(string(first), "from=list") ||
		!strings.Contains(string(first), "column_id=42") ||
		!strings.Contains(string(first), "page=1") ||
		!strings.Contains(string(first), "generated-inner-pages.css") ||
		!strings.Contains(string(first), "generated-inner-pages.js") ||
		!strings.Contains(string(second), "文章12") {
		t.Fatalf("generated list links or content are wrong")
	}
	if _, err := os.Stat(oldPage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete page was not removed: %v", err)
	}
}

func TestGenerateListCreatesEmptyFirstPage(t *testing.T) {
	cfg := pageTestConfig(t)
	g, err := NewPageGenerator(cfg, fakePageSource{
		column: model.Column{ID: 9, Name: "空栏目"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateList(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(result.Output, "1.html"))
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalPages != 1 || !strings.Contains(string(page), "暂无内容") {
		t.Fatalf("unexpected empty page: %#v %s", result, page)
	}
}

func TestGenerateListByNameResolvesRuntimeColumnID(t *testing.T) {
	cfg := pageTestConfig(t)
	source := fakePageSource{
		column: model.Column{ID: 42, Name: "行业要闻"},
		articles: []model.Article{{
			ID: "101", ColumnID: 42, Type: model.ArticleTypeContent,
			Title: "行业新闻", PublishTime: time.Now(),
		}},
		footerLinks: map[string][]model.Article{},
	}
	g, err := NewPageGenerator(cfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateListByName(context.Background(), " 行业要闻 ")
	if err != nil {
		t.Fatal(err)
	}
	if result.ColumnID != 42 || !strings.Contains(filepath.ToSlash(result.Output), "/list/42") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGenerateListByNameRejectsMissingOrDuplicateName(t *testing.T) {
	for _, test := range []struct {
		name    string
		columns []model.Column
		want    error
	}{
		{name: "missing", want: ErrColumnNotFound},
		{name: "duplicate", columns: []model.Column{{ID: 1, Name: "行业要闻"}, {ID: 2, Name: "行业要闻"}}, want: ErrColumnNotUnique},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := pageTestConfig(t)
			g, err := NewPageGenerator(cfg, fakePageSource{allColumns: test.columns}, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = g.GenerateListByName(context.Background(), "行业要闻")
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestDataArticleWithoutExternalURLHasNoLocalDetailLink(t *testing.T) {
	cfg := pageTestConfig(t)
	g, err := NewPageGenerator(cfg, fakePageSource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := g.innerArticle(model.Article{
		ID: "101", Type: model.ArticleTypeData, Title: "统计数据",
		PublishTime: time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC),
	}, 27, "../../")
	if view.Href != "" || view.ExternalLink {
		t.Fatalf("data article unexpectedly linked to detail: %#v", view)
	}
	external := g.innerArticle(model.Article{
		ID: "102", Type: model.ArticleTypeData, Title: "外部统计数据", URL: "https://example.com/stats",
	}, 27, "../../")
	if external.Href != "https://example.com/stats" || !external.ExternalLink {
		t.Fatalf("safe external data link was not preserved: %#v", external)
	}
}

func TestGenerateListKeepsCurrentDirectoryWhenRenderingFails(t *testing.T) {
	cfg := pageTestConfig(t)
	target := filepath.Join(cfg.Site.OutputRoot, "list", "42")
	oldPage := filepath.Join(target, "1")
	if err := os.MkdirAll(oldPage, 0o755); err != nil {
		t.Fatal(err)
	}
	oldContent := []byte("old list must remain")
	if err := os.WriteFile(filepath.Join(oldPage, "index.html"), oldContent, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Site.ListTemplate = filepath.Join(cfg.Site.OutputRoot, "missing-template.html")
	g, err := NewPageGenerator(cfg, fakePageSource{
		column: model.Column{ID: 42, Name: "行业要闻"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.GenerateList(context.Background(), 42); err == nil {
		t.Fatal("expected template error")
	}
	current, err := os.ReadFile(filepath.Join(oldPage, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(oldContent) {
		t.Fatalf("current list was replaced after a rendering failure: %q", current)
	}
}

func TestGenerateArticleSanitizesHTMLAndUsesSourceAwareNavigation(t *testing.T) {
	cfg := pageTestConfig(t)
	cfg.Site.MediaBaseURL = "https://demo.miic.com.cn"
	articles := []model.Article{
		{ID: "100", Title: "上一篇", PublishTime: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)},
		{ID: "101", Title: "当前文章", PublishTime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)},
		{ID: "102", Title: "下一篇", PublishTime: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)},
	}
	detail := model.Article{
		ID: "101", ColumnID: 42, Type: model.ArticleTypeContent, Title: "当前文章",
		Content: `<script>alert(1)</script><p onclick="bad()">安全正文</p><a href="javascript:alert(1)">危险链接</a><a href="../../../caam/uploads/a.pdf">附件</a><img src="uploads/a.jpg"><img src="../../../../caamm/uploads/b.jpg"><img src="https://cdn.example.com/c.jpg"><video controls src="/uploads/a.mp4"></video>`,
		Source:  "协会", Author: "编辑", PublishTime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
	}
	g, err := NewPageGenerator(cfg, fakePageSource{
		column: model.Column{ID: 42, Name: "协会动态"}, articles: articles, detail: detail,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateArticle(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if result.GeneratedFiles != 1 || result.DurationSeconds <= 0 {
		t.Fatalf("unexpected article generation metrics: %#v", result)
	}
	page, err := os.ReadFile(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, forbidden := range []string{"alert(1)", "onclick=", "javascript:"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("unsafe content %q survived sanitization: %s", forbidden, html)
		}
	}
	for _, expected := range []string{
		"安全正文",
		`src="https://demo.miic.com.cn/uploads/a.jpg"`,
		`src="https://demo.miic.com.cn/caam/uploads/b.jpg"`,
		`href="https://demo.miic.com.cn/caam/uploads/a.pdf"`,
		`src="https://cdn.example.com/c.jpg"`,
		`src="https://demo.miic.com.cn/uploads/a.mp4"`,
		"data-column-crumb",
		"data-return-link",
		"data-share-article",
		"data-like-article",
		"js/article-actions.js",
		"> 返回</a>",
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in generated article", expected)
		}
	}
	for _, removed := range []string{"上一篇：", "下一篇：", "article-siblings", "返回列表"} {
		if strings.Contains(html, removed) {
			t.Fatalf("removed navigation %q is still rendered", removed)
		}
	}
}

func TestGenerateArticleRemovesStalePageWhenUnpublished(t *testing.T) {
	cfg := pageTestConfig(t)
	target := filepath.Join(cfg.Site.OutputRoot, "article", "2026", "07", "101.html")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old article"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := NewPageGenerator(cfg, fakePageSource{
		column:    model.Column{ID: 42, Name: "协会动态"},
		articles:  []model.Article{{ID: "101", Title: "旧文章"}},
		detailErr: ErrArticleNotPublished,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.GenerateArticle(context.Background(), 101); !errors.Is(err, ErrArticleNotPublished) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale article directory was not removed: %v", err)
	}
}

func TestDeleteArticleRemovesCurrentAndLegacyStaticPages(t *testing.T) {
	cfg := testConfig(t)
	g, err := NewPageGenerator(cfg, fakePageSource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(cfg.Site.OutputRoot, "article", "2026", "07", "101.html")
	legacy := filepath.Join(cfg.Site.OutputRoot, "article", "42", "101", "index.html")
	for _, path := range []string{current, legacy} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("old article"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := g.DeleteArticle(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Deleted || result.ArticleID != 101 || len(result.DeletedPaths) != 2 {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	for _, path := range []string{current, filepath.Dir(legacy)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("article artifact still exists: %s: %v", path, err)
		}
	}

	result, err = g.DeleteArticle(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted || len(result.DeletedPaths) != 0 {
		t.Fatalf("repeat delete must be idempotent: %#v", result)
	}
}

func TestGenerateHomeArticlesPublishesInternalHomepageItemsOnly(t *testing.T) {
	cfg := pageTestConfig(t)
	source := fakePageSource{
		column: model.Column{ID: 42, Name: "协会动态"},
		articles: []model.Article{
			{ID: "101", Title: "站内文章"},
			{ID: "102", Title: "外链文章"},
		},
		homeBySlot: map[string][]model.Article{
			cfg.Columns.Headline.Key: {
				{
					ID: "101", ColumnID: 42, Type: model.ArticleTypeContent,
					Title: "站内文章", Content: "<p>首页详情正文</p>",
					PublishTime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
				},
			},
			cfg.Columns.Carousel.Key: {
				{
					ID: "101", ColumnID: 99, Type: model.ArticleTypeContent,
					Title: "重复的站内文章", Content: "<p>首页详情正文</p>",
					PublishTime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
				},
				{
					ID: "102", ColumnID: 42, Type: model.ArticleTypeContent,
					Title: "外链文章", URL: "https://example.com/news",
				},
			},
			cfg.Columns.Stats.Key: {
				{ID: "103", ColumnID: 42, Type: model.ArticleTypeData, Title: "统计数据"},
			},
		},
	}
	g, err := NewPageGenerator(cfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyPage := filepath.Join(cfg.Site.OutputRoot, "article", "42", "101", "index.html")
	if err := os.MkdirAll(filepath.Dir(legacyPage), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPage, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateHomeArticles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 1 || result.SkippedExternal != 1 || result.SkippedData != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page := filepath.Join(cfg.Site.OutputRoot, "article", "2026", "07", "101.html")
	content, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "首页详情正文") {
		t.Fatalf("generated homepage article is missing content")
	}
	if _, err := os.Stat(filepath.Dir(legacyPage)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy article directory was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.OutputRoot, "article", "42", "102")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("external article should not have a generated detail page: %v", err)
	}
}

func TestGenerateAllListsUsesEveryDatabaseColumn(t *testing.T) {
	cfg := pageTestConfig(t)
	allColumns := []model.Column{
		{ID: 11, Name: "首页头条"},
		{ID: 14, Name: "行业要闻"},
		{ID: 27, Name: "统计数据"},
		{ID: 31, Name: "友情链接"},
		{ID: 14, Name: "重复栏目"},
	}
	g, err := NewPageGenerator(cfg, fakePageSource{
		column:     model.Column{ID: 1, Name: "栏目"},
		allColumns: allColumns,
		articles: []model.Article{
			{ID: "101", ColumnID: 1, Type: model.ArticleTypeContent, Title: "列表文章"},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateAllLists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantColumns := 4
	if result.Generated != wantColumns || len(result.Lists) != wantColumns {
		t.Fatalf("generated columns = %d, want %d: %#v", result.Generated, wantColumns, result)
	}
	if result.TotalItems != wantColumns || result.TotalPages != wantColumns {
		t.Fatalf("unexpected aggregate counts: %#v", result)
	}
}

func TestGenerateAllArticlesUsesEveryPublishedArticleAndDeduplicates(t *testing.T) {
	cfg := pageTestConfig(t)
	published := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
	allArticles := []model.Article{
		{
			ID: "101", ColumnID: 42, Type: model.ArticleTypeContent,
			Title: "普通文章", Content: "<p>普通正文</p>", PublishTime: published,
		},
		{
			ID: "101", ColumnID: 99, Type: model.ArticleTypeContent,
			Title: "多栏目重复文章", Content: "<p>重复正文</p>", PublishTime: published,
		},
		{
			ID: "102", ColumnID: 42, Type: model.ArticleTypeData,
			Title: "数据文章", Content: "100", PublishTime: published.Add(-time.Hour),
		},
		{ID: "bad", ColumnID: 42, Type: model.ArticleTypeContent, Title: "非法文章"},
	}
	g, err := NewPageGenerator(cfg, fakePageSource{
		column:      model.Column{ID: 42, Name: "全部文章"},
		allArticles: allArticles,
		articles: []model.Article{
			{ID: "101", Title: "普通文章", PublishTime: published},
			{ID: "102", Title: "数据文章", PublishTime: published.Add(-time.Hour)},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateAllArticles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 1 || result.SkippedInvalid != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, articleID := range []string{"101"} {
		output := filepath.Join(cfg.Site.OutputRoot, "article", "2026", "07", articleID+".html")
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("expected generated article %s: %v", articleID, err)
		}
	}
}

func TestRetryReadRetriesTransientConnectionFailure(t *testing.T) {
	attempts := 0
	value, err := retryRead(context.Background(), nilLogger{}, "test read", func(context.Context) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("invalid connection")
		}
		return 42, nil
	})
	if err != nil || value != 42 || attempts != 3 {
		t.Fatalf("unexpected retry result value=%d attempts=%d err=%v", value, attempts, err)
	}
}

type nilLogger struct{}

func (nilLogger) Warn(string, ...any) {}

func TestContentSnapshotKeepsCrossColumnMembershipAndStableOrder(t *testing.T) {
	cfg := pageTestConfig(t)
	published := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	source := fakePageSource{
		allColumns: []model.Column{{ID: 14, Name: "行业要闻"}, {ID: 62, Name: "会员风采"}},
		mappings: []model.ArticleColumnMapping{
			{ID: 1, ColumnID: 14, ArticleID: 101},
			{ID: 2, ColumnID: 62, ArticleID: 101},
			{ID: 3, ColumnID: 14, ArticleID: 102},
			{ID: 4, ColumnID: 14, ArticleID: 101},
		},
		batchList: []model.Article{
			{ID: "101", Type: 1, Title: "普通", PublishTime: published.Add(time.Hour)},
			{ID: "102", Type: 1, Title: "置顶", IsTop: true, PublishTime: published},
		},
	}
	g, err := NewPageGenerator(cfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := g.loadContentSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.listArticlesByColumn[14]) != 2 || snapshot.listArticlesByColumn[14][0].ID != "102" {
		t.Fatalf("unexpected column 14 order: %#v", snapshot.listArticlesByColumn[14])
	}
	if len(snapshot.listArticlesByColumn[62]) != 1 || snapshot.listArticlesByColumn[62][0].ID != "101" {
		t.Fatalf("unexpected cross-column membership: %#v", snapshot.listArticlesByColumn[62])
	}
}

func pageTestConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Site.OutputRoot = t.TempDir()
	cfg.Site.ListTemplate = filepath.Join("..", "testdata", "templates", "list.html.tmpl")
	cfg.Site.ArticleTemplate = filepath.Join("..", "testdata", "templates", "article.html.tmpl")
	cfg.Site.LockStaleAfter = "1h"
	return cfg
}
