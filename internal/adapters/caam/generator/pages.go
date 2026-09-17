package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
	"portal-static/internal/adapters/caam/repository"
	"portal-static/internal/contracts"

	"github.com/microcosm-cc/bluemonday"
	nethtml "golang.org/x/net/html"
)

var (
	ErrColumnNotFound        = repository.ErrColumnNotFound
	ErrColumnNotUnique       = repository.ErrColumnNotUnique
	ErrArticleNotPublished   = repository.ErrArticleNotPublished
	ErrArticleStillPublished = contracts.ErrArticleStillPublished
)

type PageSource interface {
	ResolveColumnID(context.Context, config.SlotConfig) (int64, error)
	ResolveGlobalColumnID(context.Context, string) (int64, error)
	FetchByColumn(context.Context, config.SlotConfig) ([]model.Article, error)
	FetchColumns(context.Context) ([]model.Column, error)
	FetchAllArticles(context.Context) ([]model.Article, error)
	FetchArticleColumnMappingsBatch(context.Context, int64, int) ([]model.ArticleColumnMapping, error)
	FetchListArticlesByIDs(context.Context, []int64) ([]model.Article, error)
	FetchDetailArticlesByIDs(context.Context, []int64) ([]model.Article, error)
	FetchArticleColumns(context.Context, int64) ([]model.Column, error)
	FetchArticleRelatedColumns(context.Context, int64) ([]model.Column, error)
	FetchColumnArticles(context.Context, int64) (model.Column, []model.Article, error)
	FetchArticle(context.Context, int64) (model.Column, model.Article, error)
	FetchLinksByColumn(context.Context, config.SlotConfig) ([]model.Article, error)
}

type ListResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	ColumnID         int64     `json:"column_id"`
	TotalItems       int       `json:"total_items"`
	TotalPages       int       `json:"total_pages"`
	PageSize         int       `json:"page_size"`
	Output           string    `json:"output"`
}

type AllListsResult struct {
	GeneratedAt      time.Time    `json:"generated_at"`
	DurationSeconds  float64      `json:"duration_seconds"`
	GeneratedFiles   int          `json:"generated_files"`
	GeneratedDetails int          `json:"generated_details"`
	GeneratedLists   int          `json:"generated_lists"`
	Generated        int          `json:"generated_columns"`
	TotalItems       int          `json:"total_items"`
	TotalPages       int          `json:"total_pages"`
	Output           string       `json:"output"`
	Lists            []ListResult `json:"lists"`
}

func (r AllListsResult) GeneratedFileCount() int { return r.GeneratedFiles }

type ArticleResult struct {
	GeneratedAt        time.Time `json:"generated_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ColumnID           int64     `json:"column_id"`
	ArticleID          int64     `json:"article_id"`
	Output             string    `json:"output"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type DeleteArticleResult struct {
	DeletedAt          time.Time `json:"deleted_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ArticleID          int64     `json:"article_id"`
	Deleted            bool      `json:"deleted"`
	DeletedPaths       []string  `json:"deleted_paths"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type HomeArticlesResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	Generated        int       `json:"generated"`
	SkippedExternal  int       `json:"skipped_external"`
	SkippedData      int       `json:"skipped_data"`
	SkippedInvalid   int       `json:"skipped_invalid"`
	Output           string    `json:"output"`
}

type AllArticlesResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	Generated        int       `json:"generated_articles"`
	SkippedExternal  int       `json:"skipped_external"`
	SkippedData      int       `json:"skipped_data"`
	SkippedInvalid   int       `json:"skipped_invalid"`
	Output           string    `json:"output"`
}

func (r AllArticlesResult) GeneratedFileCount() int { return r.GeneratedFiles }

type PageGenerator struct {
	cfg        config.Config
	source     PageSource
	logger     *slog.Logger
	location   *time.Location
	staleAfter time.Duration
	sanitizer  *bluemonday.Policy
}

type InnerArticleView struct {
	ID           string
	Title        string
	Href         string
	Source       string
	Author       string
	DateISO      string
	DateCN       string
	ExternalLink bool
}

