package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"miic-portal/backend/internal/demo"
	"miic-portal/backend/internal/model"
)

type missingRelationsSource struct{ Source }

func (missingRelationsSource) FetchArticleRelatedColumns(context.Context, int64) ([]model.Column, error) {
	return nil, nil
}

type failingRelatedSource struct {
	Source
	stage string
}

func (s failingRelatedSource) FetchArticleRelatedColumns(ctx context.Context, id int64) ([]model.Column, error) {
	if s.stage == "query" {
		return nil, errors.New("related query failed")
	}
	return s.Source.FetchArticleRelatedColumns(ctx, id)
}

func (s failingRelatedSource) FetchColumnArticles(ctx context.Context, id int64) (model.Column, []model.Article, error) {
	if s.stage == "list" {
		return model.Column{}, nil, errors.New("list query failed")
	}
	return s.Source.FetchColumnArticles(ctx, id)
}

func (s failingRelatedSource) FetchAllArticles(ctx context.Context) ([]model.Article, error) {
	if s.stage == "data" {
		return nil, errors.New("shared data query failed")
	}
	return s.Source.FetchAllArticles(ctx)
}

func TestArticleRefreshRecoversRemovedRelationsFromRequestedOutput(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "move", true: "delete"}[deleting], func(t *testing.T) {
			var source Source = demo.NewSource()
			if deleting {
				source = missingRelationsSource{removedArticleSource{Source: source, articleID: 1001}}
			}
			g, cfg := relatedTestGenerator(t, source)
			root := filepath.Join(cfg.Site.DistRoot, "requested")
			oldList := filepath.Join(root, "list", "41", "2.html")
			if err := publishFile(oldList, []byte(`<a href="../../article/2026/08/1001.html?from=list&amp;x=1">old</a>`)); err != nil {
				t.Fatal(err)
			}
			detail := filepath.Join(root, "article", "2026", "08", "1001.html")
			if err := publishFile(detail, []byte("old detail")); err != nil {
				t.Fatal(err)
			}
			// A different output root must not contribute previous relationships.
			if err := publishFile(filepath.Join(cfg.Site.DistRoot, "list", "42", "1.html"), []byte(`<a href="../../article/2026/08/1001.html">other root</a>`)); err != nil {
				t.Fatal(err)
			}
			ctx := WithOptions(context.Background(), root, false)
			if deleting {
				result, err := g.DeleteArticleRelated(ctx, 1001)
				if err != nil || !result.Deleted || !slices.Equal(result.RefreshedColumnIDs, []int64{41}) || !slices.Equal(result.RefreshedPages, []string{"about"}) {
					t.Fatalf("delete = %+v, err = %v", result, err)
				}
				if _, err := os.Stat(detail); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("detail remains: %v", err)
				}
			} else {
				result, err := g.GenerateArticleRelated(ctx, 1001)
				if err != nil || !slices.Equal(result.RefreshedColumnIDs, []int64{11, 20, 21, 41}) || !slices.Equal(result.RefreshedPages, []string{"news", "about"}) {
					t.Fatalf("generate = %+v, err = %v", result, err)
				}
			}
			if _, err := os.Stat(oldList); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("old pagination remains: %v", err)
			}
			list, err := os.ReadFile(filepath.Join(root, "list", "41", "1.html"))
			if err != nil || strings.Contains(string(list), "1001.html") {
				t.Fatalf("stale list reference: err=%v", err)
			}
			if _, err := os.Stat(articleRefreshStatePath(root, 1001)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("pending state remains: %v", err)
			}
		})
	}
}

