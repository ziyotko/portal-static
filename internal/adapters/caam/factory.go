package caam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	adapterconfig "portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/generator"
	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/platform"
	"portal-static/internal/sources/portalcms"
)

type Factory struct{}

func (Factory) Driver() string { return "caam" }

func (Factory) Build(ctx context.Context, mode platform.Mode, snapshot coreconfig.Snapshot, logger *slog.Logger) (*platform.Runtime, error) {
	switch mode {
	case platform.Preview:
		_, service, err := NewPreview(snapshot, logger)
		if err != nil {
			return nil, err
		}
		return platform.NewRuntime(Operations(service), nil)
	case platform.Production:
		store, err := portalcms.Open(snapshot.Database, snapshot.Site.Timezone)
		if err != nil {
			return nil, err
		}
		productionSnapshot, cleanupTemplates, err := caamDatabaseTemplateSnapshot(ctx, snapshot, store)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		_, validator, err := newProductionWithStore(ctx, productionSnapshot, store, logger)
		if cleanupErr := cleanupTemplates(); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		operations := caamProductionOperations(snapshot, productionSnapshot.Paths.Routes, store, validator, logger)
		return platform.NewRuntime(operations, store.Close)
	default:
		return nil, fmt.Errorf("unsupported CAAM runtime mode %q", mode)
	}
}

func caamProductionOperations(snapshot coreconfig.Snapshot, initialRoutes map[string]string, store *portalcms.Store, validator *generator.SiteGenerator, logger *slog.Logger) httpapi.Operations {
	base := Operations(validator)
	base = portalcms.WithRouteAliases(base, portalcms.RoutePublisher{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Routes: initialRoutes}, portalcms.LegacyCAAMMainFiles())
	base = withCAAMTopics(base, store, snapshot)
	load := func(ctx context.Context) (httpapi.Operations, func() error, error) {
		productionSnapshot, cleanup, err := caamDatabaseTemplateSnapshot(ctx, snapshot, store)
		if err != nil {
			return httpapi.Operations{}, nil, err
		}
		_, service, err := newProductionWithStore(ctx, productionSnapshot, store, logger)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		operations := Operations(service)
		operations = portalcms.WithRouteAliases(operations, portalcms.RoutePublisher{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Routes: productionSnapshot.Paths.Routes}, portalcms.LegacyCAAMMainFiles())
		return operations, cleanup, nil
	}
	return httpapi.ReloadingOperations(base, load)
}

func withCAAMTopics(operations httpapi.Operations, store *portalcms.Store, snapshot coreconfig.Snapshot) httpapi.Operations {
	location, _ := time.LoadLocation(snapshot.Site.Timezone)
	service := portalcms.TopicService{Store: store, AllowedRoot: snapshot.Paths.DistRoot, Location: location}
	operations.GenerateTopics = func(ctx context.Context) (any, error) { return service.GenerateAll(ctx) }
	operations.GenerateTopic = func(ctx context.Context, id int64) (any, error) { return service.Generate(ctx, id) }
	operations.DeleteTopic = func(ctx context.Context, id int64) (any, error) { return service.Delete(ctx, id) }
	return operations
}

func caamDatabaseTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store) (coreconfig.Snapshot, func() error, error) {
	cfg, err := adapterconfig.FromSnapshot(snapshot)
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	binding := func(key, name, templateType string) portalcms.TemplateBinding {
		result := portalcms.TemplateBinding{Key: key, Name: name, Type: templateType}
		if code := snapshot.Paths.TemplateCodes[key]; code != "" {
			result.Code, result.Name = code, ""
		}
		return result
	}
	bindings := []portalcms.TemplateBinding{
		binding("home", cfg.Site.PageName, "home"),
		binding("about", "协会概况", "home"),
		binding("work", "协会工作", "home"),
		binding("stats", "统计数据", "home"),
		binding("members", "会员专区", "home"),
		binding("party", "党建专区", "home"),
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