type PaginationView struct {
	Number   int
	Href     string
	Current  bool
	Ellipsis bool
}

type ListPageData struct {
	GeneratedAt string
	RootPrefix  string
	ColumnID    int64
	Title       string
	Items       []InnerArticleView
	Page        int
	PageSize    int
	TotalItems  int
	TotalPages  int
	Previous    string
	Next        string
	Pagination  []PaginationView
	FooterLinks []LinkGroupView
}

type ArticlePageData struct {
	GeneratedAt string
	RootPrefix  string
	ColumnID    int64
	ColumnTitle string
	Article     InnerArticleView
	Content     template.HTML
	FooterLinks []LinkGroupView
}

func NewPageGenerator(cfg config.Config, source PageSource, logger *slog.Logger) (*PageGenerator, error) {
	location, err := time.LoadLocation(cfg.Site.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load site timezone: %w", err)
	}
	staleAfter, err := time.ParseDuration(cfg.Site.LockStaleAfter)
	if err != nil {
		return nil, fmt.Errorf("parse lock stale duration: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("video", "source")
	policy.AllowAttrs("src", "poster", "controls", "preload", "width", "height").OnElements("video")
	policy.AllowAttrs("src", "type").OnElements("source")
	policy.AllowURLSchemes("http", "https")
	policy.AllowRelativeURLs(true)
	policy.RequireNoReferrerOnLinks(true)
	return &PageGenerator{
		cfg: cfg, source: source, logger: logger, location: location,
		staleAfter: staleAfter, sanitizer: policy,
	}, nil
}

func (g *PageGenerator) GenerateList(ctx context.Context, columnID int64) (ListResult, error) {
	started := time.Now()
	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return ListResult{}, err
	}
	result, err := g.generateList(ctx, columnID, footerLinks)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = result.TotalPages
		result.GeneratedLists = result.TotalPages
	}
	return result, err
}

func (g *PageGenerator) GenerateListByName(ctx context.Context, columnName string) (ListResult, error) {
	started := time.Now()
	columnName = strings.TrimSpace(columnName)
	columnID, err := g.source.ResolveGlobalColumnID(ctx, columnName)
	if err != nil {
		return ListResult{}, err
	}
	result, err := g.GenerateList(ctx, columnID)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
	}
	return result, err
}

