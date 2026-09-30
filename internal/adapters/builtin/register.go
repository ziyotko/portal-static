package builtin

import (
	"portal-static/internal/adapters/caam"
	"portal-static/internal/adapters/camie"
	"portal-static/internal/adapters/miic"
	"portal-static/internal/platform"
)

func Registry() (*platform.Registry, error) {
	registry := platform.NewRegistry()
	for _, factory := range []platform.AdapterFactory{miic.Factory{}, caam.Factory{}, camie.Factory{}} {
		if err := registry.Register(factory); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
