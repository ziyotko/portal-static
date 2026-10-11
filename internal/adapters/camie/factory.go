package camie

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/demo"
	"portal-static/internal/adapters/camie/generator"
	"portal-static/internal/adapters/camie/repository"
	"portal-static/internal/contracts"
	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/platform"
	"portal-static/internal/sources/portalcms"
)

type Factory struct{}

func (Factory) Driver() string { return "camie" }

func (Factory) Build(ctx context.Context, mode platform.Mode, snapshot coreconfig.Snapshot, _ *slog.Logger) (*platform.Runtime, error) {
	switch mode {
	case platform.Preview:
		cfg, err := config.FromSnapshot(snapshot)
		if err != nil {
			return nil, err
		}
		cfg.Site.DistRoot = cfg.Site.PreviewRoot
		cfg.Site.AllowedOutputRoot = cfg.Site.PreviewRoot
		service := NewService(generator.New(cfg, demo.NewSource()), cfg.Site.PageSize)
		return platform.NewRuntime(operations(service), nil)
	case platform.Production:
		cfg, err := config.FromSnapshot(snapshot)
		if err != nil {
			return nil, err
		}
		store, err := portalcms.Open(snapshot.Database, snapshot.Site.Timezone)
		if err != nil {
			return nil, err
		}
		initial, cleanup, err := productionTemplateSnapshot(ctx, snapshot, store, cfg.Site.TemplateSource)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		base, err := productionService(ctx, initial, store)
		if cleanupErr := cleanup(); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		load := func(ctx context.Context) (httpapi.Operations, func() error, error) {
			current, cleanup, err := productionTemplateSnapshot(ctx, snapshot, store, cfg.Site.TemplateSource)
			if err != nil {
				return httpapi.Operations{}, nil, err
			}
			service, err := productionService(ctx, current, store)
			if err != nil {
				return httpapi.Operations{}, nil, errors.Join(err, cleanup())
			}
			return operations(service), cleanup, nil
		}
		return platform.NewRuntime(httpapi.ReloadingOperations(operations(base), load), store.Close)
	default:
		return nil, fmt.Errorf("unsupported CAMIE runtime mode %q", mode)
	}
}

func productionTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store, source string) (coreconfig.Snapshot, func() error, error) {
	switch source {
	case "file":
		if err := validateFileTemplates(snapshot.Paths.Templates); err != nil {
			return coreconfig.Snapshot{}, nil, err
		}
		return snapshot, func() error { return nil }, nil
	case "database":
		return databaseTemplateSnapshot(ctx, snapshot, store)
	default:
		return coreconfig.Snapshot{}, nil, fmt.Errorf("unsupported CAMIE template source %q", source)
	}
}

func validateFileTemplates(paths map[string]string) error {
	for _, key := range []string{"layout", "home", "list", "article", "about"} {
		path := paths[key]
		if path == "" {
			return fmt.Errorf("camie file template %q path is required", key)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("read camie file template %q: %w", key, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("camie file template %q is not a regular file: %s", key, path)
		}
	}
	root := filepath.Dir(paths["layout"])
	parsed, err := template.New("portal").Funcs(camieTemplateFuncs()).ParseGlob(filepath.Join(root, "*.tmpl"))
	if err != nil {
		return fmt.Errorf("parse camie file templates: %w", err)
	}
	for _, name := range []string{"head", "foot", "home", "list", "article", "about"} {
		if parsed.Lookup(name) == nil {
			return fmt.Errorf("camie file template definition %q is missing", name)
		}
	}
	return nil
}

func productionService(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store) (*Service, error) {
	cfg, err := config.FromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	_ = ctx
	return NewService(generator.New(cfg, repository.NewWithStore(store, 0)), cfg.Site.PageSize), nil
}

func databaseTemplateSnapshot(ctx context.Context, snapshot coreconfig.Snapshot, store *portalcms.Store) (coreconfig.Snapshot, func() error, error) {
	binding := func(key, fallbackName, templateType string) portalcms.TemplateBinding {
		if code := snapshot.Paths.TemplateCodes[key]; code != "" {
			return portalcms.TemplateBinding{Key: key, Code: code, Type: templateType}
		}
		return portalcms.TemplateBinding{Key: key, Name: fallbackName, Type: templateType}
	}
	records, err := store.LoadBoundTemplates(ctx, []portalcms.TemplateBinding{
		binding("layout", "CAMIE公共布局", "special"), binding("home", "首页", "home"),
		binding("party", "党建专栏", "home"), binding("ministry", "部委动态", "home"),
		binding("news", "新闻中心", "home"), binding("training", "交流培训", "home"),
		binding("standards", "科技标准", "home"), binding("about", "关于协会", "home"),
		binding("list", "CAMIE栏目", "column"), binding("article", "CAMIE详情", "detail"),
	})
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	paths, cleanup, err := portalcms.MaterializeTemplates(records, camieTemplateFuncs())
	if err != nil {
		return coreconfig.Snapshot{}, nil, err
	}
	snapshot.Paths.Templates = paths
	return snapshot, cleanup, nil
}

func camieTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"root":       func(...any) string { return "" },
		"add":        func(...any) int { return 0 },
		"columnURL":  func(...any) string { return "" },
		"articleURL": func(...any) string { return "" },
	}
}

func operations(service *Service) httpapi.Operations {
	result := httpapi.OperationsForGenerator(service, generator.NormalizePageName, "页面名必须是：首页、普通栏目、普通详情、视频栏目、视频详情、搜索页或会员动态壳")
	result.ClassifyError = classifyError
	return result
}

func classifyError(err error) (int, string, bool) {
	switch {
	case errors.Is(err, contracts.ErrColumnNotFound), errors.Is(err, contracts.ErrArticleNotPublished), errors.Is(err, contracts.ErrTemplateNotFound):
		return http.StatusNotFound, err.Error(), true
	case errors.Is(err, contracts.ErrColumnNotUnique), errors.Is(err, contracts.ErrTemplateNotUnique), errors.Is(err, contracts.ErrArticleStillPublished):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, contracts.ErrInvalidOutputPath):
		return http.StatusBadRequest, err.Error(), true
	default:
		return 0, "", false
	}
}
