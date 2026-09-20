package miic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"portal-static/internal/adapters/miic/demo"
	"portal-static/internal/adapters/miic/repository"
	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/platform"
	"portal-static/internal/sources/portalcms"
)

type Factory struct{}

func (Factory) Driver() string { return "miic" }

func (Factory) Build(ctx context.Context, mode platform.Mode, snapshot coreconfig.Snapshot, logger *slog.Logger) (*platform.Runtime, error) {
	cfg, err := FromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	switch mode {
	case platform.Preview:
		cfg.Site.DistRoot = cfg.Site.PreviewRoot
		service, err := New(cfg, demo.NewSource(), logger)
		if err != nil {
			return nil, err
		}
		operations := httpapi.OperationsForGenerator(service, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
		return platform.NewRuntime(operations, nil)
	case platform.Production:
		return buildMIICProductionRuntime(ctx, snapshot, cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported MIIC runtime mode %q", mode)
	}
}

func buildMIICProductionRuntime(ctx context.Context, snapshot coreconfig.Snapshot, cfg Config, logger *slog.Logger) (*platform.Runtime, error) {
	store, err := portalcms.Open(snapshot.Database, snapshot.Site.Timezone)
	if err != nil {
		return nil, err
	}
	productionSnapshot, cleanupTemplates, err := miicDatabaseTemplateSnapshot(ctx, snapshot, cfg, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	productionConfig, err := FromSnapshot(productionSnapshot)
	if err != nil {
		_ = cleanupTemplates()
		_ = store.Close()
		return nil, err
	}
	pageID, err := repository.ResolvePageIDWithStore(ctx, store, productionConfig.Site.PageName)
	if err != nil {
		_ = cleanupTemplates()
		_ = store.Close()
		return nil, err
	}
	validator, err := New(productionConfig, repository.NewWithStore(store, pageID), logger)
	if cleanupErr := cleanupTemplates(); cleanupErr != nil {
		err = errors.Join(err, cleanupErr)
	}
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	operations := miicProductionOperations(snapshot, cfg, store, validator, logger)
	return platform.NewRuntime(operations, store.Close)
}

func miicProductionOperations(snapshot coreconfig.Snapshot, cfg Config, store *portalcms.Store, validator *Generator, logger *slog.Logger) httpapi.Operations {
	base := httpapi.OperationsForGenerator(validator, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
	load := func(ctx context.Context) (httpapi.Operations, func() error, error) {
		productionSnapshot, cleanup, err := miicDatabaseTemplateSnapshot(ctx, snapshot, cfg, store)
		if err != nil {
			return httpapi.Operations{}, nil, err
		}
		productionConfig, err := FromSnapshot(productionSnapshot)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		pageID, err := repository.ResolvePageIDWithStore(ctx, store, productionConfig.Site.PageName)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		service, err := New(productionConfig, repository.NewWithStore(store, pageID), logger)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		operations := httpapi.OperationsForGenerator(service, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
		return operations, cleanup, nil
	}
	return httpapi.ReloadingOperations(base, load)
}

func miicDatabaseTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, cfg Config, store *portalcms.Store) (coreconfig.Snapshot, func() error, error) {
	bindings := []portalcms.TemplateBinding{
		{Key: "news", PageName: cfg.Site.PageName, PageType: "home", TemplateType: "home"},
		{Key: "business", PageName: cfg.Business.PageName, PageType: "home", TemplateType: "home"},
		{Key: "platforms", PageName: "服务平台", PageType: "home", TemplateType: "home"},
		{Key: "about", PageName: cfg.About.PageName, PageType: "home", TemplateType: "home"},
		{Key: "list", PageType: "column", TemplateType: "column"},
		{Key: "article", PageType: "detail", TemplateType: "detail"},
	}
	records, err := store.LoadBoundTemplates(ctx, bindings)
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	paths, cleanup, err := portalcms.MaterializeTemplates(records)
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	templates := make(map[string]string, len(snapshot.Paths.Templates)+len(paths))
	for key, path := range snapshot.Paths.Templates {
		templates[key] = path
	}
	for key, path := range paths {
		templates[key] = path
	}
	snapshot.Paths.Templates = templates
	return snapshot, cleanup, nil
}
