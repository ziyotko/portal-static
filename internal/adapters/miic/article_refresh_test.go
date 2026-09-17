package miic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"portal-static/internal/cms/demo"
	"portal-static/internal/cms/model"
	"portal-static/internal/cms/repository"
)

func relatedTestGenerator(t *testing.T, source Source) (*Generator, Config) {
	t.Helper()
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	unique := filepath.Base(filepath.Dir(temp)) + "-" + filepath.Base(temp)
	cfg.Site.DistRoot = filepath.Join(cfg.Site.SourceRoot, "dist", unique)
	t.Cleanup(func() { _ = os.RemoveAll(cfg.Site.DistRoot) })
	g, err := New(cfg, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	return g, cfg
}

func TestGenerateArticleRelatedRefreshesCurrentAndPreviousColumns(t *testing.T) {
	g, cfg := relatedTestGenerator(t, demo.NewSource())
	if err := publishFile(filepath.Join(cfg.Site.DistRoot, "list", "22", "2.html"), []byte(`<a href="../../article/2026/08/1001.html">old</a>`)); err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateArticleRelated(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	wantColumns := []int64{11, 20, 21, 22}
	if !slices.Equal(result.RefreshedColumnIDs, wantColumns) {
		t.Fatalf("refreshed columns = %v, want %v", result.RefreshedColumnIDs, wantColumns)
	}
	if !slices.Equal(result.RefreshedPages, []string{"news"}) {
		t.Fatalf("refreshed pages = %v", result.RefreshedPages)
	}
	if result.GeneratedDetails != 1 || result.GeneratedLists != 4 || result.GeneratedFiles != 7 {
		t.Fatalf("unexpected metrics: %#v", result)
	}
	for _, name := range []string{"article/2026/08/1001.html", "list/11/1.html", "list/22/1.html", "news.html", "generated-content.js"} {
		if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, filepath.FromSlash(name))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

type removedArticleSource struct {
	Source
	articleID int64
}

func (s removedArticleSource) FetchArticleColumns(context.Context, int64) ([]model.Column, error) {
	return nil, repository.ErrArticleNotPublished
}

func (s removedArticleSource) FetchColumnArticles(ctx context.Context, id int64) (model.Column, []model.Article, error) {
	column, articles, err := s.Source.FetchColumnArticles(ctx, id)
	return column, withoutArticle(articles, s.articleID), err
}

func (s removedArticleSource) FetchByColumnName(ctx context.Context, name string, limit int) (model.Column, []model.Article, error) {
	column, articles, err := s.Source.FetchByColumnName(ctx, name, limit)
	return column, withoutArticle(articles, s.articleID), err
}

func (s removedArticleSource) FetchPageColumnArticles(ctx context.Context, pageName, columnName string, limit int) (model.Column, []model.Article, error) {
	column, articles, err := s.Source.FetchPageColumnArticles(ctx, pageName, columnName, limit)
	return column, withoutArticle(articles, s.articleID), err
}

func (s removedArticleSource) FetchAllArticles(ctx context.Context) ([]model.Article, error) {
	articles, err := s.Source.FetchAllArticles(ctx)
	return withoutArticle(articles, s.articleID), err
}

func withoutArticle(articles []model.Article, id int64) []model.Article {
	result := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		if article.ID != id {
			result = append(result, article)
		}
	}
	return result
}

func TestDeleteArticleRelatedRefreshesReferencesBeforeDeletingDetail(t *testing.T) {
	base := demo.NewSource()
	g, cfg := relatedTestGenerator(t, base)
	if _, err := g.GenerateArticle(context.Background(), 1001); err != nil {
		t.Fatal(err)
	}
	removed, err := New(cfg, removedArticleSource{Source: base, articleID: 1001}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := removed.DeleteArticleRelated(context.Background(), 1001)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Deleted || !slices.Equal(result.RefreshedColumnIDs, []int64{11, 20, 21}) || !slices.Equal(result.RefreshedPages, []string{"news"}) {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "1001.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("detail still exists: %v", err)
	}
	list, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "list", "21", "1.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(list), "1001.html") {
		t.Fatal("refreshed list still references deleted article")
	}
}

func TestDeleteArticleRelatedRejectsPublishedArticle(t *testing.T) {
	g, _ := relatedTestGenerator(t, demo.NewSource())
	_, err := g.DeleteArticleRelated(context.Background(), 1001)
	if !errors.Is(err, ErrArticleStillPublished) {
		t.Fatalf("error = %v, want ErrArticleStillPublished", err)
	}
}

func TestGenerateArticleRelatedConflictsWithSiteLock(t *testing.T) {
	g, cfg := relatedTestGenerator(t, demo.NewSource())
	if err := os.MkdirAll(filepath.Dir(cfg.Site.DistRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireLock(cfg.Site.DistRoot+".lock", g.stale)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.release()
	if _, err := g.GenerateArticleRelated(context.Background(), 1001); !errors.Is(err, ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
}

func TestUnsupportedArticleTypeDoesNotProduceLocalDetailLink(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	view := makeView(cfg, "异常栏目", model.Article{ID: 9, Type: 4})
	if view.Href != "" || view.External {
		t.Fatalf("unsupported type produced link: %#v", view)
	}
}