func (g *PageGenerator) generateList(ctx context.Context, columnID int64, footerLinks []LinkGroupView) (ListResult, error) {
	column, articles, err := g.source.FetchColumnArticles(ctx, columnID)
	if err != nil {
		return ListResult{}, err
	}
	totalPages := int(math.Ceil(float64(len(articles)) / float64(g.cfg.Site.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	targetParent := filepath.Join(g.cfg.Site.OutputRoot, "list")
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return ListResult{}, fmt.Errorf("create list output root: %w", err)
	}
	target := filepath.Join(targetParent, strconv.FormatInt(columnID, 10))
	lock, err := acquireFileLock(target+".lock", g.staleAfter)
	if err != nil {
		return ListResult{}, err
	}
	defer lock.release()

	staging, err := os.MkdirTemp(targetParent, ".column-"+strconv.FormatInt(columnID, 10)+"-*")
	if err != nil {
		return ListResult{}, fmt.Errorf("create list staging directory: %w", err)
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	tpl, err := template.ParseFiles(g.cfg.Site.ListTemplate)
	if err != nil {
		return ListResult{}, fmt.Errorf("parse list template: %w", err)
	}
	title := g.columnTitle(column)
	generatedAt := time.Now().In(g.location)
	for page := 1; page <= totalPages; page++ {
		start := (page - 1) * g.cfg.Site.PageSize
		end := start + g.cfg.Site.PageSize
		if start > len(articles) {
			start = len(articles)
		}
		if end > len(articles) {
			end = len(articles)
		}
		items := make([]InnerArticleView, 0, end-start)
		for _, article := range articles[start:end] {
			item := g.innerArticle(article, columnID, "../../")
			if !item.ExternalLink {
				item.Href = withListOrigin(item.Href, columnID, title, page)
			}
			items = append(items, item)
		}
		data := ListPageData{
			GeneratedAt: generatedAt.Format(time.RFC3339),
			RootPrefix:  "../../",
			ColumnID:    columnID,
			Title:       title,
			Items:       items,
			Page:        page,
			PageSize:    g.cfg.Site.PageSize,
			TotalItems:  len(articles),
			TotalPages:  totalPages,
			Pagination:  buildPagination(page, totalPages),
			FooterLinks: footerLinks,
		}
		if page > 1 {
			data.Previous = strconv.Itoa(page-1) + ".html"
		}
		if page < totalPages {
			data.Next = strconv.Itoa(page+1) + ".html"
		}
		var rendered bytes.Buffer
		if err := tpl.Execute(&rendered, data); err != nil {
			return ListResult{}, fmt.Errorf("render list page %d: %w", page, err)
		}
		if err := validateInnerPage(rendered.Bytes(), "class=\"full-news-list\""); err != nil {
			return ListResult{}, fmt.Errorf("validate list page %d: %w", page, err)
		}
		if err := os.WriteFile(filepath.Join(staging, strconv.Itoa(page)+".html"), rendered.Bytes(), 0o644); err != nil {
			return ListResult{}, fmt.Errorf("write list page %d: %w", page, err)
		}
	}
	if err := replaceDirectory(staging, target); err != nil {
		return ListResult{}, err
	}
	cleanupStaging = false
	result := ListResult{
		GeneratedAt: generatedAt, GeneratedFiles: totalPages, GeneratedLists: totalPages,
		ColumnID: columnID, TotalItems: len(articles),
		TotalPages: totalPages, PageSize: g.cfg.Site.PageSize, Output: target,
	}
	g.logger.Info("栏目列表生成成功", "column_id", columnID, "items", len(articles), "pages", totalPages, "output", target)
	return result, nil
}

func (g *PageGenerator) generateAllListsLegacy(ctx context.Context) (AllListsResult, error) {
	started := time.Now()
	listRoot := filepath.Join(g.cfg.Site.OutputRoot, "list")
	if err := os.MkdirAll(listRoot, 0o755); err != nil {
		return AllListsResult{}, fmt.Errorf("create list output root: %w", err)
	}
	batchLock, err := acquireFileLock(filepath.Join(listRoot, ".all-lists.lock"), g.staleAfter)
	if err != nil {
		return AllListsResult{}, err
	}
	defer batchLock.release()

	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return AllListsResult{}, err
	}
	columns, err := g.source.FetchColumns(ctx)
	if err != nil {
		return AllListsResult{}, err
	}
	uniqueColumns := make([]model.Column, 0, len(columns))
	seen := make(map[int64]struct{})
	for _, column := range columns {
		if column.ID <= 0 {
			continue
		}
		if _, exists := seen[column.ID]; exists {
			continue
		}
		seen[column.ID] = struct{}{}
		uniqueColumns = append(uniqueColumns, column)
	}

	results := make([]ListResult, 0, len(uniqueColumns))
	totalItems := 0
	totalPages := 0
	for _, column := range uniqueColumns {
		g.logger.Info("生成栏目列表", "column_id", column.ID, "column", column.Name)
		result, err := g.generateList(ctx, column.ID, footerLinks)
		if err != nil {
			return AllListsResult{}, err
		}
		results = append(results, result)
		totalItems += result.TotalItems
		totalPages += result.TotalPages
	}
	generatedAt := time.Now().In(g.location)
	result := AllListsResult{
		GeneratedAt:     generatedAt,
		DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:  totalPages,
		GeneratedLists:  totalPages,
		Generated:       len(results),
		TotalItems:      totalItems,
		TotalPages:      totalPages,
		Output:          listRoot,
		Lists:           results,
	}
	g.logger.Info("首页栏目列表批量生成成功",
		"generated_columns", result.Generated,
		"total_items", result.TotalItems,
		"total_pages", result.TotalPages,
		"duration_seconds", result.DurationSeconds,
		"output", result.Output,
	)
	return result, nil
}

func (g *PageGenerator) GenerateArticle(ctx context.Context, articleID int64) (ArticleResult, error) {
	started := time.Now()
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	lockRoot := filepath.Join(articleRoot, ".locks")
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		return ArticleResult{}, fmt.Errorf("create article output root: %w", err)
	}
	lock, err := acquireFileLock(filepath.Join(lockRoot, strconv.FormatInt(articleID, 10)+".lock"), g.staleAfter)
	if err != nil {
		return ArticleResult{}, err
	}
	defer lock.release()

	column, article, err := g.source.FetchArticle(ctx, articleID)
	if err != nil {
		if errors.Is(err, ErrArticleNotPublished) || errors.Is(err, ErrColumnNotFound) {
			return ArticleResult{}, removeStaleArticle(articleRoot, articleID, err)
		}
		return ArticleResult{}, err
	}
	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return ArticleResult{}, err
	}
	output, err := g.articleOutput(article)
	if err != nil {
		return ArticleResult{}, err
	}
	result, err := g.publishPreparedArticle(column, []model.Article{article}, article, footerLinks, column.ID, articleID, output)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = 1
		result.GeneratedDetails = 1
	}
	return result, err
}

