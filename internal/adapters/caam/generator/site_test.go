package generator

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

type fakeAboutSource struct {
	columns map[string]PreparedColumn
	byID    map[int64]PreparedColumn
	err     error
}

func (f fakeAboutSource) FetchPublishedByColumnName(_ context.Context, name string) (model.Column, []model.Article, error) {
	if f.err != nil {
		return model.Column{}, nil, f.err
	}
	column, ok := f.columns[name]
	if !ok {
		return model.Column{}, nil, ErrColumnNotFound
	}
	return column.Column, append([]model.Article(nil), column.Articles...), nil
}

func (f fakeAboutSource) FetchPublishedByColumnNameLimit(ctx context.Context, name string, types []int, limit int) (model.Column, []model.Article, error) {
	column, articles, err := f.FetchPublishedByColumnName(ctx, name)
	if err != nil {
		return column, nil, err
	}
	allowed := make(map[int]struct{}, len(types))
	for _, articleType := range types {
		allowed[articleType] = struct{}{}
	}
	filtered := make([]model.Article, 0, limit)
	for _, article := range articles {
		if len(allowed) > 0 {
			if _, ok := allowed[article.Type]; !ok {
				continue
			}
		}
		filtered = append(filtered, article)
		if len(filtered) == limit {
			break
		}
	}
	if len(filtered) > 0 {
		column.ID = filtered[0].ColumnID
	}
	return column, filtered, nil
}

func (f fakeAboutSource) FetchPublishedByColumnID(_ context.Context, id int64) (model.Column, []model.Article, error) {
	if f.err != nil {
		return model.Column{}, nil, f.err
	}
	if column, ok := f.byID[id]; ok {
		return column.Column, append([]model.Article(nil), column.Articles...), nil
	}
	return model.Column{}, nil, ErrColumnNotFound
}

func TestGeneratePagesToleratesMissingConfiguredColumns(t *testing.T) {
	cfg, _, homeSource := siteTestFixture(t)
	missingAbout := fakeAboutSource{columns: map[string]PreparedColumn{}, byID: map[int64]PreparedColumn{}}
	homeSource.column = model.Column{}
	homeSource.columnErr = ErrColumnNotFound
	homeSource.footerErr = ErrColumnNotFound
	homeSource.allColumns = nil
	homeSource.allArticles = nil
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, missingAbout, pages, home, nil)

	result, err := site.GeneratePages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 6 || len(result.Pages) != 6 || result.GeneratedFiles != 6 ||
		result.GeneratedDetails != 0 || result.GeneratedLists != 0 || result.DurationSeconds <= 0 {
		t.Fatalf("unexpected empty page result: %#v", result)
	}
	for _, name := range []string{"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html"} {
		page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, name))
		if err != nil {
			t.Fatalf("read generated %s: %v", name, err)
		}
		if len(page) == 0 {
			t.Fatalf("generated %s is empty", name)
		}
		if index := bytes.Index(page, []byte(`href="list/0/`)); index >= 0 {
			end := index + 100
			if end > len(page) {
				end = len(page)
			}
			t.Fatalf("generated %s contains an unresolved list link: %s", name, page[index:end])
		}
	}
}

func TestSplitAboutColumnsKeepsVerticalReadingOrder(t *testing.T) {
	items := []ArticleView{{Title: "1"}, {Title: "2"}, {Title: "3"}, {Title: "4"}, {Title: "5"}}
	columns := splitAboutColumns(items)
	if len(columns) != 2 || len(columns[0]) != 3 || len(columns[1]) != 2 ||
		columns[0][2].Title != "3" || columns[1][0].Title != "4" {
		t.Fatalf("unexpected columns: %#v", columns)
	}
}

