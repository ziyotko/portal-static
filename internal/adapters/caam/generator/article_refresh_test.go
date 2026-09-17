package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

func TestGenerateArticleRelatedResolvesMultipleColumnsAndPages(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	published := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	article := model.Article{ID: "900", ColumnID: 36, Type: model.ArticleTypeContent, Title: "联动文章", Content: "<p>正文</p>", PublishTime: published}
	homeSource.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
	homeSource.articles = []model.Article{article}
	homeSource.detail = article
	homeSource.articleColumns = []model.Column{{ID: 36, Name: cfg.WorkPage.Association}, {ID: 42, Name: cfg.StatsPage.DomesticReports}, {ID: 36, Name: cfg.WorkPage.Association}}
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	result, err := site.GenerateArticleRelated(context.Background(), 900)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.RefreshedColumnIDs, []int64{36, 42}) {
		t.Fatalf("refreshed columns = %v", result.RefreshedColumnIDs)
	}
	if !reflect.DeepEqual(result.RefreshedPages, []string{"work", "stats"}) {
		t.Fatalf("refreshed pages = %v", result.RefreshedPages)
	}
	if result.GeneratedDetails != 1 || result.GeneratedLists != 2 || result.GeneratedFiles != 5 {
		t.Fatalf("unexpected generated counts: %#v", result)
	}
	for _, path := range []string{
		filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "900.html"),
		filepath.Join(cfg.Site.DistRoot, "list", "36", "1.html"),
		filepath.Join(cfg.Site.DistRoot, "list", "42", "1.html"),
		filepath.Join(cfg.Site.DistRoot, cfg.WorkPage.Output),
		filepath.Join(cfg.Site.DistRoot, cfg.StatsPage.Output),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected related output %s: %v", path, err)
		}
	}
}

func TestDeleteArticleRelatedRefreshesReferencesBeforeDeletingDetail(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	homeSource.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
	homeSource.articles = nil
	homeSource.detailErr = ErrArticleNotPublished
	homeSource.relatedColumns = []model.Column{{ID: 36, Name: cfg.WorkPage.Association}, {ID: 42, Name: cfg.StatsPage.DomesticReports}, {ID: 36, Name: cfg.WorkPage.Association}}
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	detail := filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "900.html")
	if err := os.MkdirAll(filepath.Dir(detail), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(detail, []byte("old detail"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := site.DeleteArticleRelated(context.Background(), 900)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Deleted || !reflect.DeepEqual(result.RefreshedColumnIDs, []int64{36, 42}) || !reflect.DeepEqual(result.RefreshedPages, []string{"work", "stats"}) {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	if result.GeneratedFiles != 4 || result.GeneratedDetails != 0 || result.GeneratedLists != 2 {
		t.Fatalf("unexpected delete generated counts: %#v", result)
	}
	if _, err := os.Stat(detail); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("detail should be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "list", "36", "1.html")); err != nil {
		t.Fatalf("list was not refreshed: %v", err)
	}
}

func TestDeleteArticleRelatedRejectsStillPublishedArticle(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	homeSource.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
	homeSource.articleColumns = []model.Column{{ID: 36, Name: cfg.WorkPage.Association}}
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	_, err := site.DeleteArticleRelated(context.Background(), 900)
	if !errors.Is(err, ErrArticleStillPublished) {
		t.Fatalf("expected ErrArticleStillPublished, got %v", err)
	}
}

func TestGenerateArticleRelatedStopsOnColumnQueryFailureBeforeWriting(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	published := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	homeSource.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
	homeSource.detail = model.Article{ID: "900", ColumnID: 36, Type: model.ArticleTypeContent, Title: "联动文章", PublishTime: published}
	homeSource.articleColumns = []model.Column{homeSource.column}
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	site.homeSource = failingRelatedSource{fakePageSource: homeSource}
	_, err := site.GenerateArticleRelated(context.Background(), 900)
	if err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("expected column query failure, got %v", err)
	}
	output := filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "900.html")
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("detail should not be written after column validation failure: %v", statErr)
	}
}

func TestGenerateArticleRelatedStopsAfterListFailure(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	published := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	homeSource.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
	homeSource.detail = model.Article{ID: "900", ColumnID: 36, Type: model.ArticleTypeContent, Title: "联动文章", PublishTime: published}
	homeSource.articleColumns = []model.Column{homeSource.column}
	homeSource.columnErr = errors.New("list failed")
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	_, err := site.GenerateArticleRelated(context.Background(), 900)
	if err == nil || !strings.Contains(err.Error(), "refresh related list 36") {
		t.Fatalf("expected list-stage failure, got %v", err)
	}
	detail := filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "900.html")
	if _, statErr := os.Stat(detail); statErr != nil {
		t.Fatalf("detail stage should have completed first: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Site.DistRoot, cfg.WorkPage.Output)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("main page stage should not run after list failure: %v", statErr)
	}
}

func TestGenerateArticleRelatedConflictsWithSiteLock(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	if err := os.WriteFile(cfg.Site.DistRoot+".lock", []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)

	_, err := site.GenerateArticleRelated(context.Background(), 900)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
}

func TestRelatedPageNamesUsesConfiguredColumnNamesAndStableOrder(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	got := site.relatedPageNames([]model.Column{
		{ID: 1, Name: cfg.PartyPage.News},
		{ID: 2, Name: cfg.Columns.Headline.Name},
		{ID: 3, Name: cfg.About.Intro},
		{ID: 4, Name: cfg.WorkPage.Association},
		{ID: 5, Name: cfg.StatsPage.DomesticReports},
		{ID: 6, Name: cfg.MembersPage.Work},
	})
	want := []string{"home", "about", "work", "stats", "members", "party"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("related pages = %v, want %v", got, want)
	}
}

func TestGenerateSiteDoesNotCreateBrokenLinkForUnsupportedArticleType(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	homeSource.allArticles = append(homeSource.allArticles, model.Article{
		ID: "5235393", ColumnID: 40, Type: 4, Title: "不支持的历史类型",
		PublishTime: time.Date(2022, 1, 10, 9, 0, 0, 0, time.UTC),
	})
	pages, home := relatedTestGenerators(t, cfg, homeSource)
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	if _, err := site.GenerateSite(context.Background()); err != nil {
		t.Fatalf("site generation should ignore unsupported detail links: %v", err)
	}
	listPage, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "list", "40", "1.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listPage), "article/2022/01/5235393.html") {
		t.Fatalf("unsupported type produced a broken article link")
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "article", "2022", "01", "5235393.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported type should not produce a detail page: %v", err)
	}
}

func relatedTestGenerators(t *testing.T, cfg config.Config, source fakePageSource) (*PageGenerator, *Generator) {
	t.Helper()
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pageCfg.Site.Output = filepath.Join(cfg.Site.DistRoot, "index.html")
	pages, err := NewPageGenerator(pageCfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(pageCfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pages, home
}

type failingRelatedSource struct{ fakePageSource }

func (f failingRelatedSource) FetchArticleRelatedColumns(context.Context, int64) ([]model.Column, error) {
	return nil, errors.New("database unavailable")
}
