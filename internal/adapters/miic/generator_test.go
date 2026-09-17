package miic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portal-static/internal/cms/demo"
	"portal-static/internal/cms/model"
)

func TestMergeArticlesDeduplicatesAndSorts(t *testing.T) {
	now := time.Now()
	items := mergeArticles([][]model.Article{{{ID: 1, PublishTime: now}, {ID: 2, IsTop: true, PublishTime: now.Add(-time.Hour)}}, {{ID: 1, PublishTime: now}}})
	if len(items) != 2 || items[0].ID != 2 || items[1].ID != 1 {
		t.Fatalf("unexpected order: %+v", items)
	}
}

func TestSanitizerRemovesScriptsAndUnsafeURLs(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(cfg, demo.NewSource(), nil)
	if err != nil {
		t.Fatal(err)
	}
	clean := g.sanitizer.Sanitize(`<p onclick="bad()">ok</p><script>alert(1)</script><a href="javascript:bad()">bad</a><video controls src="/uploads/a.mp4"></video>`)
	if strings.Contains(clean, "script") || strings.Contains(clean, "onclick") || strings.Contains(clean, "javascript:") {
		t.Fatalf("unsafe content survived: %s", clean)
	}
	if !strings.Contains(clean, "video") {
		t.Fatalf("allowed video removed: %s", clean)
	}
}

func TestNormalizesUploadURLsToMIICPublicPath(t *testing.T) {
	content := `<p><img src="../../../mic/uploads/article/a.png?size=large#preview"><a href="../../../../miic/uploads/article/a.pdf">附件</a><video poster="/mic/uploads/poster.jpg" src="https://cdn.example.com/video.mp4"></video></p>`
	normalized := normalizeContentMediaURLs(content)
	for _, expected := range []string{
		`src="/miic/uploads/article/a.png?size=large#preview"`,
		`href="/miic/uploads/article/a.pdf"`,
		`poster="/miic/uploads/poster.jpg"`,
		`src="https://cdn.example.com/video.mp4"`,
	} {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("normalized content missing %q: %s", expected, normalized)
		}
	}
	if strings.Contains(normalized, "/mic/uploads/") || strings.Contains(normalized, "../miic/uploads/") {
		t.Fatalf("legacy upload path survived: %s", normalized)
	}
}

