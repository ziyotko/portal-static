package generator

import (
	"testing"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

func TestBuildTabsUsesFirstArticleCover(t *testing.T) {
	t.Parallel()
	slots := []config.SlotConfig{{
		Key:           "work-file",
		Title:         "协会文件",
		Href:          "../first-version/list.html",
		FallbackCover: "assets/slice/work-photo.png",
	}}
	data := map[string][]model.Article{
		"work-file": {
			{ID: "first", Title: "第一条新闻", Cover: "uploads/first.jpg"},
			{ID: "second", Title: "第二条新闻", Cover: "uploads/second.jpg"},
		},
	}

	tabs := buildTabs(slots, data, newViewBuilder("", time.UTC), nil)
	if len(tabs) != 1 {
		t.Fatalf("tab count = %d, want 1", len(tabs))
	}
	if tabs[0].Cover != "/uploads/first.jpg" {
		t.Fatalf("cover = %q, want first article cover", tabs[0].Cover)
	}
	if tabs[0].CoverAlt != "第一条新闻" {
		t.Fatalf("cover alt = %q, want first article title", tabs[0].CoverAlt)
	}
}

func TestHomepageArticleHrefUsesCurrentDetailPage(t *testing.T) {
	t.Parallel()
	builder := newViewBuilder("", time.UTC)

	view := builder.article(model.Article{
		ID:          "news 42",
		ColumnID:    77,
		Type:        model.ArticleTypeContent,
		Title:       "首页文章",
		URL:         "../first-version/article.html?id=news-42",
		PublishTime: time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC),
	}, 0, "")
	if !view.Clickable || view.ExternalLink || view.Href != "article/2026/07/news%2042.html?from=home" {
		t.Fatalf("unexpected homepage detail link: %#v", view)
	}

	external := builder.article(model.Article{
		ID:    "external",
		Type:  model.ArticleTypeContent,
		Title: "外部文章",
		URL:   "https://example.com/news",
	}, 0, "")
	if !external.Clickable || !external.ExternalLink || external.Href != "https://example.com/news" {
		t.Fatalf("unexpected external link: %#v", external)
	}
}

func TestUnsupportedArticleTypeNeverCreatesLink(t *testing.T) {
	article := model.Article{
		ID: "5235393", Type: 4, URL: "https://example.com/unsupported",
		PublishTime: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if href, external, ok := homepageArticleHref(article, time.UTC); ok || external || href != "" {
		t.Fatalf("unsupported type produced homepage link: href=%q external=%v ok=%v", href, external, ok)
	}
	builder := &PageGenerator{location: time.UTC}
	if view := builder.innerArticle(article, 40, "../../"); view.Href != "" || view.ExternalLink {
		t.Fatalf("unsupported type produced list link: %#v", view)
	}
}

func TestSlotListHrefUsesFlatPageFile(t *testing.T) {
	t.Parallel()
	href := slotListHref(config.SlotConfig{}, nil, 42)
	if href != "list/42/1.html" {
		t.Fatalf("list href = %q, want flat page file", href)
	}
}