func TestSplitLeaderTitle(t *testing.T) {
	tests := []struct {
		title   string
		summary string
		name    string
		role    string
	}{
		{"中国汽车工业协会副秘书长陈旭简历", "", "陈旭", "副秘书长"},
		{"中国汽车工业协会常务副会长兼秘书长付炳锋简历", "", "付炳锋", "常务副会长兼秘书长"},
		{"中国汽车工业协会监事会监事长、专务副秘书长姚杰简历", "", "姚杰", "监事会监事长、专务副秘书长"},
		{"张会长", "会长", "张会长", "会长"},
		{"外部领导资料", "", "外部领导资料", ""},
	}
	for _, test := range tests {
		name, role := splitLeaderTitle(test.title, test.summary)
		if name != test.name || role != test.role {
			t.Fatalf("splitLeaderTitle(%q, %q) = (%q, %q), want (%q, %q)", test.title, test.summary, name, role, test.name, test.role)
		}
	}
}

func TestGenerateAboutPublishesDetailsListsAndDynamicPage(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GeneratePage(context.Background(), "about")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "about" || result.GeneratedDetails != 0 || result.GeneratedLists != 0 ||
		result.GeneratedFiles != 1 || result.DurationSeconds <= 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "about.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{
		"真实协会简介", "张会长", "会长",
		"article/2026/08/101.html?from=about&amp;section=leadership",
		"https://example.com/external-leader", "list/206/1.html",
		`class="is-bold"`, `style="color:#0055AA"`, "暂无内容",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated about page missing %q", want)
		}
	}
	for _, forbidden := range []string{"onclick=", "href=\"#\"", "演示姓名", "leader-photo"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("generated about page contains %q", forbidden)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "article")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("single page generation must not generate details: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("single page generation must not generate lists: %v", err)
	}
}

func TestGenerateAboutKeepsPreviousPageWhenTemplateFails(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	oldPage := []byte("previous generated about")
	if err := os.WriteFile(filepath.Join(cfg.Site.DistRoot, "about.html"), oldPage, 0o644); err != nil {
		t.Fatal(err)
	}
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.About.Template = filepath.Join(t.TempDir(), "missing-about-template.tmpl")
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	if _, err := site.GeneratePage(context.Background(), "about"); err == nil {
		t.Fatal("expected template error")
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "about.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(page) != string(oldPage) {
		t.Fatalf("previous about page was replaced: %q", page)
	}
}

func TestGenerateHomePublishesDistIndexWithoutOverwritingSourcePages(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	distCfg := cfg
	distCfg.Site.OutputRoot = cfg.Site.DistRoot
	distCfg.Site.Output = filepath.Join(cfg.Site.DistRoot, "index.html")
	pages, err := NewPageGenerator(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(cfg.Site.DistRoot, "index.generated.html")
	if err := os.WriteFile(legacy, []byte("stale generated home"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	result, err := site.GeneratePage(context.Background(), "home")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "home" || filepath.Clean(result.Output) != filepath.Join(cfg.Site.DistRoot, "index.html") {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), `href="#"`) || !strings.Contains(string(page), `href="about.html"`) {
		t.Fatalf("unexpected generated homepage links")
	}
	if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy dist homepage should be removed: %v", err)
	}
	sourcePage, err := os.ReadFile(filepath.Join(cfg.Site.OutputRoot, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sourcePage) != "original source home" {
		t.Fatalf("source homepage was overwritten: %q", sourcePage)
	}
}

func TestGenerateWorkUsesHeadlineListsPlatformsAndDetails(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GeneratePage(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "work" || result.GeneratedDetails != 0 || result.GeneratedLists != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "work.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"真实协会工作头条", "真实协会动态", "真实专业平台", "article/2026/08/401.html?from=work&amp;section=association", `href="list/36/1.html"`, `data-work-tab="association"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated work page missing %q", want)
		}
	}
	if strings.Contains(html, "href=\"#\"") {
		t.Fatal("generated work page contains placeholder link")
	}
}

