package caam

import (
	"context"
	"fmt"
	"log/slog"

	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/platform"
	"portal-static/internal/sources/portalcms"
)

type Factory struct{}

func (Factory) Driver() string { return "caam" }

func (Factory) Build(_ context.Context, mode platform.Mode, snapshot coreconfig.Snapshot, logger *slog.Logger) (*platform.Runtime, error) {
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
		_, service, err := NewProductionWithStore(snapshot, store, logger)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
		return platform.NewRuntime(Operations(service), store.Close)
	default:
		return nil, fmt.Errorf("unsupported CAAM runtime mode %q", mode)
	}
}