func TestMediaURLNormalizesLegacyUploadPrefix(t *testing.T) {
	cfg := Config{Media: defaultMediaConfig()}
	for input, expected := range map[string]string{
		"../../../mic/uploads/cover.png": "/miic/uploads/cover.png",
		"../../miic/uploads/cover.png":   "/miic/uploads/cover.png",
		"/miic/uploads/cover.png":        "/miic/uploads/cover.png",
		"https://cdn.example.com/a.png":  "https://cdn.example.com/a.png",
	} {
		if actual := mediaURL(cfg, input); actual != expected {
			t.Errorf("mediaURL(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestPreviewGeneratesCompleteSite(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	unique := filepath.Base(filepath.Dir(temp)) + "-" + filepath.Base(temp)
	cfg.Site.DistRoot = filepath.Join(cfg.Site.SourceRoot, "dist", unique)
	g, err := New(cfg, demo.NewSource(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(result.Output)
	if result.DurationSeconds < 0.001 || result.GeneratedFiles != result.GeneratedArticles+4 ||
		result.GeneratedDetails != result.GeneratedArticles || result.GeneratedLists != 0 {
		t.Fatalf("unexpected unified generation metrics: %#v", result)
	}
	for _, name := range []string{"news.html", "generated-content.js", "article/2026/08/1001.html"} {
		data, err := os.ReadFile(filepath.Join(result.Output, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	article, _ := os.ReadFile(filepath.Join(result.Output, "article", "2026", "08", "1001.html"))
	if !strings.Contains(string(article), `data-root-prefix="../../../"`) {
		t.Fatal("article root prefix missing")
	}
	if !strings.Contains(string(article), `data-active-root="news"`) {
		t.Fatal("article active navigation root missing")
	}
	if _, err := os.Stat(filepath.Join(result.Output, "list")); !os.IsNotExist(err) {
		t.Fatalf("site generation should not publish independent list pages: %v", err)
	}
	manifest, _ := os.ReadFile(filepath.Join(result.Output, "generated-content.js"))
	if !strings.Contains(string(manifest), `"1001":"article/2026/08/1001.html"`) {
		t.Fatal("legacy article mapping missing")
	}
	if !strings.Contains(string(manifest), `"category":"中心动态"`) {
		t.Fatal("important news did not retain its editorial category")
	}
	articleText := string(article)
	if !strings.Contains(articleText, `class="detail-label">中心动态`) {
		t.Fatal("article did not use preferred editorial category")
	}
}

func TestGenerateAllListsPreservesPublishedRootFiles(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	unique := filepath.Base(filepath.Dir(temp)) + "-" + filepath.Base(temp)
	cfg.Site.DistRoot = filepath.Join(cfg.Site.SourceRoot, "dist", "list-preserve-"+unique)
	defer os.RemoveAll(cfg.Site.DistRoot)

	g, err := New(cfg, demo.NewSource(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.GenerateSite(context.Background()); err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"index.html",
		"news.html",
		"generated-content.js",
		"styles.css",
		"script.js",
		filepath.Join("assets", "miic-logo.png"),
	}
	oldTime := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	contents := make(map[string][]byte, len(paths))
	for _, name := range paths {
		path := filepath.Join(cfg.Site.DistRoot, name)
		contents[name], err = os.ReadFile(path)
		if err != nil {
			t.Fatalf("read published %s: %v", name, err)
		}
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatalf("set published timestamp for %s: %v", name, err)
		}
	}

	result, err := g.GenerateAllLists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.GeneratedLists == 0 {
		t.Fatal("list generation did not produce any list pages")
	}
	for _, name := range paths {
		path := filepath.Join(cfg.Site.DistRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read preserved %s: %v", name, err)
		}
		if string(data) != string(contents[name]) {
			t.Errorf("list generation changed published root file %s", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat preserved %s: %v", name, err)
		}
		if !info.ModTime().Equal(oldTime) {
			t.Errorf("list generation rewrote %s: modtime = %s, want %s", name, info.ModTime(), oldTime)
		}
	}
}

func TestOutputPathRejectsUnsafeSourceChild(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(cfg, demo.NewSource(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.ValidateOutputPath(filepath.Join(cfg.Site.SourceRoot, "published")); !errors.Is(err, ErrInvalidOutputPath) {
		t.Fatalf("ValidateOutputPath error = %v, want ErrInvalidOutputPath", err)
	}
	ctx := WithOptions(context.Background(), filepath.Join(cfg.Site.SourceRoot, "published"), false)
	if _, err := g.GeneratePages(ctx); err == nil {
		t.Fatal("expected unsafe output path error")
	}
}

func TestGrayscaleHTMLCanBeEnabledAndRemoved(t *testing.T) {
	original := []byte("<html><head></head><body></body></html>")
	gray := grayscaleHTML(original, true)
	if !strings.Contains(string(gray), `id="miic-grayscale"`) {
		t.Fatal("grayscale marker missing")
	}
	color := grayscaleHTML(gray, false)
	if strings.Contains(string(color), "miic-grayscale") {
		t.Fatal("grayscale marker was not removed")
	}
}

type failingSource struct{ Source }

func (failingSource) FetchAllArticles(context.Context) ([]model.Article, error) {
	return nil, errors.New("snapshot failed")
}

func TestFailedSiteGenerationKeepsPublishedDirectory(t *testing.T) {
	cfg, err := LoadLegacyConfig(filepath.Join("..", "..", "..", "miic-backend-seed", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Site.DistRoot = filepath.Join(cfg.Site.SourceRoot, "dist", "atomic-failure-test")
	defer os.RemoveAll(cfg.Site.DistRoot)
	if err := os.MkdirAll(cfg.Site.DistRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(cfg.Site.DistRoot, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("old release"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := New(cfg, failingSource{Source: demo.NewSource()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.GenerateSite(context.Background()); err == nil {
		t.Fatal("expected generation failure")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "old release" {
		t.Fatalf("published directory changed: %q %v", data, err)
	}
}