func (g *PageGenerator) DeleteArticle(ctx context.Context, articleID int64) (DeleteArticleResult, error) {
	if err := ctx.Err(); err != nil {
		return DeleteArticleResult{}, err
	}
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	lockRoot := filepath.Join(articleRoot, ".locks")
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		return DeleteArticleResult{}, fmt.Errorf("create article output root: %w", err)
	}
	lock, err := acquireFileLock(filepath.Join(lockRoot, strconv.FormatInt(articleID, 10)+".lock"), g.staleAfter)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	defer lock.release()

	if err := ctx.Err(); err != nil {
		return DeleteArticleResult{}, err
	}
	deletedPaths, err := removeOldArticleArtifacts(articleRoot, articleID, "")
	if err != nil {
		return DeleteArticleResult{}, fmt.Errorf("delete article page: %w", err)
	}
	result := DeleteArticleResult{
		DeletedAt: time.Now().In(g.location), ArticleID: articleID,
		Deleted: len(deletedPaths) > 0, DeletedPaths: deletedPaths,
	}
	g.logger.Info("文章静态页删除完成", "article_id", articleID, "deleted", result.Deleted, "paths", deletedPaths)
	return result, nil
}

func (g *PageGenerator) GenerateHomeArticles(ctx context.Context) (HomeArticlesResult, error) {
	started := time.Now()
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	if err := os.MkdirAll(articleRoot, 0o755); err != nil {
		return HomeArticlesResult{}, fmt.Errorf("create article output root: %w", err)
	}
	batchLock, err := acquireFileLock(filepath.Join(articleRoot, ".home-articles.lock"), g.staleAfter)
	if err != nil {
		return HomeArticlesResult{}, err
	}
	defer batchLock.release()

	type selectedArticle struct {
		columnID int64
		column   model.Column
		article  model.Article
	}
	selected := make(map[string]selectedArticle)
	skippedExternal := 0
	skippedData := 0
	skippedInvalid := 0
	for _, slot := range g.cfg.ContentSlots() {
		articles, err := g.source.FetchByColumn(ctx, slot)
		if err != nil {
			return HomeArticlesResult{}, err
		}
		for _, article := range articles {
			if article.Type == model.ArticleTypeData {
				skippedData++
				continue
			}
			_, external, linkOK := homepageArticleHref(article, g.location)
			if !linkOK {
				skippedInvalid++
				continue
			}
			if external {
				skippedExternal++
				continue
			}
			articleID, err := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
			if err != nil || articleID <= 0 || article.ColumnID <= 0 {
				skippedInvalid++
				continue
			}
			key := strconv.FormatInt(articleID, 10)
			if _, exists := selected[key]; !exists {
				selected[key] = selectedArticle{
					columnID: article.ColumnID,
					column:   model.Column{ID: article.ColumnID, Name: slot.Name},
					article:  article,
				}
			}
		}
	}

	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return HomeArticlesResult{}, err
	}
	columns := make(map[int64][]selectedArticle)
	for _, item := range selected {
		columns[item.columnID] = append(columns[item.columnID], item)
	}
	generated := 0
	for columnID, items := range columns {
		column := items[0].column
		siblings := make([]model.Article, 0, len(items))
		for _, item := range items {
			siblings = append(siblings, item.article)
		}
		for _, item := range items {
			articleID, _ := strconv.ParseInt(item.article.ID, 10, 64)
			lockRoot := filepath.Join(articleRoot, ".locks")
			if err := os.MkdirAll(lockRoot, 0o755); err != nil {
				return HomeArticlesResult{}, fmt.Errorf("create article lock root: %w", err)
			}
			lock, err := acquireFileLock(filepath.Join(lockRoot, strconv.FormatInt(articleID, 10)+".lock"), g.staleAfter)
			if err != nil {
				return HomeArticlesResult{}, err
			}
			output, outputErr := g.articleOutput(item.article)
			if outputErr != nil {
				lock.release()
				return HomeArticlesResult{}, outputErr
			}
			_, publishErr := g.publishPreparedArticle(
				column, siblings, item.article, footerLinks, columnID, articleID, output,
			)
			lock.release()
			if publishErr != nil {
				return HomeArticlesResult{}, publishErr
			}
			generated++
		}
	}
	generatedAt := time.Now().In(g.location)
	result := HomeArticlesResult{
		GeneratedAt:      generatedAt,
		DurationSeconds:  math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:   generated,
		GeneratedDetails: generated,
		Generated:        generated,
		SkippedExternal:  skippedExternal,
		SkippedData:      skippedData,
		SkippedInvalid:   skippedInvalid,
		Output:           articleRoot,
	}
	g.logger.Info("首页详情批量生成成功",
		"generated", result.Generated,
		"skipped_external", result.SkippedExternal,
		"skipped_data", result.SkippedData,
		"skipped_invalid", result.SkippedInvalid,
		"duration_seconds", result.DurationSeconds,
		"output", result.Output,
	)
	return result, nil
}

