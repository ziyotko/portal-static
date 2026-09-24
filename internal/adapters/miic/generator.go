package miic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/miic/model"
	"portal-static/internal/adapters/miic/repository"
	"portal-static/internal/contracts"
	"portal-static/internal/core/media"

	"github.com/microcosm-cc/bluemonday"
)

type Generator struct {
	cfg       Config
	source    Source
	logger    *slog.Logger
	location  *time.Location
	stale     time.Duration
	sanitizer *bluemonday.Policy
	media     *media.Resolver
}

var (
	ErrInvalidOutputPath     = contracts.ErrInvalidOutputPath
	ErrArticleStillPublished = contracts.ErrArticleStillPublished
)

func New(cfg Config, source Source, logger *slog.Logger) (*Generator, error) {
	location, err := time.LoadLocation(cfg.Site.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	stale, err := time.ParseDuration(cfg.Site.LockStaleAfter)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	resolver, err := media.New(cfg.Media)
	if err != nil {
		return nil, err
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("video", "source")
	policy.AllowAttrs("src", "poster", "controls", "preload", "width", "height").OnElements("video")
	policy.AllowAttrs("src", "type").OnElements("source")
	policy.AllowURLSchemes("http", "https")
	policy.AllowRelativeURLs(true)
	policy.RequireNoReferrerOnLinks(true)
	return &Generator{cfg: cfg, source: source, logger: logger, location: location, stale: stale, sanitizer: policy, media: resolver}, nil
}

func NormalizePageName(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "资讯动态", "news":
		return "news", true
	case "核心业务", "business":
		return "business", true
	case "服务平台", "platforms":
		return "platforms", true
	case "关于我们", "about":
		return "about", true
	default:
		return "", false
	}
}

func (g *Generator) outputRoot(ctx context.Context) (string, error) {
	raw := strings.TrimSpace(optionsFrom(ctx).OutputPath)
	if raw == "" {
		raw = g.cfg.Site.DistRoot
	}
	return g.resolveOutputPath(raw)
}

// ValidateOutputPath validates an API-supplied output path before a background
// job is accepted, so invalid paths fail synchronously with HTTP 400.
func (g *Generator) ValidateOutputPath(raw string) error {
	_, err := g.resolveOutputPath(raw)
	return err
}

func (g *Generator) resolveOutputPath(raw string) (string, error) {
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%w: path must be absolute", ErrInvalidOutputPath)
	}
	target, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	target = filepath.Clean(target)
	if filepath.Dir(target) == target {
		return "", fmt.Errorf("%w: path must not be a volume root", ErrInvalidOutputPath)
	}
	if !safeOutputPath(g.cfg.Site.DistRoot, target) {
		return "", fmt.Errorf("%w: path must be the configured dist_root or one of its descendants", ErrInvalidOutputPath)
	}
	return target, nil
}

func safeOutputPath(allowedRoot, target string) bool {
	allowedRoot, _ = filepath.Abs(allowedRoot)
	target, _ = filepath.Abs(target)
	allowedRoot, target = filepath.Clean(allowedRoot), filepath.Clean(target)
	if filepath.Dir(target) == target {
		return false
	}
	rel, err := filepath.Rel(allowedRoot, target)
	if err != nil {
		return false
	}
	if !(rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))) {
		return false
	}
	realRoot, ok := resolveProspectivePath(allowedRoot)
	if !ok {
		return false
	}
	realTarget, ok := resolveProspectivePath(target)
	if !ok {
		return false
	}
	realRel, err := filepath.Rel(realRoot, realTarget)
	return err == nil && (realRel == "." || (realRel != ".." && !strings.HasPrefix(realRel, ".."+string(filepath.Separator)) && !filepath.IsAbs(realRel)))
}

func resolveProspectivePath(path string) (string, bool) {
	path = filepath.Clean(path)
	ancestor := path
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", false
		}
		ancestor = next
	}
	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		info, statErr := os.Lstat(ancestor)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
		realAncestor = ancestor
	}
	rel, err := filepath.Rel(ancestor, path)
	if err != nil {
		return "", false
	}
	return filepath.Join(realAncestor, rel), true
}

