package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	Site     SiteConfig     `yaml:"site"`
	News     NewsConfig     `yaml:"news"`
	Business BusinessConfig `yaml:"business"`
	About    AboutConfig    `yaml:"about"`
}

type DatabaseConfig struct {
	DSNEnv          string `yaml:"dsn_env"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type ServerConfig struct {
	Addr             string `yaml:"addr"`
	TokenEnv         string `yaml:"token_env"`
	RequestTimeout   string `yaml:"request_timeout"`
	BatchIdleTimeout string `yaml:"batch_idle_timeout"`
	BatchMaxDuration string `yaml:"batch_max_duration"`
}

type SiteConfig struct {
	SourceRoot      string `yaml:"source_root"`
	DistRoot        string `yaml:"dist_root"`
	PreviewRoot     string `yaml:"preview_root"`
	NewsTemplate    string `yaml:"news_template"`
	ListTemplate    string `yaml:"list_template"`
	ArticleTemplate string `yaml:"article_template"`
	PageSize        int    `yaml:"page_size"`
	MediaBaseURL    string `yaml:"media_base_url"`
	Timezone        string `yaml:"timezone"`
	LockStaleAfter  string `yaml:"lock_stale_after"`
	FallbackCover   string `yaml:"fallback_cover"`
	PageName        string `yaml:"page_name"`
}

type NewsConfig struct {
	Important      SlotConfig   `yaml:"important"`
	Categories     []SlotConfig `yaml:"categories,omitempty"`
	DynamicColumns bool         `yaml:"dynamic_columns"`
	ExcludeColumns []string     `yaml:"exclude_columns"`
}

type BusinessConfig struct {
	PageName string `yaml:"page_name"`
}

type AboutConfig struct {
	PageName        string `yaml:"page_name"`
	RecruitmentName string `yaml:"recruitment_name"`
	DisclosureName  string `yaml:"disclosure_name"`
}

type SlotConfig struct {
	Key   string `yaml:"key"`
	Title string `yaml:"title"`
	Name  string `yaml:"name"`
	Limit int    `yaml:"limit"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	resolve := func(value string) string {
		if value == "" || filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Clean(filepath.Join(base, value))
	}
	cfg.Site.SourceRoot = resolve(cfg.Site.SourceRoot)
	cfg.Site.DistRoot = resolve(cfg.Site.DistRoot)
	cfg.Site.PreviewRoot = resolve(cfg.Site.PreviewRoot)
	cfg.Site.NewsTemplate = resolve(cfg.Site.NewsTemplate)
	cfg.Site.ListTemplate = resolve(cfg.Site.ListTemplate)
	cfg.Site.ArticleTemplate = resolve(cfg.Site.ArticleTemplate)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Site.SourceRoot) == "" || strings.TrimSpace(c.Site.DistRoot) == "" {
		return errors.New("site.source_root and site.dist_root are required")
	}
	if !safeOutput(c.Site.SourceRoot, c.Site.DistRoot) || (c.Site.PreviewRoot != "" && !safeOutput(c.Site.SourceRoot, c.Site.PreviewRoot)) {
		return errors.New("site output directories must be outside source_root or inside its dist directory")
	}
	if c.Site.PageSize <= 0 {
		return errors.New("site.page_size must be positive")
	}
	if c.Site.PageName == "" || c.News.Important.Name == "" {
		return errors.New("site.page_name and news.important are required")
	}
	if !c.News.DynamicColumns && len(c.News.Categories) == 0 {
		return errors.New("news.categories is required when dynamic_columns is false")
	}
	if c.Business.PageName == "" || c.About.PageName == "" || c.About.RecruitmentName == "" || c.About.DisclosureName == "" {
		return errors.New("business.page_name and about page/column names are required")
	}
	seen := map[string]bool{}
	for _, slot := range c.News.Categories {
		if slot.Key == "" || slot.Title == "" || slot.Name == "" {
			return errors.New("every news category requires key, title and name")
		}
		if seen[slot.Key] {
			return fmt.Errorf("duplicate news category key %q", slot.Key)
		}
		seen[slot.Key] = true
	}
	for label, raw := range map[string]string{
		"database.conn_max_lifetime": c.Database.ConnMaxLifetime,
		"server.request_timeout":     c.Server.RequestTimeout,
		"server.batch_idle_timeout":  c.Server.BatchIdleTimeout,
		"server.batch_max_duration":  c.Server.BatchMaxDuration,
		"site.lock_stale_after":      c.Site.LockStaleAfter,
	} {
		if _, err := time.ParseDuration(raw); err != nil {
			return fmt.Errorf("parse %s: %w", label, err)
		}
	}
	idle, _ := time.ParseDuration(c.Server.BatchIdleTimeout)
	max, _ := time.ParseDuration(c.Server.BatchMaxDuration)
	if max <= idle {
		return errors.New("server.batch_max_duration must exceed batch_idle_timeout")
	}
	return nil
}

func (c Config) DSN() (string, error) {
	value := strings.TrimSpace(os.Getenv(c.Database.DSNEnv))
	if c.Database.DSNEnv == "" || value == "" {
		return "", fmt.Errorf("database DSN environment variable %q is empty", c.Database.DSNEnv)
	}
	return value, nil
}

func (c Config) Token() (string, error) {
	value := strings.TrimSpace(os.Getenv(c.Server.TokenEnv))
	if c.Server.TokenEnv == "" || value == "" {
		return "", fmt.Errorf("static token environment variable %q is empty", c.Server.TokenEnv)
	}
	return value, nil
}

func safeOutput(source, output string) bool {
	source, _ = filepath.Abs(source)
	output, _ = filepath.Abs(output)
	source, output = filepath.Clean(source), filepath.Clean(output)
	if strings.EqualFold(source, output) {
		return false
	}
	rel, err := filepath.Rel(source, output)
	if err != nil {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	return len(parts) > 1 && strings.EqualFold(parts[0], "dist")
}