func (g *PageGenerator) generateAllArticlesLegacy(ctx context.Context) (AllArticlesResult, error) {
	started := time.Now()
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	lockRoot := filepath.Join(articleRoot, ".locks")
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		return AllArticlesResult{}, fmt.Errorf("create article output root: %w", err)
	}
	batchLock, err := acquireFileLock(filepath.Join(articleRoot, ".all-articles.lock"), g.staleAfter)
	if err != nil {
		return AllArticlesResult{}, err
	}
	defer batchLock.release()

	articles, err := g.source.FetchAllArticles(ctx)
	if err != nil {
		return AllArticlesResult{}, err
	}
	unique := make(map[int64]model.Article)
	skippedInvalid := 0
	for _, article := range articles {
		articleID, parseErr := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
		if parseErr != nil || articleID <= 0 || article.ColumnID <= 0 || article.PublishTime.IsZero() {
			skippedInvalid++
			continue
		}
		if _, exists := unique[articleID]; !exists {
			unique[articleID] = article
		}
	}

	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return AllArticlesResult{}, err
	}
	grouped := make(map[int64][]model.Article)
	for _, article := range unique {
		grouped[article.ColumnID] = append(grouped[article.ColumnID], article)
	}
	generated := 0
	for columnID, selected := range grouped {
		column, siblings, err := g.source.FetchColumnArticles(ctx, columnID)
		if err != nil {
			return AllArticlesResult{}, err
		}
		for _, article := range selected {
			articleID, _ := strconv.ParseInt(article.ID, 10, 64)
			lock, err := acquireFileLock(filepath.Join(lockRoot, strconv.FormatInt(articleID, 10)+".lock"), g.staleAfter)
			if err != nil {
				return AllArticlesResult{}, err
			}
			output, outputErr := g.articleOutput(article)
			if outputErr != nil {
				lock.release()
				return AllArticlesResult{}, outputErr
			}
			_, publishErr := g.publishPreparedArticle(
				column, siblings, article, footerLinks, columnID, articleID, output,
			)
			lock.release()
			if publishErr != nil {
				return AllArticlesResult{}, publishErr
			}
			generated++
		}
	}
	generatedAt := time.Now().In(g.location)
	result := AllArticlesResult{
		GeneratedAt:      generatedAt,
		DurationSeconds:  math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:   generated,
		GeneratedDetails: generated,
		Generated:        generated,
		SkippedInvalid:   skippedInvalid,
		Output:           articleRoot,
	}
	g.logger.Info("全部文章详情批量生成成功",
		"generated_articles", result.Generated,
		"skipped_invalid", result.SkippedInvalid,
		"duration_seconds", result.DurationSeconds,
		"output", result.Output,
	)
	return result, nil
}