func (g *Generator) GenerateSite(ctx context.Context) (GenerationResult, error) {
	started := time.Now()
	target, err := g.outputRoot(ctx)
	if err != nil {
		return GenerationResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return GenerationResult{}, err
	}
	lock, err := acquireLock(target+".lock", g.stale)
	if err != nil {
		return GenerationResult{}, err
	}
	defer lock.release()
	staging, err := os.MkdirTemp(filepath.Dir(target), ".miic-site-*")
	if err != nil {
		return GenerationResult{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(staging)
		}
	}()
	reportProgress(ctx, Progress{Stage: "复制静态资源"})
	if err := copyScaffold(g.cfg.Site.SourceRoot, staging); err != nil {
		return GenerationResult{}, fmt.Errorf("copy scaffold: %w", err)
	}
	reportProgress(ctx, Progress{Stage: "生成文章详情"})
	articles, err := g.generateAllArticlesAt(ctx, staging)
	if err != nil {
		return GenerationResult{}, err
	}
	reportProgress(ctx, Progress{Stage: "生成主页面"})
	if err := g.renderNews(ctx, staging, optionsFrom(ctx).Grayscale); err != nil {
		return GenerationResult{}, err
	}
	for _, name := range []string{"business", "platforms", "about"} {
		if err := g.renderStaticMainPage(staging, name, false); err != nil {
			return GenerationResult{}, err
		}
	}
	if err := g.writeGeneratedContent(ctx, staging); err != nil {
		return GenerationResult{}, err
	}
	if optionsFrom(ctx).Grayscale {
		if err := grayscaleTree(staging, true); err != nil {
			return GenerationResult{}, err
		}
	}
	reportProgress(ctx, Progress{Stage: "校验内部链接"})
	if err := validateSite(staging); err != nil {
		return GenerationResult{}, err
	}
	reportProgress(ctx, Progress{Stage: "发布整站"})
	if err := replaceDirectory(staging, target); err != nil {
		return GenerationResult{}, err
	}
	published = true
	return GenerationResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: articles + 4, GeneratedDetails: articles, GeneratedLists: 0,
		Output: target, GeneratedPages: 4, GeneratedArticles: articles, Gray: grayCode(optionsFrom(ctx).Grayscale),
	}, nil
}

func (g *Generator) GeneratePages(ctx context.Context) (GenerationResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return GenerationResult{}, err
	}
	lock, err := g.lockFor(root, "pages")
	if err != nil {
		return GenerationResult{}, err
	}
	defer lock.release()
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return GenerationResult{}, err
	}
	if err := g.renderNews(ctx, root, optionsFrom(ctx).Grayscale); err != nil {
		return GenerationResult{}, err
	}
	if err := g.writeGeneratedContent(ctx, root); err != nil {
		return GenerationResult{}, err
	}
	for _, name := range []string{"business.html", "platforms.html", "about.html"} {
		normalized := strings.TrimSuffix(name, ".html")
		if err := g.renderStaticMainPage(root, normalized, optionsFrom(ctx).Grayscale); err != nil {
			return GenerationResult{}, err
		}
	}
	return GenerationResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: 4, GeneratedDetails: 0, GeneratedLists: 0,
		Output: root, GeneratedPages: 4, Gray: grayCode(optionsFrom(ctx).Grayscale),
	}, nil
}

