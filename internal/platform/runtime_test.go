package platform

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
)

type testFactory struct {
	driver string
	build  func(Mode) (*Runtime, error)
}

func (f testFactory) Driver() string { return f.driver }
func (f testFactory) Build(_ context.Context, mode Mode, _ config.Snapshot, _ *slog.Logger) (*Runtime, error) {
	return f.build(mode)
}

func testOperations() httpapi.Operations {
	operation := func(context.Context) (any, error) { return nil, nil }
	article := func(context.Context, int64) (any, error) { return nil, nil }
	return httpapi.Operations{
		GenerateSite: operation, GeneratePages: operation, GenerateAllLists: operation, GenerateAllArticles: operation,
		GeneratePage: func(context.Context, string) (any, error) { return nil, nil },
		GenerateList: article, GenerateListByName: func(context.Context, string) (any, error) { return nil, nil },
		GenerateArticle: article, DeleteArticle: article, GenerateArticleRelated: article, DeleteArticleRelated: article,
		NormalizePageName: func(name string) (string, bool) { return name, true }, PageNameError: "invalid page",
		ValidateOutputPath: func(string) error { return nil }, ClassifyError: func(error) (int, string, bool) { return 0, "", false },
	}
}

func TestRegistryRejectsDuplicatesAndListsSortedDrivers(t *testing.T) {
	registry := NewRegistry()
	factory := testFactory{driver: "miic", build: func(Mode) (*Runtime, error) { return NewRuntime(testOperations(), nil) }}
	if err := registry.Register(factory); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(factory); err == nil {
		t.Fatal("expected duplicate error")
	}
	if err := registry.Register(testFactory{driver: "caam", build: func(Mode) (*Runtime, error) { return nil, nil }}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(registry.Drivers(), ","); got != "caam,miic" {
		t.Fatalf("drivers = %q", got)
	}
}

func TestRegistryReportsAvailableDrivers(t *testing.T) {
	registry := NewRegistry()
	_ = registry.Register(testFactory{driver: "miic", build: func(Mode) (*Runtime, error) { return nil, nil }})
	_, err := registry.Build(context.Background(), Preview, config.Snapshot{Document: config.Document{Driver: "missing"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "available: miic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryPassesPreviewAndProductionModesToFactory(t *testing.T) {
	registry := NewRegistry()
	var modes []Mode
	if err := registry.Register(testFactory{driver: "site", build: func(mode Mode) (*Runtime, error) {
		modes = append(modes, mode)
		return NewRuntime(testOperations(), nil)
	}}); err != nil {
		t.Fatal(err)
	}
	snapshot := config.Snapshot{Document: config.Document{Driver: "site"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, mode := range []Mode{Preview, Production} {
		runtime, err := registry.Build(context.Background(), mode, snapshot, logger)
		if err != nil {
			t.Fatal(err)
		}
		_ = runtime.Close()
	}
	if len(modes) != 2 || modes[0] != Preview || modes[1] != Production {
		t.Fatalf("modes = %v", modes)
	}
}

func TestNewRuntimeClosesResourceWhenContractIsIncomplete(t *testing.T) {
	closed := false
	_, err := NewRuntime(httpapi.Operations{}, func() error { closed = true; return nil })
	if err == nil || !closed {
		t.Fatalf("err=%v closed=%v", err, closed)
	}
}

func TestRuntimeCloseReturnsResourceError(t *testing.T) {
	want := errors.New("close failed")
	runtime, err := NewRuntime(testOperations(), func() error { return want })
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); !errors.Is(err, want) {
		t.Fatalf("close error = %v", err)
	}
}
