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

	"portal-static/internal/adapters/caam/model"
)

func TestRelatedArticleRecoversRemovedRelationsFromRequestedOutput(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "move", true: "delete"}[deleting], func(t *testing.T) {
			cfg, aboutSource, source := siteTestFixture(t)
			source.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
			source.relatedColumns = []model.Column{source.column}
			source.detail = model.Article{ID: "900", ColumnID: 36, Type: model.ArticleTypeContent, Title: "新文章", PublishTime: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)}
			if deleting {
				source.relatedColumns = []model.Column{}
				source.detailErr = ErrArticleNotPublished
			}
			pages, home := relatedTestGenerators(t, cfg, source)
			site := NewSiteGenerator(cfg, source, aboutSource, pages, home, nil)
			output := filepath.Join(t.TempDir(), "deployment")
			oldList := filepath.Join(output, "list", "42", "2.html")
			mustWriteTestFile(t, oldList, `<a href="../../article/2026/08/900.html?from=list&amp;column_id=42">旧引用</a>`)
			detail := filepath.Join(output, "article", "2026", "08", "900.html")
			mustWriteTestFile(t, detail, "old detail")
			ctx := WithOutputPath(context.Background(), output)
			if deleting {
				result, err := site.DeleteArticleRelated(ctx, 900)
				if err != nil {
					t.Fatal(err)
				}
				if !result.Deleted || !reflect.DeepEqual(result.RefreshedColumnIDs, []int64{42}) {
					t.Fatalf("unexpected deletion: %#v", result)
				}
				if _, err := os.Stat(detail); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("detail remains: %v", err)
				}
			} else {
				result, err := site.GenerateArticleRelated(ctx, 900)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result.RefreshedColumnIDs, []int64{36, 42}) {
					t.Fatalf("unexpected generation: %#v", result)
				}
			}
			if _, err := os.Stat(oldList); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("old pagination remains: %v", err)
			}
			list, err := os.ReadFile(filepath.Join(output, "list", "42", "1.html"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(list), "/900.html") {
				t.Fatal("old article link remains")
			}
		})
	}
}

func TestListReferencesArticleMatchesExactInternalDetail(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "list", "42", "1.html")
	for _, tc := range []struct {
		href string
		want bool
	}{
		{"../../article/2026/08/900.html?from=list&column_id=42", true},
		{"../../article/900/index.html", true},
		{"../../article/2026/08/1900.html", false},
		{"../../article/2026/08/900.html.bak", false},
		{"https://example.com/article/2026/08/900.html", false},
		{"../../list/900/1.html", false},
	} {
		t.Run(tc.href, func(t *testing.T) {
			mustWriteTestFile(t, source, `<a href="`+tc.href+`">article</a>`)
			got, err := listReferencesArticle(context.Background(), root, source, 900)
			if err != nil || got != tc.want {
				t.Fatalf("found=%v err=%v, want %v", got, err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := listReferencesArticle(ctx, root, source, 900); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestDeleteArticlePreservesDetailOnRelatedQueryOrListFailure(t *testing.T) {
	for _, stage := range []string{"query", "list"} {
		t.Run(stage, func(t *testing.T) {
			cfg, aboutSource, source := siteTestFixture(t)
			source.column = model.Column{ID: 36, Name: cfg.WorkPage.Association}
			source.detailErr = ErrArticleNotPublished
			if stage == "query" {
				source.relatedErr = errors.New("database unavailable")
			} else {
				source.columnErr = errors.New("list unavailable")
			}
			pages, home := relatedTestGenerators(t, cfg, source)
			site := NewSiteGenerator(cfg, source, aboutSource, pages, home, nil)
			detail := filepath.Join(cfg.Site.DistRoot, "article", "2026", "08", "900.html")
			mustWriteTestFile(t, detail, "old detail")
			if _, err := site.DeleteArticleRelated(context.Background(), 900); err == nil {
				t.Fatal("expected failure")
			}
			data, err := os.ReadFile(detail)
			if err != nil || string(data) != "old detail" {
				t.Fatalf("detail changed: %q %v", data, err)
			}
		})
	}
}

func TestArticleRefreshRetainsOldColumnsForRetry(t *testing.T) {
	cfg, aboutSource, source := siteTestFixture(t)
	source.relatedColumns = []model.Column{}
	source.detailErr = ErrArticleNotPublished
	pages, home := relatedTestGenerators(t, cfg, source)
	site := NewSiteGenerator(cfg, source, aboutSource, pages, home, nil)
	list := filepath.Join(cfg.Site.DistRoot, "list", "42", "1.html")
	mustWriteTestFile(t, list, `<a href="../../article/2026/08/900.html">old</a>`)
	columns, err := site.resolveArticleRefreshColumns(context.Background(), 900)
	if err != nil || len(columns) != 1 || columns[0].ID != 42 {
		t.Fatalf("resolve old column: %v %v", columns, err)
	}
	// Simulate the list succeeding before the main-page stage fails.
	mustWriteTestFile(t, list, "refreshed list without the article")
	result, err := site.DeleteArticleRelated(context.Background(), 900)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.RefreshedColumnIDs, []int64{42}) || !reflect.DeepEqual(result.RefreshedPages, []string{"stats"}) {
		t.Fatalf("retry lost the old column: %#v", result)
	}
	if _, err := os.Stat(site.articleRefreshStatePath(900)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed refresh retained pending state: %v", err)
	}
}