func (g *PageGenerator) publishPreparedArticle(
	column model.Column,
	articles []model.Article,
	article model.Article,
	footerLinks []LinkGroupView,
	columnID, articleID int64,
	output string,
) (ArticleResult, error) {
	started := time.Now()
	articleIndex := -1
	for i := range articles {
		if articles[i].ID == strconv.FormatInt(articleID, 10) {
			articleIndex = i
			break
		}
	}
	if articleIndex < 0 {
		return ArticleResult{}, removeStaleArticle(filepath.Join(g.cfg.Site.OutputRoot, "article"), articleID, ErrArticleNotPublished)
	}
	data := ArticlePageData{
		GeneratedAt: time.Now().In(g.location).Format(time.RFC3339),
		RootPrefix:  "../../../",
		ColumnID:    columnID,
		ColumnTitle: g.columnTitle(column),
		Article:     g.innerArticle(article, columnID, "../../../"),
		Content:     g.sanitizeContent(article.Content),
		FooterLinks: footerLinks,
	}

	tpl, err := template.ParseFiles(g.cfg.Site.ArticleTemplate)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("parse article template: %w", err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return ArticleResult{}, fmt.Errorf("render article page: %w", err)
	}
	if err := validateInnerPage(rendered.Bytes(), "class=\"article-body\""); err != nil {
		return ArticleResult{}, fmt.Errorf("validate article page: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return ArticleResult{}, fmt.Errorf("create article directory: %w", err)
	}
	if err := publishFile(output, rendered.Bytes()); err != nil {
		return ArticleResult{}, err
	}
	if _, err := removeOldArticleArtifacts(filepath.Join(g.cfg.Site.OutputRoot, "article"), articleID, output); err != nil {
		return ArticleResult{}, err
	}
	generatedAt := time.Now().In(g.location)
	result := ArticleResult{
		GeneratedAt: generatedAt, DurationSeconds: elapsedSeconds(started), GeneratedFiles: 1, GeneratedDetails: 1,
		ColumnID: columnID, ArticleID: articleID, Output: output,
	}
	g.logger.Info("文章详情生成成功", "column_id", columnID, "article_id", articleID, "output", output)
	return result, nil
}

func (g *PageGenerator) articleOutput(article model.Article) (string, error) {
	if strings.TrimSpace(article.ID) == "" || article.PublishTime.IsZero() {
		return "", errors.New("article id and publish time are required for static archive path")
	}
	published := article.PublishTime.In(g.location)
	return filepath.Join(
		g.cfg.Site.OutputRoot,
		"article",
		published.Format("2006"),
		published.Format("01"),
		strings.TrimSpace(article.ID)+".html",
	), nil
}

func removeStaleArticle(articleRoot string, articleID int64, cause error) error {
	if _, err := removeOldArticleArtifacts(articleRoot, articleID, ""); err != nil {
		return fmt.Errorf("remove stale article page: %w", err)
	}
	return cause
}

func removeOldArticleArtifacts(articleRoot string, articleID int64, keepOutput string) ([]string, error) {
	rootEntries, err := os.ReadDir(articleRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect article output root: %w", err)
	}
	id := strconv.FormatInt(articleID, 10)
	keepOutput = filepath.Clean(keepOutput)
	removed := make([]string, 0, 1)
	for _, rootEntry := range rootEntries {
		if !rootEntry.IsDir() {
			continue
		}
		rootDir := filepath.Join(articleRoot, rootEntry.Name())

		legacyDir := filepath.Join(rootDir, id)
		if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
			if err := os.RemoveAll(legacyDir); err != nil {
				return nil, fmt.Errorf("remove legacy article directory: %w", err)
			}
			removed = append(removed, legacyDir)
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect legacy article directory: %w", statErr)
		}

		if len(rootEntry.Name()) != 4 {
			continue
		}
		year, yearErr := strconv.Atoi(rootEntry.Name())
		if yearErr != nil || year < 1900 || year > 9999 {
			continue
		}
		monthEntries, readErr := os.ReadDir(rootDir)
		if readErr != nil {
			return nil, fmt.Errorf("inspect article year directory: %w", readErr)
		}
		for _, monthEntry := range monthEntries {
			if !monthEntry.IsDir() || len(monthEntry.Name()) != 2 {
				continue
			}
			month, monthErr := strconv.Atoi(monthEntry.Name())
			if monthErr != nil || month < 1 || month > 12 {
				continue
			}
			candidate := filepath.Join(rootDir, monthEntry.Name(), id+".html")
			if keepOutput != "" && filepath.Clean(candidate) == keepOutput {
				continue
			}
			if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("remove archived article file: %w", err)
			} else if err == nil {
				removed = append(removed, candidate)
			}
		}
	}
	return removed, nil
}

