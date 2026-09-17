package config

import (
	"bytes"
	"fmt"
	"path/filepath"

	coreconfig "portal-static/internal/core/config"

	"gopkg.in/yaml.v3"
)

type adapterDocument struct {
	Site        adapterSiteConfig `yaml:"site"`
	Columns     ColumnsConfig     `yaml:"columns"`
	About       AboutConfig       `yaml:"about"`
	WorkPage    WorkPageConfig    `yaml:"work_page"`
	StatsPage   StatsPageConfig   `yaml:"stats_page"`
	MembersPage MembersPageConfig `yaml:"members_page"`
	PartyPage   PartyPageConfig   `yaml:"party_page"`
}

type adapterSiteConfig struct {
	PageSize        int    `yaml:"page_size"`
	LockStaleAfter  string `yaml:"lock_stale_after"`
	PageName        string `yaml:"page_name"`
	AllowEmptyStats bool   `yaml:"allow_empty_stats"`
}

func FromSnapshot(snapshot coreconfig.Snapshot) (Config, error) {
	data, err := yaml.Marshal(snapshot.Adapter)
	if err != nil {
		return Config{}, fmt.Errorf("encode caam adapter config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var adapter adapterDocument
	if err := decoder.Decode(&adapter); err != nil {
		return Config{}, fmt.Errorf("parse caam adapter config: %w", err)
	}
	cfg := Config{
		Database: DatabaseConfig{
			DSNEnv: snapshot.Database.DSNEnv, MaxOpenConns: snapshot.Database.MaxOpenConns,
			MaxIdleConns: snapshot.Database.MaxIdleConns, ConnMaxLifetime: snapshot.Database.ConnMaxLifetime,
		},
		Server: ServerConfig{
			Addr: snapshot.Server.Addr, TokenEnv: snapshot.Server.TokenEnv, RequestTimeout: snapshot.Server.RequestTimeout,
			BatchIdleTimeout: snapshot.Server.BatchIdleTimeout, BatchMaxDuration: snapshot.Server.BatchMaxDuration,
		},
		Site: SiteConfig{
			Template: snapshot.Paths.Templates["home"], ListTemplate: snapshot.Paths.Templates["list"],
			ArticleTemplate: snapshot.Paths.Templates["article"], OutputRoot: snapshot.Paths.SourceRoot,
			DistRoot: snapshot.Paths.DistRoot, PageSize: adapter.Site.PageSize, Timezone: snapshot.Site.Timezone,
			LockStaleAfter: adapter.Site.LockStaleAfter, PageName: adapter.Site.PageName, AllowEmptyStats: adapter.Site.AllowEmptyStats,
		},
		Columns: adapter.Columns, About: adapter.About, WorkPage: adapter.WorkPage, StatsPage: adapter.StatsPage,
		MembersPage: adapter.MembersPage, PartyPage: adapter.PartyPage, Media: snapshot.Media,
	}
	cfg.Site.Output = filepath.Join(cfg.Site.OutputRoot, "index.html")
	cfg.About.Template = snapshot.Paths.Templates["about"]
	cfg.WorkPage.Template = snapshot.Paths.Templates["work"]
	cfg.StatsPage.Template = snapshot.Paths.Templates["stats"]
	cfg.MembersPage.Template = snapshot.Paths.Templates["members"]
	cfg.PartyPage.Template = snapshot.Paths.Templates["party"]
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