func (g *Generator) GeneratePage(ctx context.Context, name string) (GenerationResult, error) {
	normalized, ok := NormalizePageName(name)
	if !ok {
		return GenerationResult{}, fmt.Errorf("unsupported page %q", name)
	}
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return GenerationResult{}, err
	}
	lock, err := g.lockFor(root, "page-"+normalized)
	if err != nil {
		return GenerationResult{}, err
	}
	defer lock.release()
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return GenerationResult{}, err
	}
	if normalized == "news" {
		err = g.renderNews(ctx, root, optionsFrom(ctx).Grayscale)
	} else {
		err = g.renderStaticMainPage(root, normalized, optionsFrom(ctx).Grayscale)
	}
	if err != nil {
		return GenerationResult{}, err
	}
	if err := g.writeGeneratedContent(ctx, root); err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: 1, GeneratedDetails: 0, GeneratedLists: 0,
		Output: filepath.Join(root, normalized+".html"), GeneratedPages: 1, Gray: grayCode(optionsFrom(ctx).Grayscale),
	}, nil
}

func (g *Generator) GenerateAllArticles(ctx context.Context) (GenerationResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return GenerationResult{}, err
	}
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return GenerationResult{}, err
	}
	lock, err := g.lockFor(root, "articles")
	if err != nil {
		return GenerationResult{}, err
	}
	defer lock.release()
	count, err := g.generateAllArticlesAt(ctx, root)
	if err != nil {
		return GenerationResult{}, err
	}
	if err := g.writeGeneratedContent(ctx, root); err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: count, GeneratedDetails: count, GeneratedLists: 0,
		Output: filepath.Join(root, "article"), GeneratedArticles: count,
	}, nil
}

func (g *Generator) GenerateAllLists(ctx context.Context) (GenerationResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return GenerationResult{}, err
	}
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return GenerationResult{}, err
	}
	lock, err := g.lockFor(root, "lists")
	if err != nil {
		return GenerationResult{}, err
	}
	defer lock.release()
	columns, pages, err := g.generateAllListsAt(ctx, root)
	if err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: pages, GeneratedDetails: 0, GeneratedLists: pages,
		Output: filepath.Join(root, "list"), GeneratedColumns: columns, TotalPages: pages,
	}, nil
}

func (g *Generator) GenerateListByName(ctx context.Context, name string) (ListResult, error) {
	id, err := g.source.ResolveGlobalColumnID(ctx, name)
	if err != nil {
		return ListResult{}, err
	}
	return g.GenerateList(ctx, id)
}

func (g *Generator) GenerateList(ctx context.Context, id int64) (ListResult, error) {
	root, err := g.outputRoot(ctx)
	if err != nil {
		return ListResult{}, err
	}
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return ListResult{}, err
	}
	lock, err := g.lockFor(root, "list-"+strconv.FormatInt(id, 10))
	if err != nil {
		return ListResult{}, err
	}
	defer lock.release()
	return g.generateListAt(ctx, root, id)
}

func (g *Generator) GenerateArticle(ctx context.Context, id int64) (ArticleResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := ensureScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return ArticleResult{}, err
	}
	lock, err := g.lockFor(root, "article-"+strconv.FormatInt(id, 10))
	if err != nil {
		return ArticleResult{}, err
	}
	defer lock.release()
	column, article, err := g.source.FetchArticle(ctx, id)
	if err != nil {
		return ArticleResult{}, err
	}
	if _, external := safeExternal(article.URL); external {
		return ArticleResult{}, errors.New("external articles do not have local detail pages")
	}
	if !model.HasStaticDetail(article.Type) {
		return ArticleResult{}, repository.ErrArticleNotPublished
	}
	if preferred, _, contextErr := g.categoryContext(ctx); contextErr != nil {
		return ArticleResult{}, contextErr
	} else if selected, exists := preferred[id]; exists {
		column = selected
	}
	path, err := g.renderArticle(root, column, article)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := g.writeGeneratedContent(ctx, root); err != nil {
		return ArticleResult{}, err
	}
	return ArticleResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: 1, GeneratedDetails: 1, GeneratedLists: 0,
		ArticleID: id, ColumnID: column.ID, Output: path,
	}, nil
}

