package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
)

func TestGenerateCommandFlowAlwaysUsesWholeSiteOperation(t *testing.T) {
	siteCalls, pageCalls := 0, 0
	operations := httpapi.Operations{
		GenerateSite: func(context.Context) (any, error) {
			siteCalls++
			return map[string]any{"generated_files": 1}, nil
		},
		GeneratePage: func(context.Context, string) (any, error) {
			pageCalls++
			return nil, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, driver := range []string{"miic", "caam"} {
		if err := generateSite("preview", config.Snapshot{Document: config.Document{Driver: driver, Site: config.SiteConfig{ID: driver}}}, operations, logger); err != nil {
			t.Fatal(err)
		}
	}
	if siteCalls != 2 || pageCalls != 0 {
		t.Fatalf("site calls=%d page calls=%d", siteCalls, pageCalls)
	}
}