func TestGenerateStatsSeparatesChartDataFromReportDetails(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GeneratePage(context.Background(), "stats")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "stats" || result.GeneratedDetails != 0 || result.GeneratedLists != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "stats.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"国内数据报告", "汽车月度销量", "--a:", "article/2026/08/501.html?from=stats&amp;section=domestic", `href="list/42/1.html"`, `data-stat-tab="domestic"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated stats page missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "502.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("chart data article must not generate a detail page: %v", err)
	}
}

func TestGenerateMembersUsesConfiguredSectionsAndPublishesDetails(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GeneratePage(context.Background(), "members")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "members" || result.GeneratedDetails != 0 || result.GeneratedLists != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "members.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"真实会员工作", "真实会员管理办法", "真实会长单位", "article/2026/08/601.html?from=members&amp;section=work", "article/2026/08/620.html?from=members&amp;section=management", `href="list/61/1.html"`, `data-member-tab`} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated members page missing %q", want)
		}
	}
	if strings.Contains(html, "onclick=") || strings.Contains(html, "href=\"#\"") {
		t.Fatal("generated members page contains unsafe or placeholder markup")
	}
}

func TestGeneratePartyUsesNewsCarouselWorkAndStudyColumns(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GeneratePage(context.Background(), "party")
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != "party" || result.GeneratedDetails != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "party.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{"真实党建要闻", "真实党建轮播", "真实党建工作动态", "真实学习教育", "article/2026/08/704.html?from=party&amp;section=carousel", `<time class="party-card-date" datetime="2026-08-05">2026年8月5日</time>`, `<a href="list/69/1.html">工作动态</a>`, `<a href="list/70/1.html">学习教育</a>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated party page missing %q", want)
		}
	}
	if strings.Contains(html, "href=\"#\"") {
		t.Fatal("generated party page contains placeholder link")
	}
	for _, unwanted := range []string{"动态摘要", `<time class="party-card-date" datetime="2026-08-05">发布时间：`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("generated party page should not contain %q", unwanted)
		}
	}
}

func TestGenerateSiteArticlesIncludesMembersAndParty(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GenerateSiteArticles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 24 || result.SkippedExternal != 2 || result.SkippedData != 2 {
		t.Fatalf("unexpected site article result: %#v", result)
	}
	for _, id := range []string{"601", "704"} {
		if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", id+".html")); err != nil {
			t.Fatalf("expected configured detail %s: %v", id, err)
		}
	}
}

