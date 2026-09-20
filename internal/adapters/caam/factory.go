package caam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
		operations := caamProductionOperations(snapshot, store, validator, logger)
		return platform.NewRuntime(operations, store.Close)
	default:
		return nil, fmt.Errorf("unsupported CAAM runtime mode %q", mode)
	}
}

func caamProductionOperations(snapshot coreconfig.Snapshot, store *portalcms.Store, validator *generator.SiteGenerator, logger *slog.Logger) httpapi.Operations {
	base := Operations(validator)
	load := func(ctx context.Context) (httpapi.Operations, func() error, error) {
		productionSnapshot, cleanup, err := caamDatabaseTemplateSnapshot(ctx, snapshot, store)
		if err != nil {
			return httpapi.Operations{}, nil, err
		}
		_, service, err := newProductionWithStore(ctx, productionSnapshot, store, logger)
		if err != nil {
			return httpapi.Operations{}, nil, errors.Join(err, cleanup())
		}
		return Operations(service), cleanup, nil
	}
	return httpapi.ReloadingOperations(base, load)
}

func caamDatabaseTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store) (coreconfig.Snapshot, func() error, error) {
	cfg, err := adapterconfig.FromSnapshot(snapshot)
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	bindings := []portalcms.TemplateBinding{
		{Key: "home", PageName: cfg.Site.PageName, PageType: "home", TemplateType: "home"},
		{Key: "about", PageName: "协会概况", PageType: "home", TemplateType: "home"},
		{Key: "work", PageName: "协会工作", PageType: "home", TemplateType: "home"},
		{Key: "stats", PageName: "统计数据", PageType: "home", TemplateType: "home"},
		{Key: "members", PageName: "会员专区", PageType: "home", TemplateType: "home"},
		{Key: "party", PageName: "党建专区", PageType: "home", TemplateType: "home"},
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
