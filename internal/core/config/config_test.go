package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerReloadsGenerationConfigAndDetectsBootstrapChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	write := func(mediaMode, addr string) {
		t.Helper()
		content := strings.ReplaceAll(exampleConfig, "MEDIA_MODE", mediaMode)
		content = strings.ReplaceAll(content, "SERVER_ADDR", addr)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("same_origin", "127.0.0.1:9143")
	manager, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	write("cdn\n  base_url: https://cdn.example.com", "127.0.0.1:9143")
	second, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash == second.Hash || second.Media.Mode != "cdn" {
		t.Fatalf("generation config was not reloaded: %#v", second.Media)
	}
	write("same_origin", "127.0.0.1:9999")
	if _, err := manager.Load(); err == nil || !manager.Info().RestartRequired {
		t.Fatal("expected restart-required error")
	}
}

func TestCommonConfigAllowsAdapterWithoutMySQL(t *testing.T) {
	config := Document{
		Version: 1,
		Driver:  "external-api",
		Site:    SiteConfig{ID: "external", Timezone: "Asia/Shanghai"},
		Server: ServerConfig{
			Addr: "127.0.0.1:9999", TokenEnv: "EXTERNAL_TOKEN",
			RequestTimeout: "1m", BatchIdleTimeout: "1m", BatchMaxDuration: "1h",
		},
		Paths: PathsConfig{SourceRoot: "/source", DistRoot: "/dist", Templates: map[string]string{"home": "/source/home.tmpl"}},
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := config.Database.ValidateMySQL(); err == nil {
		t.Fatal("expected MySQL-specific validation to reject empty database config")
	}
}

const exampleConfig = `version: 1
driver: miic
site:
  id: miic
  timezone: Asia/Shanghai
database:
  dsn_env: MIIC_DB_DSN
  schema: miic_portal
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 5m
server:
  addr: SERVER_ADDR
  token_env: MIIC_STATIC_TOKEN
  request_timeout: 10m
  batch_idle_timeout: 15m
  batch_max_duration: 6h
paths:
  source_root: .
  dist_root: ../dist/site
  templates:
    news: templates/news.html.tmpl
media:
  mode: MEDIA_MODE
adapter: {}
`
