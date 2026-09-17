package builtin

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"portal-static/internal/core/config"
	"portal-static/internal/platform"
	"portal-static/internal/platform/contracttest"
)

func TestBuiltinAdaptersSharePublicContract(t *testing.T) {
	registry, err := Registry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		driver  string
		config  string
		aliases []contracttest.PageAlias
	}{
		{
			driver: "miic", config: filepath.Join("..", "..", "..", "configs", "miic.example.yaml"),
			aliases: []contracttest.PageAlias{{Input: "资讯动态", Normalized: "news"}, {Input: "news", Normalized: "news"}, {Input: "关于我们", Normalized: "about"}},
		},
		{
			driver: "caam", config: filepath.Join("..", "..", "..", "configs", "caam.example.yaml"),
			aliases: []contracttest.PageAlias{{Input: "首页", Normalized: "home"}, {Input: "home", Normalized: "home"}, {Input: "党建专区", Normalized: "party"}},
		},
	}
	for _, test := range tests {
		t.Run(test.driver, func(t *testing.T) {
			manager, err := config.Open(test.config)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := registry.Build(context.Background(), platform.Preview, manager.Bootstrap(), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.Close()
			contracttest.Run(t, runtime.Operations, test.aliases)
		})
	}
}