func (g *PageGenerator) fetchFooterLinks(ctx context.Context) ([]LinkGroupView, error) {
	data := make(map[string][]model.Article)
	for _, slot := range g.cfg.Columns.FooterLinks {
		links, err := g.source.FetchLinksByColumn(ctx, slot)
		if errors.Is(err, ErrColumnNotFound) {
			g.logger.Warn("首页友链栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
			links = nil
		} else if err != nil {
			return nil, err
		}
		data[slot.Key] = links
	}
	return buildLinkGroups(g.cfg.Columns.FooterLinks, data, newViewBuilderForConfig(g.cfg, g.location)), nil
}

func (g *PageGenerator) columnTitle(column model.Column) string {
	for _, slot := range g.cfg.ContentSlots() {
		if slot.Name == column.Name && strings.TrimSpace(slot.Title) != "" {
			return strings.TrimSpace(slot.Title)
		}
	}
	return strings.TrimSpace(column.Name)
}

func (g *PageGenerator) innerArticle(article model.Article, columnID int64, internalPrefix string) InnerArticleView {
	view := InnerArticleView{
		ID: article.ID, Title: strings.TrimSpace(article.Title),
		Source: strings.TrimSpace(article.Source), Author: strings.TrimSpace(article.Author),
	}
	if !article.PublishTime.IsZero() {
		published := article.PublishTime.In(g.location)
		view.DateISO = published.Format("2006-01-02")
		view.DateCN = published.Format("2006年1月2日")
	}
	if !model.IsSupportedArticleType(article.Type) {
		return view
	}
	if href, external, ok := safeHref(article.URL); ok && external {
		view.Href = href
		view.ExternalLink = true
	} else if !model.HasStaticDetail(article.Type) {
		return view
	} else if href, ok := articleArchiveHref(article, g.location); ok {
		view.Href = internalPrefix + href
	}
	return view
}

func withListOrigin(href string, columnID int64, columnTitle string, page int) string {
	if strings.TrimSpace(href) == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	query := parsed.Query()
	query.Set("from", "list")
	query.Set("column_id", strconv.FormatInt(columnID, 10))
	query.Set("page", strconv.Itoa(page))
	query.Set("column_title", strings.TrimSpace(columnTitle))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (g *PageGenerator) sanitizeContent(content string) template.HTML {
	sanitized := strings.TrimSpace(g.sanitizer.Sanitize(content))
	if sanitized == "" {
		sanitized = "<p>暂无正文内容</p>"
	}
	return template.HTML(g.prefixContentMediaURLs(sanitized))
}

func (g *PageGenerator) prefixContentMediaURLs(content string) string {
	tokenizer := nethtml.NewTokenizer(strings.NewReader(content))
	builder := newViewBuilderForConfig(g.cfg, g.location)
	var output strings.Builder
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case nethtml.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return output.String()
			}
			return content
		case nethtml.StartTagToken, nethtml.SelfClosingTagToken:
			token := tokenizer.Token()
			tag := strings.ToLower(token.Data)
			for index := range token.Attr {
				key := strings.ToLower(token.Attr[index].Key)
				isMediaSource := (tag == "img" || tag == "video" || tag == "source") && key == "src"
				isVideoPoster := tag == "video" && key == "poster"
				isAttachment := tag == "a" && key == "href"
				if !isMediaSource && !isVideoPoster && !isAttachment {
					continue
				}
				normalized := normalizeUploadPath(token.Attr[index].Val)
				isUploadPath := normalized == "/uploads" || strings.HasPrefix(normalized, "/uploads/") ||
					normalized == "/caam/uploads" || strings.HasPrefix(normalized, "/caam/uploads/")
				if isUploadPath || isMediaSource || isVideoPoster {
					token.Attr[index].Val = builder.resolveCover(normalized, normalized)
				}
			}
			output.WriteString(token.String())
		default:
			output.Write(tokenizer.Raw())
		}
	}
}

