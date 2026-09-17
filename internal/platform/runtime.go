package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
)

type Mode string

const (
	Preview    Mode = "preview"
	Production Mode = "production"
)

type Runtime struct {
	Operations httpapi.Operations
	close      func() error
}

func NewRuntime(operations httpapi.Operations, close func() error) (*Runtime, error) {
	if err := operations.Validate(); err != nil {
		if close != nil {
			_ = close()
		}
		return nil, err
	}
	if close == nil {
		close = func() error { return nil }
	}
	return &Runtime{Operations: operations, close: close}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

type AdapterFactory interface {
	Driver() string
	Build(context.Context, Mode, config.Snapshot, *slog.Logger) (*Runtime, error)
}

type Registry struct {
	mu        sync.RWMutex
	factories map[string]AdapterFactory
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]AdapterFactory)}
}

func (r *Registry) Register(factory AdapterFactory) error {
	if factory == nil {
		return errors.New("adapter factory is nil")
	}
	driver := strings.TrimSpace(factory.Driver())
	if driver == "" {
		return errors.New("adapter driver is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[driver]; exists {
		return fmt.Errorf("adapter driver %q is already registered", driver)
	}
	r.factories[driver] = factory
	return nil
}

func (r *Registry) Drivers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	drivers := make([]string, 0, len(r.factories))
	for driver := range r.factories {
		drivers = append(drivers, driver)
	}
	sort.Strings(drivers)
	return drivers
}

func (r *Registry) Build(ctx context.Context, mode Mode, snapshot config.Snapshot, logger *slog.Logger) (*Runtime, error) {
	if mode != Preview && mode != Production {
		return nil, fmt.Errorf("unsupported runtime mode %q", mode)
	}
	r.mu.RLock()
	factory, ok := r.factories[snapshot.Driver]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("driver %q is not implemented; available: %s", snapshot.Driver, strings.Join(r.Drivers(), ", "))
	}
	runtime, err := factory.Build(ctx, mode, snapshot, logger)
	if err != nil {
		return nil, fmt.Errorf("build %s adapter %q: %w", mode, snapshot.Driver, err)
	}
	if runtime == nil {
		return nil, fmt.Errorf("build %s adapter %q: runtime is nil", mode, snapshot.Driver)
	}
	return runtime, nil
}
