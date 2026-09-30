package config

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

// Config contains only CAMIE-specific rendering options. Process lifecycle,
// HTTP authentication, database and output safety are owned by portal-static.
type Config struct {
	Site struct {
		SourceRoot     string
		DistRoot       string
		PreviewRoot    string
		TemplateRoot   string
		PageName       string
		PageSize       int
		Timezone       string
		MediaBaseURL   string
		FallbackCover  string
		HeroColumn     string
		LockStaleAfter string
	}
	Media media.Config
}

type adapterDocument struct {
	Site struct {
		PageName       string `yaml:"page_name"`
		PageSize       int    `yaml:"page_size"`
		FallbackCover  string `yaml:"fallback_cover"`
		HeroColumn     string `yaml:"hero_column"`
		LockStaleAfter string `yaml:"lock_stale_after"`
	} `yaml:"site"`
}

func FromSnapshot(snapshot coreconfig.Snapshot) (Config, error) {
	var adapter adapterDocument
	data, err := yaml.Marshal(snapshot.Adapter)
	if err != nil {
		return Config{}, fmt.Errorf("encode camie adapter config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&adapter); err != nil {
		return Config{}, fmt.Errorf("parse camie adapter config: %w", err)
	}
	var cfg Config
	cfg.Site.SourceRoot = snapshot.Paths.SourceRoot
	cfg.Site.DistRoot = snapshot.Paths.DistRoot
	cfg.Site.PreviewRoot = snapshot.Paths.PreviewRoot
	cfg.Site.Timezone = snapshot.Site.Timezone
	cfg.Site.MediaBaseURL = snapshot.Media.BaseURL
	cfg.Site.PageName = adapter.Site.PageName
	cfg.Site.PageSize = adapter.Site.PageSize
	cfg.Site.FallbackCover = adapter.Site.FallbackCover
	cfg.Site.HeroColumn = adapter.Site.HeroColumn
	cfg.Site.LockStaleAfter = adapter.Site.LockStaleAfter
	cfg.Media = snapshot.Media

	// CAMIE templates share layout definitions and therefore must live together.
	for _, path := range snapshot.Paths.Templates {
		dir := filepath.Dir(path)
		if cfg.Site.TemplateRoot == "" {
			cfg.Site.TemplateRoot = dir
		} else if !strings.EqualFold(cfg.Site.TemplateRoot, dir) {
			return Config{}, errors.New("camie templates must be materialized in one directory")
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Site.SourceRoot == "" || c.Site.DistRoot == "" || c.Site.TemplateRoot == "" {
		return errors.New("camie source, dist and template roots are required")
	}
	if c.Site.PageSize < 1 || c.Site.PageSize > 100 {
		return errors.New("camie page_size must be between 1 and 100")
	}
	if strings.TrimSpace(c.Site.PageName) == "" || strings.TrimSpace(c.Site.HeroColumn) == "" {
		return errors.New("camie page_name and hero_column are required")
	}
	if _, err := time.LoadLocation(c.Site.Timezone); err != nil {
		return fmt.Errorf("load camie timezone: %w", err)
	}
	if _, err := time.ParseDuration(c.Site.LockStaleAfter); err != nil {
		return fmt.Errorf("parse camie lock_stale_after: %w", err)
	}
	if _, err := media.New(c.Media); err != nil {
		return err
	}
	return nil
}

// ValidateOutput remains as a defense inside the adapter. The public runtime
// also validates request-selected output paths before invoking it.
func ValidateOutput(source, target string) error {
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if strings.EqualFold(sourceAbs, targetAbs) || targetAbs == filepath.VolumeName(targetAbs)+string(filepath.Separator) {
		return errors.New("invalid camie output path")
	}
	for path := targetAbs; ; path = filepath.Dir(path) {
		info, statErr := os.Lstat(path)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("output path contains a symlink: %s", path)
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return statErr
		}
		if strings.EqualFold(path, sourceAbs) || filepath.Dir(path) == path {
			break
		}
	}
	return nil
}
