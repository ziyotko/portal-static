package miic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	templateID, err := repository.ResolveTemplateIDWithStore(ctx, store, productionConfig.Site.PageName)
	if err != nil {
		_ = cleanupTemplates()
		_ = store.Close()
		return nil, err
	}
	validator, err := New(productionConfig, repository.NewWithStore(store, templateID), logger)
	if cleanupErr := cleanupTemplates(); cleanupErr != nil {
		err = errors.Join(err, cleanupErr)
	}
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	operations := miicProductionOperations(snapshot, productionSnapshot.Paths.Routes, cfg, store, validator, logger)
	return platform.NewRuntime(operations, store.Close)
}

func miicProductionOperations(snapshot coreconfig.Snapshot, initialRoutes map[string]string, cfg Config, store *portalcms.Store, validator *Generator, logger *slog.Logger) httpapi.Operations {
	base := httpapi.OperationsForGenerator(validator, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
	base = portalcms.WithRouteAliases(base, portalcms.RoutePublisher{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Routes: initialRoutes}, portalcms.LegacyMainFiles("news", "business", "platforms", "about"))
	base = withMIICTopics(base, store, snapshot)
	load := func(ctx context.Context) (httpapi.Operations, func() error, error) {
		productionSnapshot, cleanup, err := miicDatabaseTemplateSnapshot(ctx, snapshot, cfg, store)
		if err != nil {
			return httpapi.Operations{}, nil, err
		}
		productionConfig, err := FromSnapshot(productionSnapshot)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		templateID, err := repository.ResolveTemplateIDWithStore(ctx, store, productionConfig.Site.PageName)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		service, err := New(productionConfig, repository.NewWithStore(store, templateID), logger)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		operations := httpapi.OperationsForGenerator(service, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
		operations = portalcms.WithRouteAliases(operations, portalcms.RoutePublisher{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Routes: productionSnapshot.Paths.Routes}, portalcms.LegacyMainFiles("news", "business", "platforms", "about"))
		return operations, cleanup, nil
	}
	return httpapi.ReloadingOperations(base, load)
}

func withMIICTopics(operations httpapi.Operations, store *portalcms.Store, snapshot coreconfig.Snapshot) httpapi.Operations {
	location, _ := time.LoadLocation(snapshot.Site.Timezone)
	service := portalcms.TopicService{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Location: location}
	operations.GenerateTopics = func(ctx context.Context) (any, error) { return service.GenerateAll(ctx) }
	operations.GenerateTopic = func(ctx context.Context, id int64) (any, error) { return service.Generate(ctx, id) }
	operations.DeleteTopic = func(ctx context.Context, id int64) (any, error) { return service.Delete(ctx, id) }
	return operations
}

func miicDatabaseTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, cfg Config, store *portalcms.Store) (coreconfig.Snapshot, func() error, error) {
	binding := func(key, name, templateType string) portalcms.TemplateBinding {
		result := portalcms.TemplateBinding{Key: key, Name: name, Type: templateType}
		if code := snapshot.Paths.TemplateCodes[key]; code != "" {
			result.Code, result.Name = code, ""
		}
		return result
	}
	bindings := []portalcms.TemplateBinding{
		binding("news", cfg.Site.PageName, "home"),
		binding("business", cfg.Business.PageName, "home"),
		binding("platforms", "服务平台", "home"),
		binding("about", cfg.About.PageName, "home"),
		binding("list", "栏目", "column"),
		binding("article", "详情", "detail"),
	}
	records, err := store.LoadBoundTemplates(ctx, bindings)
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	if err := portalcms.ValidateRoutePlan(ctx, store, records); err != nil {
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
	snapshot.Paths.Routes = make(map[string]string, len(records))
	for key, record := range records {
		snapshot.Paths.Routes[key] = record.RoutePath
	}
	return snapshot, cleanup, nil
}