func TestGenerateSitePublishesCompleteDirectoryAtomically(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(cfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Site.DistRoot, "old.txt"), []byte("old deployment"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	result, err := site.GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.GeneratedDetails != 24 || len(result.Pages) != 6 || filepath.Clean(result.Output) != filepath.Clean(cfg.Site.DistRoot) {
		t.Fatalf("unexpected site result: %#v", result)
	}
	for _, page := range []string{"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html"} {
		if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, page)); err != nil {
			t.Fatalf("expected generated page %s: %v", page, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old deployment artifact should be removed: %v", err)
	}
}

func TestGenerateSiteGrayscaleAppliesToMainListAndArticlePages(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(cfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	result, err := site.GenerateSite(WithGrayscale(context.Background(), true))
	if err != nil {
		t.Fatal(err)
	}
	if result.Gray != "1" {
		t.Fatalf("unexpected site result: %#v", result)
	}
	for _, relative := range []string{
		"index.html",
		filepath.Join("list", "42", "1.html"),
		filepath.Join("article", "2026", "08", "101.html"),
	} {
		page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		if !bytes.Contains(page, []byte(grayscaleMarker)) {
			t.Fatalf("page %s is not grayscale", relative)
		}
	}
}

func TestGenerateSiteKeepsPreviousDirectoryWhenPageFails(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	cfg.PartyPage.Template = filepath.Join(t.TempDir(), "missing-party-template.tmpl")
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(cfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cfg.Site.DistRoot, "previous.txt")
	if err := os.WriteFile(marker, []byte("previous deployment"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	if _, err := site.GenerateSite(context.Background()); err == nil {
		t.Fatal("expected site generation failure")
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "previous deployment" {
		t.Fatalf("previous deployment was not preserved: content=%q error=%v", content, err)
	}
}

func TestGeneratePreparedSiteArticlesDeduplicatesAndSkipsExternalLinks(t *testing.T) {
	cfg := pageTestConfig(t)
	published := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	article := model.Article{ID: "701", ColumnID: 81, Type: model.ArticleTypeContent, Title: "跨栏目文章", PublishTime: published}
	columns := []PreparedColumn{
		{Key: "intro", Column: model.Column{ID: 81, Name: "栏目一"}, Articles: []model.Article{article}},
		{Key: "charter", Column: model.Column{ID: 82, Name: "栏目二"}, Articles: []model.Article{{ID: "701", ColumnID: 82, Type: model.ArticleTypeContent, Title: "重复文章", PublishTime: published}}},
		{Key: "leadership", Column: model.Column{ID: 83, Name: "栏目三"}, Articles: []model.Article{{ID: "702", ColumnID: 83, Type: model.ArticleTypeContent, Title: "外链", URL: "https://example.com/702", PublishTime: published}}},
	}
	pages, err := NewPageGenerator(cfg, fakePageSource{footerLinks: map[string][]model.Article{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := pages.generatePreparedArticles(context.Background(), columns, ".test-site.lock")
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 1 || result.SkippedExternal != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.OutputRoot, "article", "2026", "08", "701.html")); err != nil {
		t.Fatalf("expected deduplicated detail: %v", err)
	}
}

func TestLegacyListGenerationPublishesOnlyUnderDistRoot(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	distCfg := cfg
	distCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, nil, nil)

	result, err := site.GenerateList(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cfg.Site.DistRoot, "list", "42")
	if filepath.Clean(result.Output) != filepath.Clean(want) {
		t.Fatalf("list output = %q, want %q", result.Output, want)
	}
	if _, err := os.Stat(filepath.Join(want, "1.html")); err != nil {
		t.Fatalf("expected dist list page: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.OutputRoot, "list")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source root must not receive generated lists: %v", err)
	}
}

func TestGenerateAllListsPreservesPublishedScaffold(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	distCfg := cfg
	distCfg.Site.OutputRoot = cfg.Site.DistRoot
	distCfg.Site.Output = filepath.Join(cfg.Site.DistRoot, "index.html")
	pages, err := NewPageGenerator(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	files := map[string][]byte{
		"index.html":                        []byte("published home"),
		"search.html":                       []byte("published search"),
		filepath.Join("css", "site.css"):    []byte("published css"),
		filepath.Join("js", "site.js"):      []byte("published js"),
		filepath.Join("assets", "logo.svg"): []byte("published asset"),
	}
	for name, content := range files {
		path := filepath.Join(cfg.Site.DistRoot, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
		sourcePath := filepath.Join(cfg.Site.OutputRoot, name)
		if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sourcePath, []byte("source replacement"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := site.GenerateAllLists(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, name))
		if err != nil {
			t.Fatalf("read preserved file %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("list generation changed %s: got %q, want %q", name, got, want)
		}
	}
}

func siteTestFixture(t *testing.T) (config.Config, fakeAboutSource, fakePageSource) {
	t.Helper()
	cfg := config.Default()
	sourceRoot := t.TempDir()
	distRoot := t.TempDir()
	for _, directory := range []string{"assets", "css", "js"} {
		if err := os.MkdirAll(filepath.Join(sourceRoot, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "about.html"), []byte("manual about must not be copied"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "index.html"), []byte("original source home"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "index.generated.html"), []byte("generated home"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Site.OutputRoot = sourceRoot
	cfg.Site.DistRoot = distRoot
	cfg.Site.Template = filepath.Join("..", "testdata", "templates", "home.html.tmpl")
	cfg.Site.ListTemplate = filepath.Join("..", "testdata", "templates", "list.html.tmpl")
	cfg.Site.ArticleTemplate = filepath.Join("..", "testdata", "templates", "article.html.tmpl")
	cfg.About.Template = filepath.Join("..", "testdata", "templates", "about.html.tmpl")
	cfg.WorkPage.Template = filepath.Join("..", "testdata", "templates", "work.html.tmpl")
	cfg.StatsPage.Template = filepath.Join("..", "testdata", "templates", "stats.html.tmpl")
	cfg.MembersPage.Template = filepath.Join("..", "testdata", "templates", "members.html.tmpl")
	cfg.PartyPage.Template = filepath.Join("..", "testdata", "templates", "party.html.tmpl")
	cfg.Site.LockStaleAfter = "1h"

	published := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	columns := make(map[string]PreparedColumn)
	columnsByID := make(map[int64]PreparedColumn)
	add := func(name string, id int64, articles ...model.Article) {
		for index := range articles {
			articles[index].ColumnID = id
		}
		column := PreparedColumn{Column: model.Column{ID: id, Name: name}, Articles: articles}
		columns[name] = column
		columnsByID[id] = column
	}
	add(cfg.About.Intro, 91, model.Article{ID: "100", Type: model.ArticleTypeContent, Title: "简介", Content: `<p onclick="bad()">真实协会简介</p>`, PublishTime: published})
	add(cfg.About.Leadership, 92,
		model.Article{ID: "101", Type: model.ArticleTypeContent, Title: "张会长", Summary: "会长", Cover: "/caam/uploads/leader.jpg", PublishTime: published},
		model.Article{ID: "102", Type: model.ArticleTypeContent, Title: "外部领导资料", URL: "https://example.com/external-leader", PublishTime: published.Add(-time.Hour)},
	)
	add(cfg.About.Charter, 93, model.Article{ID: "103", Type: model.ArticleTypeContent, Title: "协会章程全文", IsBold: true, DefaultColor: "#0055AA", PublishTime: published})
	add(cfg.About.Organization, 94)
	add(cfg.About.Responsibilities, 95)
	add(cfg.About.Honors, 96)
	memberNames := []string{cfg.About.RotatingPresident, cfg.About.VicePresident, cfg.About.ExecutiveDirector, cfg.About.Director, cfg.About.MemberDelegate, cfg.About.RegularMember}
	for index, name := range memberNames {
		columnID := int64(201 + index)
		articleID := stringID(int64(301 + index))
		add(name, columnID, model.Article{ID: articleID, Type: model.ArticleTypeContent, Title: name + "文章", PublishTime: published})
	}
	add(cfg.WorkPage.Headline, 35, model.Article{ID: "400", Type: model.ArticleTypeContent, Title: "真实协会工作头条", Summary: "头条摘要", PublishTime: published})
	add(cfg.WorkPage.Association, 36, model.Article{ID: "401", Type: model.ArticleTypeContent, Title: "真实协会动态", PublishTime: published})
	add(cfg.WorkPage.Branch, 37)
	add(cfg.WorkPage.Industry, 38)
	add(cfg.WorkPage.International, 39)
	add(cfg.WorkPage.Expo, 40)
	add(cfg.WorkPage.Platform, 41, model.Article{ID: "402", Type: model.ArticleTypeContent, Title: "真实专业平台", URL: "https://example.com/platform", PublishTime: published})
	add(cfg.StatsPage.DomesticReports, 42, model.Article{ID: "501", Type: model.ArticleTypeContent, Title: "国内数据报告", PublishTime: published})
	add(cfg.StatsPage.OverseasReports, 43)
	add(cfg.StatsPage.ProductionReports, 44)
	add(cfg.StatsPage.ImportExportReports, 45)
	add(cfg.StatsPage.DomesticChart, 46,
		model.Article{ID: "502", Type: model.ArticleTypeData, Title: "汽车月度销量", Summary: "2026-08", Content: "35.6", PublishTime: published},
		model.Article{ID: "503", Type: model.ArticleTypeData, Title: "汽车月度销量", Summary: "2025-08", Content: "31.2", PublishTime: published.Add(-365 * 24 * time.Hour)},
	)
	add(cfg.StatsPage.OverseasChart, 47)
	add(cfg.StatsPage.ProductionChart, 48)
	add(cfg.StatsPage.ImportExportChart, 49)
	add(cfg.MembersPage.Work, 61, model.Article{ID: "601", Type: model.ArticleTypeContent, Title: "真实会员工作", PublishTime: published})
	add(cfg.MembersPage.Style, 62)
	add(cfg.MembersPage.Policy, 63, model.Article{ID: "603", Type: model.ArticleTypeContent, Title: "真实相关制度", PublishTime: published})
	add(cfg.MembersPage.Charter, 65)
	add(cfg.MembersPage.Fees, 66, model.Article{ID: "606", Type: model.ArticleTypeContent, Title: "真实会费办法", PublishTime: published})
	add(cfg.MembersPage.President, 67, model.Article{ID: "607", Type: model.ArticleTypeContent, Title: "真实会长单位", Cover: "/caam/uploads/president.png", PublishTime: published})
	add(cfg.MembersPage.VicePresident, 68)
	add(cfg.MembersPage.BranchIntro, 76, model.Article{ID: "616", Type: model.ArticleTypeContent, Title: "真实机构介绍", PublishTime: published})
	add(cfg.MembersPage.BranchRules, 77)
	add(cfg.MembersPage.BranchService, 78)
	add(cfg.MembersPage.BranchRoster, 79)
	add(cfg.MembersPage.Management, 80, model.Article{ID: "620", Type: model.ArticleTypeContent, Title: "会员管理办法", Content: `<p onclick="bad()">真实会员管理办法</p>`, PublishTime: published})
	add(cfg.MembersPage.ExecutiveDirector, 81, model.Article{ID: "621", Type: model.ArticleTypeContent, Title: "真实常务理事", PublishTime: published})
	add(cfg.MembersPage.Director, 82)
	add(cfg.MembersPage.Delegate, 83)
	add(cfg.MembersPage.Member, 84, model.Article{ID: "624", Type: model.ArticleTypeContent, Title: "真实会员单位", Cover: "/caam/uploads/member.png", PublishTime: published})
	add(cfg.PartyPage.Work, 69, model.Article{ID: "701", Type: model.ArticleTypeContent, Title: "真实党建工作动态", Summary: "动态摘要", PublishTime: published})
	add(cfg.PartyPage.Study, 70, model.Article{ID: "702", Type: model.ArticleTypeContent, Title: "真实学习教育", PublishTime: published})
	add(cfg.PartyPage.News, 71, model.Article{ID: "703", Type: model.ArticleTypeContent, Title: "真实党建要闻", PublishTime: published})
	add(cfg.PartyPage.Carousel, 75, model.Article{ID: "704", Type: model.ArticleTypeContent, Title: "真实党建轮播", Cover: "/caam/uploads/party.jpg", PublishTime: published})
	allColumns := make([]model.Column, 0, len(columnsByID))
	allArticles := make([]model.Article, 0)
	for _, prepared := range columnsByID {
		allColumns = append(allColumns, prepared.Column)
		allArticles = append(allArticles, prepared.Articles...)
	}
	return cfg, fakeAboutSource{columns: columns, byID: columnsByID}, fakePageSource{
		column:      model.Column{ID: 42, Name: "首页栏目"},
		allColumns:  allColumns,
		allArticles: allArticles,
		footerLinks: map[string][]model.Article{},
	}
}

func stringID(value int64) string {
	return strconv.FormatInt(value, 10)
}
