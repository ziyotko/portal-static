package miic

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/media"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Site     SiteConfig
	News     NewsConfig
	Business BusinessConfig
	About    AboutConfig
	Media    media.Config
}

type SiteConfig struct {
	SourceRoot        string `yaml:"source_root"`
	DistRoot          string `yaml:"dist_root"`
	PreviewRoot       string `yaml:"preview_root"`
	NewsTemplate      string `yaml:"news_template"`
	BusinessTemplate  string `yaml:"business_template"`
	PlatformsTemplate string `yaml:"platforms_template"`
	AboutTemplate     string `yaml:"about_template"`
	ListTemplate      string `yaml:"list_template"`
	ArticleTemplate   string `yaml:"article_template"`
	PageSize          int    `yaml:"page_size"`
	MediaBaseURL      string `yaml:"media_base_url"`
	Timezone          string `yaml:"timezone"`
	LockStaleAfter    string `yaml:"lock_stale_after"`
	FallbackCover     string `yaml:"fallback_cover"`
	PageName          string `yaml:"page_name"`
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

type adapterDocument struct {
	Site     adapterSiteConfig `yaml:"site"`
	News     NewsConfig        `yaml:"news"`
	Business BusinessConfig    `yaml:"business"`
	About    AboutConfig       `yaml:"about"`
}

type adapterSiteConfig struct {
	PageSize       int    `yaml:"page_size"`
	LockStaleAfter string `yaml:"lock_stale_after"`
	FallbackCover  string `yaml:"fallback_cover"`
	PageName       string `yaml:"page_name"`
}

func FromSnapshot(snapshot coreconfig.Snapshot) (Config, error) {
	var adapter adapterDocument
	data, err := yaml.Marshal(snapshot.Adapter)
	if err != nil {
		return Config{}, fmt.Errorf("encode miic adapter config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&adapter); err != nil {
		return Config{}, fmt.Errorf("parse miic adapter config: %w", err)
	}
	cfg := Config{
		Site: SiteConfig{
			SourceRoot: snapshot.Paths.SourceRoot, DistRoot: snapshot.Paths.DistRoot, PreviewRoot: snapshot.Paths.PreviewRoot,
			NewsTemplate: snapshot.Paths.Templates["news"], BusinessTemplate: snapshot.Paths.Templates["business"],
			PlatformsTemplate: snapshot.Paths.Templates["platforms"], AboutTemplate: snapshot.Paths.Templates["about"],
			ListTemplate: snapshot.Paths.Templates["list"], ArticleTemplate: snapshot.Paths.Templates["article"],
			PageSize: adapter.Site.PageSize, Timezone: snapshot.Site.Timezone, LockStaleAfter: adapter.Site.LockStaleAfter,
			FallbackCover: adapter.Site.FallbackCover, PageName: adapter.Site.PageName,
		},
		News: adapter.News, Business: adapter.Business, About: adapter.About, Media: snapshot.Media,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadLegacyConfig keeps the original MIIC YAML useful for regression tests
// and one-time migration. New deployments should use the versioned config.
func LoadLegacyConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var legacy struct {
		Site     SiteConfig     `yaml:"site"`
		News     NewsConfig     `yaml:"news"`
		Business BusinessConfig `yaml:"business"`
		About    AboutConfig    `yaml:"about"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(false)
	if err := decoder.Decode(&legacy); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, err
	}
	resolve := func(value string) string {
		if value == "" || filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Clean(filepath.Join(base, value))
	}
	legacy.Site.SourceRoot = resolve(legacy.Site.SourceRoot)
	legacy.Site.DistRoot = resolve(legacy.Site.DistRoot)
	legacy.Site.PreviewRoot = resolve(legacy.Site.PreviewRoot)
	legacy.Site.NewsTemplate = resolve(legacy.Site.NewsTemplate)
	if strings.TrimSpace(legacy.Site.BusinessTemplate) == "" {
		legacy.Site.BusinessTemplate = filepath.Join(legacy.Site.SourceRoot, "business.html")
	} else {
		legacy.Site.BusinessTemplate = resolve(legacy.Site.BusinessTemplate)
	}
	if strings.TrimSpace(legacy.Site.PlatformsTemplate) == "" {
		legacy.Site.PlatformsTemplate = filepath.Join(legacy.Site.SourceRoot, "platforms.html")
	} else {
		legacy.Site.PlatformsTemplate = resolve(legacy.Site.PlatformsTemplate)
	}
	if strings.TrimSpace(legacy.Site.AboutTemplate) == "" {
		legacy.Site.AboutTemplate = filepath.Join(legacy.Site.SourceRoot, "about.html")
	} else {
		legacy.Site.AboutTemplate = resolve(legacy.Site.AboutTemplate)
	}
	legacy.Site.ListTemplate = resolve(legacy.Site.ListTemplate)
	legacy.Site.ArticleTemplate = resolve(legacy.Site.ArticleTemplate)
	mediaConfig := defaultMediaConfig()
	if baseURL := strings.TrimSpace(legacy.Site.MediaBaseURL); baseURL != "" {
		mediaConfig.Mode, mediaConfig.BaseURL = media.ModeCDN, baseURL
	}
	cfg := Config{Site: legacy.Site, News: legacy.News, Business: legacy.Business, About: legacy.About, Media: mediaConfig}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaultMediaConfig() media.Config {
	return media.Config{Mode: media.ModeSameOrigin, Aliases: map[string]string{
		"mic/uploads": "/miic/uploads", "miic/uploads": "/miic/uploads",
	}}
}

func (c Config) Validate() error {
	if c.Site.SourceRoot == "" || c.Site.DistRoot == "" {
		return errors.New("miic source and dist roots are required")
	}
	if c.Site.NewsTemplate == "" || c.Site.BusinessTemplate == "" || c.Site.PlatformsTemplate == "" || c.Site.AboutTemplate == "" ||
		c.Site.ListTemplate == "" || c.Site.ArticleTemplate == "" {
		return errors.New("miic main page, list and article templates are required")
	}
	if c.Site.PageSize <= 0 {
		return errors.New("miic page_size must be positive")
	}
	if c.Site.PageName == "" || c.News.Important.Name == "" {
		return errors.New("miic page_name and news.important are required")
	}
	if c.Business.PageName == "" || c.About.PageName == "" || c.About.RecruitmentName == "" || c.About.DisclosureName == "" {
		return errors.New("miic business and about page names are required")
	}
	if _, err := time.ParseDuration(c.Site.LockStaleAfter); err != nil {
		return fmt.Errorf("parse miic lock_stale_after: %w", err)
	}
	if _, err := media.New(c.Media); err != nil {
		return err
	}
	return nil
}