func (g *Generator) DeleteArticle(ctx context.Context, id int64) (DeleteArticleResult, error) {
	root, err := g.outputRoot(ctx)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	pattern := filepath.Join(root, "article", "*", "*", strconv.FormatInt(id, 10)+".html")
	paths, _ := filepath.Glob(pattern)
	deleted := []string{}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return DeleteArticleResult{}, err
		}
		deleted = append(deleted, path)
	}
	return DeleteArticleResult{DeletedAt: time.Now().In(g.location), ArticleID: id, Deleted: len(deleted) > 0, DeletedPaths: deleted}, nil
}

func (g *Generator) lockFor(root, name string) (*fileLock, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return acquireLock(filepath.Join(root, "."+name+".lock"), g.stale)
}

func grayCode(value bool) string {
	if value {
		return "1"
	}
	return "2"
}

func (g *Generator) loadNews(ctx context.Context) ([]ArticleView, []CategoryView, error) {
	columns, err := g.source.FetchPageColumns(ctx, g.cfg.Site.PageName, 0)
	if err != nil {
		return nil, nil, err
	}
	excluded := map[string]bool{strings.TrimSpace(g.cfg.News.Important.Name): true}
	for _, name := range g.cfg.News.ExcludeColumns {
		excluded[strings.TrimSpace(name)] = true
	}
	var categories []CategoryView
	var views []ArticleView
	for _, column := range columns {
		if excluded[strings.TrimSpace(column.Name)] {
			continue
		}
		key := strings.TrimSpace(column.Code)
		if key == "" {
			key = strconv.FormatInt(column.ID, 10)
		}
		categories = append(categories, CategoryView{Key: key, Title: column.Name})
		_, items, err := g.source.FetchColumnArticles(ctx, column.ID)
		if err != nil {
			return nil, nil, err
		}
		for index, item := range items {
			view := makeViewWithResolver(g.cfg, g.media, column.Name, item)
			view.CategoryKey = key
			view.Feature = index == 0
			views = append(views, view)
		}
	}
	return views, categories, nil
}

func (g *Generator) renderNews(ctx context.Context, root string, gray bool) error {
	items, categories, err := g.loadNews(ctx)
	if err != nil {
		return err
	}
	tpl, err := template.ParseFiles(g.cfg.Site.NewsTemplate)
	if err != nil {
		return fmt.Errorf("parse news template: %w", err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, NewsPageData{GeneratedAt: time.Now().In(g.location).Format(time.RFC3339), Items: items, Categories: categories}); err != nil {
		return err
	}
	return publishFile(filepath.Join(root, "news.html"), grayscaleHTML(out.Bytes(), gray))
}

func (g *Generator) renderStaticMainPage(root, normalized string, gray bool) error {
	var source string
	switch normalized {
	case "business":
		source = g.cfg.Site.BusinessTemplate
	case "platforms":
		source = g.cfg.Site.PlatformsTemplate
	case "about":
		source = g.cfg.Site.AboutTemplate
	default:
		return fmt.Errorf("unsupported static main page %q", normalized)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read %s template: %w", normalized, err)
	}
	return publishFile(filepath.Join(root, normalized+".html"), grayscaleHTML(data, gray))
}

func (g *Generator) generateAllArticlesAt(ctx context.Context, root string) (int, error) {
	items, err := g.source.FetchAllArticles(ctx)
	if err != nil {
		return 0, err
	}
	parent := filepath.Join(root, "article")
	staging, err := os.MkdirTemp(root, ".articles-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(staging)
	count := 0
	preferredColumns, _, err := g.categoryContext(ctx)
	if err != nil {
		return 0, err
	}
	for index, item := range items {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		reportProgress(ctx, Progress{Stage: "生成文章详情", Processed: index, Total: len(items), CurrentArticleID: item.ID, GeneratedFiles: count})
		if _, external := safeExternal(item.URL); external {
			continue
		}
		if !model.HasStaticDetail(item.Type) {
			continue
		}
		column, article, err := g.source.FetchArticle(ctx, item.ID)
		if errors.Is(err, repository.ErrArticleNotPublished) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if preferred, exists := preferredColumns[item.ID]; exists {
			column = preferred
		}
		if _, err := g.renderArticle(staging, column, article); err != nil {
			return 0, err
		}
		count++
	}
	_ = os.RemoveAll(parent)
	if err := os.Rename(filepath.Join(staging, "article"), parent); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, os.MkdirAll(parent, 0o755)
		}
		return 0, err
	}
	reportProgress(ctx, Progress{Stage: "生成文章详情", Processed: len(items), Total: len(items), GeneratedFiles: count})
	return count, nil
}

