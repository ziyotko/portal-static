package caam

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/demo"
	"portal-static/internal/adapters/caam/generator"
	"portal-static/internal/adapters/caam/repository"
	"portal-static/internal/contracts"
	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/sources/portalcms"
)

type runtimeSource interface {
	repository.ArticleSource
	generator.PageSource
	generator.AboutSource
}

func newProductionWithStore(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store, logger *slog.Logger) (config.Config, *generator.SiteGenerator, error) {
	cfg, err := config.FromSnapshot(snapshot)
	if err != nil {
		return config.Config{}, nil, err
	}
	templateID, err := repository.ResolveTemplateIDWithStore(ctx, store, cfg.Site.PageName)
	if err != nil {
		return config.Config{}, nil, err
	}
	source := repository.NewArticleRepositoryWithStore(store, templateID)
	site, err := newSiteGenerator(cfg, source, logger)
	return cfg, site, err
}

func NewPreview(snapshot coreconfig.Snapshot, logger *slog.Logger) (config.Config, *generator.SiteGenerator, error) {
	cfg, err := config.FromSnapshot(snapshot)
	if err != nil {
		return config.Config{}, nil, err
	}
	cfg.Site.DistRoot = snapshot.Paths.PreviewRoot
	site, err := newSiteGenerator(cfg, demo.NewSiteSource(cfg), logger)
	return cfg, site, err
}

func newSiteGenerator(cfg config.Config, source runtimeSource, logger *slog.Logger) (*generator.SiteGenerator, error) {
	distConfig := cfg
	distConfig.Site.OutputRoot = cfg.Site.DistRoot
	distConfig.Site.Output = filepath.Join(cfg.Site.DistRoot, "index.html")
	pages, err := generator.NewPageGenerator(distConfig, source, logger)
	if err != nil {
		return nil, err
	}
	home, err := generator.New(distConfig, source, logger)
	if err != nil {
		return nil, err
	}
	return generator.NewSiteGenerator(cfg, source, source, pages, home, logger), nil
}

var _ runtimeSource = (*repository.ArticleRepository)(nil)
var _ runtimeSource = (*demo.SiteSource)(nil)

func Operations(site *generator.SiteGenerator) httpapi.Operations {
	return withHTTPContract(httpapi.Operations{
		GenerateSite:           func(ctx context.Context) (any, error) { return site.GenerateSite(ctx) },
		GeneratePages:          func(ctx context.Context) (any, error) { return site.GeneratePages(ctx) },
		GenerateAllLists:       func(ctx context.Context) (any, error) { return site.GenerateAllLists(ctx) },
		GenerateAllArticles:    func(ctx context.Context) (any, error) { return site.GenerateAllArticles(ctx) },
		GeneratePage:           func(ctx context.Context, name string) (any, error) { return site.GeneratePage(ctx, name) },
		GenerateList:           func(ctx context.Context, id int64) (any, error) { return site.GenerateList(ctx, id) },
		GenerateListByName:     func(ctx context.Context, name string) (any, error) { return site.GenerateListByName(ctx, name) },
		GenerateArticle:        func(ctx context.Context, id int64) (any, error) { return site.GenerateArticle(ctx, id) },
		DeleteArticle:          func(ctx context.Context, id int64) (any, error) { return site.DeleteArticle(ctx, id) },
		GenerateArticleRelated: func(ctx context.Context, id int64) (any, error) { return site.GenerateArticleRelated(ctx, id) },
		DeleteArticleRelated:   func(ctx context.Context, id int64) (any, error) { return site.DeleteArticleRelated(ctx, id) },
		ValidateOutputPath:     site.ValidateOutputPath,
	})
}

func withHTTPContract(operations httpapi.Operations) httpapi.Operations {
	operations.NormalizePageName = generator.NormalizeStaticPageName
	operations.PageNameError = "页面名必须是：首页、协会概况、协会工作、统计数据、会员专区或党建专区"
	operations.ClassifyError = classifyError
	operations.Messages = httpapi.Messages{
		ListRequired:       "column_name（栏目名称）不能为空",
		InvalidColumnID:    "column_id 必须是正整数",
		ArticleIDRequired:  "id is required",
		InvalidArticleID:   "id must be a positive integer",
		InvalidRefresh:     "refresh must be related or none when provided",
		InvalidGray:        "gray must be the string 1 or 2",
		InvalidOutputPath:  "path must be an absolute non-root directory",
		ContentUnavailable: "content generation unavailable",
	}
	return operations
}

func classifyError(err error) (int, string, bool) {
	switch {
	case errors.Is(err, generator.ErrBusy):
		return http.StatusConflict, "静态化任务正在执行，请稍后重试", true
	case errors.Is(err, generator.ErrColumnNotUnique):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, generator.ErrColumnNotFound):
		return http.StatusNotFound, err.Error(), true
	case errors.Is(err, generator.ErrArticleNotPublished):
		return http.StatusNotFound, "文章不存在、未发布或不属于有效栏目", true
	case errors.Is(err, generator.ErrArticleStillPublished):
		return http.StatusConflict, "文章仍处于可发布状态，请先在数据库下架", true
	case errors.Is(err, contracts.ErrTemplateNotUnique):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, contracts.ErrTemplateNotFound), errors.Is(err, contracts.ErrTopicNotFound):
		return http.StatusNotFound, err.Error(), true
	case errors.Is(err, contracts.ErrInvalidOutputPath):
		return http.StatusBadRequest, err.Error(), true
	default:
		return 0, "", false
	}
}