func TestArticleRefreshKeepsDetailAndPendingColumnsForRetry(t *testing.T) {
	for _, stage := range []string{"query", "list", "data"} {
		t.Run(stage, func(t *testing.T) {
			source := missingRelationsSource{removedArticleSource{Source: demo.NewSource(), articleID: 1001}}
			g, cfg := relatedTestGenerator(t, failingRelatedSource{Source: source, stage: stage})
			root := cfg.Site.DistRoot
			detail := filepath.Join(root, "article", "2026", "08", "1001.html")
			if err := publishFile(detail, []byte("old detail")); err != nil {
				t.Fatal(err)
			}
			if err := publishFile(filepath.Join(root, "list", "41", "2.html"), []byte(`<a href="../../article/2026/08/1001.html">old</a>`)); err != nil {
				t.Fatal(err)
			}
			if _, err := g.DeleteArticleRelated(context.Background(), 1001); err == nil {
				t.Fatal("expected refresh failure")
			}
			data, err := os.ReadFile(detail)
			if err != nil || string(data) != "old detail" {
				t.Fatalf("detail changed on failure: %q, %v", data, err)
			}
			// At the data stage all old lists have already been replaced. Retry
			// must still refresh column 41 using the saved pending relationships.
			g.source = source
			result, err := g.DeleteArticleRelated(context.Background(), 1001)
			if err != nil || !result.Deleted || !slices.Equal(result.RefreshedColumnIDs, []int64{41}) {
				t.Fatalf("retry = %+v, err = %v", result, err)
			}
		})
	}
}

func TestDeleteArticleWithoutAnyRemainingRelationsIsIdempotent(t *testing.T) {
	g, _ := relatedTestGenerator(t, missingRelationsSource{removedArticleSource{Source: demo.NewSource(), articleID: 1001}})
	for attempt := 0; attempt < 2; attempt++ {
		result, err := g.DeleteArticleRelated(context.Background(), 1001)
		if err != nil || result.Deleted || len(result.RefreshedColumnIDs) != 0 {
			t.Fatalf("delete = %+v, err = %v", result, err)
		}
	}
}

func TestDeleteArticleRecoversMainPageReferencesWithoutLists(t *testing.T) {
	g, cfg := relatedTestGenerator(t, missingRelationsSource{removedArticleSource{Source: demo.NewSource(), articleID: 1001}})
	root := cfg.Site.DistRoot
	if err := publishFile(filepath.Join(root, "news.html"), []byte(`<a href="article/2026/08/1001.html">old news</a>`)); err != nil {
		t.Fatal(err)
	}
	if err := publishFile(filepath.Join(root, "about.html"), []byte("unaffected live about page")); err != nil {
		t.Fatal(err)
	}
	if err := publishFile(filepath.Join(root, "article", "2026", "08", "1001.html"), []byte("old detail")); err != nil {
		t.Fatal(err)
	}
	result, err := g.DeleteArticleRelated(context.Background(), 1001)
	if err != nil || !result.Deleted || !slices.Equal(result.RefreshedPages, []string{"news"}) || !slices.Equal(result.RefreshedColumnIDs, []int64{11, 20, 21, 22, 23, 24}) {
		t.Fatalf("delete = %+v, err = %v", result, err)
	}
	news, err := os.ReadFile(filepath.Join(root, "news.html"))
	if err != nil || strings.Contains(string(news), "1001.html") {
		t.Fatalf("stale news reference: err=%v", err)
	}
	about, err := os.ReadFile(filepath.Join(root, "about.html"))
	if err != nil || string(about) != "unaffected live about page" {
		t.Fatalf("unaffected page replaced: %q, err=%v", about, err)
	}
}

func TestListReferencesArticleMatchesExactInternalDetail(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "list", "41", "1.html")
	for _, tc := range []struct {
		href string
		want bool
	}{
		{"../../article/2026/08/1001.html?x=1&amp;y=2#title", true},
		{"/article/2026/08/1001.html", true},
		{"../../article/2026/08/11001.html", false},
		{"../../article/2026/08/1001.html.bak", false},
		{"https://example.com/article/2026/08/1001.html", false},
		{"//example.com/article/2026/08/1001.html", false},
		{"../../list/1001/1.html", false},
		{"../../../outside/2026/1001.html", false},
	} {
		if err := publishFile(source, []byte(`<a href="`+tc.href+`">article</a>`)); err != nil {
			t.Fatal(err)
		}
		found, err := listReferencesArticle(context.Background(), root, source, 1001)
		if err != nil || found != tc.want {
			t.Fatalf("href=%s found=%v err=%v want=%v", tc.href, found, err, tc.want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := listReferencesArticle(ctx, root, source, 1001); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