func (g *Generator) renderArticle(root string, column model.Column, article model.Article) (string, error) {
	view := makeViewWithResolver(g.cfg, g.media, column.Name, article)
	if view.Cover != "" && !strings.HasPrefix(view.Cover, "/") && !strings.HasPrefix(view.Cover, "http://") && !strings.HasPrefix(view.Cover, "https://") {
		view.Cover = "../../../" + strings.TrimPrefix(view.Cover, "./")
	}
	view.Content = template.HTML(g.media.RewriteHTML(g.sanitizer.Sanitize(article.Content)))
	tpl, err := template.ParseFiles(g.cfg.Site.ArticleTemplate)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	backHref, backLabel, activeRoot := "../../../news.html", "返回资讯动态", "news"
	if column.Name == g.cfg.About.RecruitmentName {
		backHref, backLabel = "../../../about.html#recruitment", "返回招聘信息"
		activeRoot = "about"
	} else if column.Name == g.cfg.About.DisclosureName {
		backHref, backLabel = "../../../about.html#disclosure", "返回信息公开"
		activeRoot = "about"
	} else if column.ParentID != 0 {
		backHref, backLabel = "../../../business.html", "返回核心业务"
		activeRoot = "business"
	}
	if err := tpl.Execute(&out, ArticlePageData{GeneratedAt: time.Now().In(g.location).Format(time.RFC3339), RootPrefix: "../../../", ActiveRoot: activeRoot, Column: column, Article: view, BackHref: backHref, BackLabel: backLabel}); err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(articleRelativePath(article)))
	if err := publishFile(target, out.Bytes()); err != nil {
		return "", err
	}
	return target, nil
}

func (g *Generator) generateAllListsAt(ctx context.Context, root string) (int, int, error) {
	columns, err := g.source.FetchColumns(ctx)
	if err != nil {
		return 0, 0, err
	}
	parent := filepath.Join(root, "list")
	staging, err := os.MkdirTemp(root, ".lists-*")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(staging)
	pages := 0
	for index, column := range columns {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		result, err := g.generateListAt(ctx, staging, column.ID)
		if err != nil {
			return 0, 0, err
		}
		pages += result.TotalPages
		reportProgress(ctx, Progress{Stage: "生成栏目列表", Processed: index + 1, Total: len(columns), CurrentColumnID: column.ID, GeneratedFiles: pages})
	}
	_ = os.RemoveAll(parent)
	if err := os.Rename(filepath.Join(staging, "list"), parent); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, os.MkdirAll(parent, 0o755)
		}
		return 0, 0, err
	}
	return len(columns), pages, nil
}

