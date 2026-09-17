package miic

import (
	"context"
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

func (Factory) Build(_ context.Context, mode platform.Mode, snapshot coreconfig.Snapshot, logger *slog.Logger) (*platform.Runtime, error) {
	cfg, err := FromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	var closeRuntime func() error
	var source Source
	switch mode {
	case platform.Preview:
		cfg.Site.DistRoot = cfg.Site.PreviewRoot
		source = demo.NewSource()
	case platform.Production:
		store, openErr := portalcms.Open(snapshot.Database, snapshot.Site.Timezone)
		if openErr != nil {
			return nil, openErr
		}
		pageID, resolveErr := repository.ResolvePageIDWithStore(context.Background(), store, cfg.Site.PageName)
		if resolveErr != nil {
			_ = store.Close()
			return nil, resolveErr
		}
		source = repository.NewWithStore(store, pageID)
		closeRuntime = store.Close
	default:
		return nil, fmt.Errorf("unsupported MIIC runtime mode %q", mode)
	}
	service, err := New(cfg, source, logger)
	if err != nil {
		if closeRuntime != nil {
			_ = closeRuntime()
		}
		return nil, err
	}
	operations := httpapi.OperationsForGenerator(service, NormalizePageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们")
	return platform.NewRuntime(operations, closeRuntime)
}