func buildPagination(current, total int) []PaginationView {
	result := make([]PaginationView, 0, 9)
	last := 0
	for page := 1; page <= total; page++ {
		show := page == 1 || page == total || (page >= current-2 && page <= current+2)
		if !show {
			continue
		}
		if last > 0 && page-last > 1 {
			result = append(result, PaginationView{Ellipsis: true})
		}
		result = append(result, PaginationView{
			Number: page, Href: strconv.Itoa(page) + ".html", Current: page == current,
		})
		last = page
	}
	return result
}

func validateInnerPage(page []byte, marker string) error {
	text := string(page)
	if len(page) < 800 {
		return errors.New("rendered page is unexpectedly small")
	}
	for _, required := range []string{"<!DOCTYPE html>", "data-generated-at=", marker} {
		if !strings.Contains(text, required) {
			return fmt.Errorf("rendered page is missing required marker %q", required)
		}
	}
	return nil
}

func replaceDirectory(staging, target string) error {
	if err := normalizeStaticTreePermissions(staging); err != nil {
		return fmt.Errorf("set staged static permissions: %w", err)
	}
	backup := target + ".bak"
	_ = os.RemoveAll(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		hadTarget = true
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup current list directory: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect current list directory: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("replace list directory: %w", err)
	}
	if hadTarget {
		_ = os.RemoveAll(backup)
	}
	return nil
}

// normalizeStaticTreePermissions makes an atomically published tree readable
// by the web-server user. MkdirTemp intentionally creates its root as 0700,
// which must not be carried into the live static directory by os.Rename.
func normalizeStaticTreePermissions(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		mode := os.FileMode(0o644)
		if entry.IsDir() {
			mode = 0o755
		} else if !entry.Type().IsRegular() {
			return nil
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("chmod %s to %04o: %w", path, mode, err)
		}
		return nil
	})
}

func publishFile(output string, page []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(output), ".article-*.tmp")
	if err != nil {
		return fmt.Errorf("create article temporary file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		_ = temp.Close()
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(page); err != nil {
		return fmt.Errorf("write article temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync article temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close article temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("set article permissions: %w", err)
	}
	backup := output + ".bak"
	_ = os.Remove(backup)
	hadOutput := false
	if _, err := os.Stat(output); err == nil {
		hadOutput = true
		if err := os.Rename(output, backup); err != nil {
			return fmt.Errorf("backup current article: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect current article: %w", err)
	}
	if err := os.Rename(tempPath, output); err != nil {
		if hadOutput {
			_ = os.Rename(backup, output)
		}
		return fmt.Errorf("replace article: %w", err)
	}
	cleanup = false
	if hadOutput {
		_ = os.Remove(backup)
	}
	return nil
}