func (g *Generator) generateListAt(ctx context.Context, root string, id int64) (ListResult, error) {
	started := time.Now()
	column, items, err := g.source.FetchColumnArticles(ctx, id)
	if err != nil {
		return ListResult{}, err
	}
	totalPages := int(math.Ceil(float64(len(items)) / float64(g.cfg.Site.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	target := filepath.Join(root, "list", strconv.FormatInt(id, 10))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return ListResult{}, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), ".column-*")
	if err != nil {
		return ListResult{}, err
	}
	defer os.RemoveAll(staging)
	tpl, err := template.ParseFiles(g.cfg.Site.ListTemplate)
	if err != nil {
		return ListResult{}, err
	}
	for pageNumber := 1; pageNumber <= totalPages; pageNumber++ {
		start := (pageNumber - 1) * g.cfg.Site.PageSize
		end := start + g.cfg.Site.PageSize
		if start > len(items) {
			start = len(items)
		}
		if end > len(items) {
			end = len(items)
		}
		views := make([]ArticleView, 0, end-start)
		for _, item := range items[start:end] {
			view := makeViewWithResolver(g.cfg, g.media, column.Name, item)
			if !view.External {
				view.Href = "../../" + view.Href
			}
			views = append(views, view)
		}
		data := ListPageData{GeneratedAt: time.Now().In(g.location).Format(time.RFC3339), RootPrefix: "../../", Column: column, Items: views, Page: pageNumber, TotalPages: totalPages}
		if pageNumber > 1 {
			data.Previous = strconv.Itoa(pageNumber-1) + ".html"
		}
		if pageNumber < totalPages {
			data.Next = strconv.Itoa(pageNumber+1) + ".html"
		}
		for n := 1; n <= totalPages; n++ {
			data.Pages = append(data.Pages, n)
		}
		var out bytes.Buffer
		if err := tpl.Execute(&out, data); err != nil {
			return ListResult{}, err
		}
		if err := publishFile(filepath.Join(staging, strconv.Itoa(pageNumber)+".html"), out.Bytes()); err != nil {
			return ListResult{}, err
		}
	}
	if err := replaceDirectory(staging, target); err != nil {
		return ListResult{}, err
	}
	return ListResult{
		GeneratedAt: time.Now().In(g.location), DurationSeconds: roundDuration(started),
		GeneratedFiles: totalPages, GeneratedDetails: 0, GeneratedLists: totalPages,
		ColumnID: id, TotalItems: len(items), TotalPages: totalPages, PageSize: g.cfg.Site.PageSize, Output: target,
	}, nil
}

func (g *Generator) writeGeneratedContent(ctx context.Context, root string) error {
	_, important, err := g.source.FetchByColumnName(ctx, g.cfg.News.Important.Name, g.cfg.News.Important.Limit)
	if errors.Is(err, repository.ErrColumnNotFound) {
		important = nil
		err = nil
	}
	if err != nil {
		return err
	}
	articles, err := g.source.FetchAllArticles(ctx)
	if err != nil {
		return err
	}
	type importantItem struct {
		Category string `json:"category"`
		Date     string `json:"date"`
		Title    string `json:"title"`
		Summary  string `json:"summary"`
		Image    string `json:"image"`
		ImageAlt string `json:"imageAlt"`
		Href     string `json:"href"`
		External bool   `json:"external,omitempty"`
	}
	type serviceItem struct {
		ArticleID int64  `json:"articleId"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Lead      string `json:"lead"`
		Image     string `json:"image"`
		ImageAlt  string `json:"imageAlt"`
		Content   string `json:"content"`
	}
	type aboutItem struct {
		ArticleID int64  `json:"articleId"`
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		Date      string `json:"date"`
		Href      string `json:"href"`
		External  bool   `json:"external,omitempty"`
	}
	payload := struct {
		Generated    bool                   `json:"generated"`
		Important    []importantItem        `json:"importantNews"`
		ArticlePaths map[string]string      `json:"articlePaths"`
		ServiceItems map[string]serviceItem `json:"serviceDetails"`
		Recruitment  []aboutItem            `json:"recruitment"`
		Disclosure   []aboutItem            `json:"disclosure"`
	}{Generated: true, ArticlePaths: map[string]string{}, ServiceItems: map[string]serviceItem{}}
	_, categoryLabels, err := g.categoryContext(ctx)
	if err != nil {
		return err
	}
	for _, article := range important {
		category := categoryLabels[article.ID]
		if category == "" {
			category = "资讯动态"
		}
		view := makeViewWithResolver(g.cfg, g.media, category, article)
		payload.Important = append(payload.Important, importantItem{Category: view.Category, Date: view.DateDot, Title: view.Title, Summary: view.Summary, Image: view.Cover, ImageAlt: view.Title, Href: view.Href, External: view.External})
	}
	for _, article := range articles {
		if _, external := safeExternal(article.URL); !external && model.HasStaticDetail(article.Type) {
			payload.ArticlePaths[strconv.FormatInt(article.ID, 10)] = articleRelativePath(article)
		}
	}
	parents, err := g.source.FetchPageColumns(ctx, g.cfg.Business.PageName, 0)
	if errors.Is(err, repository.ErrTemplateNotFound) {
		parents = nil
		err = nil
	}
	if err != nil {
		return err
	}
	for _, parent := range parents {
		children, childErr := g.source.FetchPageColumns(ctx, g.cfg.Business.PageName, parent.ID)
		if childErr != nil {
			return childErr
		}
		for _, child := range children {
			if strings.TrimSpace(child.Code) == "" {
				g.logger.Warn("核心业务子栏目缺少 code，跳过详情映射", "column", child.Name, "id", child.ID)
				continue
			}
			_, items, fetchErr := g.source.FetchColumnArticles(ctx, child.ID)
			if fetchErr != nil {
				return fetchErr
			}
			if len(items) == 0 {
				continue
			}
			article := items[0]
			view := makeViewWithResolver(g.cfg, g.media, parent.Name, article)
			payload.ServiceItems[child.Code] = serviceItem{ArticleID: article.ID, Category: parent.Name, Title: article.Title, Lead: article.Summary, Image: view.Cover, ImageAlt: article.Title, Content: g.media.RewriteHTML(g.sanitizer.Sanitize(article.Content))}
		}
	}
	loadAbout := func(columnName string) ([]aboutItem, error) {
		_, items, fetchErr := g.source.FetchPageColumnArticles(ctx, g.cfg.About.PageName, columnName, 0)
		if errors.Is(fetchErr, repository.ErrColumnNotFound) || errors.Is(fetchErr, repository.ErrTemplateNotFound) {
			return nil, nil
		}
		if fetchErr != nil {
			return nil, fetchErr
		}
		result := make([]aboutItem, 0, len(items))
		for _, article := range items {
			view := makeViewWithResolver(g.cfg, g.media, columnName, article)
			href, external := safeExternal(article.URL)
			if href == "" {
				attachments, attachmentErr := g.source.FetchAttachments(ctx, article.ID)
				if attachmentErr != nil {
					return nil, attachmentErr
				}
				if len(attachments) > 0 {
					href = g.media.Resolve(attachments[0].URL)
					_, external = safeExternal(href)
				}
			}
			if href == "" {
				href = view.Href
				external = false
			}
			result = append(result, aboutItem{ArticleID: article.ID, Title: article.Title, Summary: article.Summary, Date: view.DateISO, Href: href, External: external})
		}
		return result, nil
	}
	payload.Recruitment, err = loadAbout(g.cfg.About.RecruitmentName)
	if err != nil {
		return err
	}
	payload.Disclosure, err = loadAbout(g.cfg.About.DisclosureName)
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return publishFile(filepath.Join(root, "generated-content.js"), append([]byte("window.MIIC_STATIC_CONTENT = "), append(data, []byte(";\n")...)...))
}

func (g *Generator) categoryContext(ctx context.Context) (map[int64]model.Column, map[int64]string, error) {
	columns := map[int64]model.Column{}
	labels := map[int64]string{}
	pageColumns, err := g.source.FetchPageColumns(ctx, g.cfg.Site.PageName, 0)
	if err != nil {
		return nil, nil, err
	}
	excluded := map[string]bool{strings.TrimSpace(g.cfg.News.Important.Name): true}
	for _, name := range g.cfg.News.ExcludeColumns {
		excluded[strings.TrimSpace(name)] = true
	}
	for _, column := range pageColumns {
		if excluded[strings.TrimSpace(column.Name)] {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(column.Code), "latest") {
			continue
		}
		_, articles, err := g.source.FetchColumnArticles(ctx, column.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, article := range articles {
			if _, exists := columns[article.ID]; !exists {
				columns[article.ID] = column
				labels[article.ID] = column.Name
			}
		}
	}
	return columns, labels, nil
}
