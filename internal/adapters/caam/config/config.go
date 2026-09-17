package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portal-static/internal/core/media"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database    DatabaseConfig    `yaml:"database"`
	Server      ServerConfig      `yaml:"server"`
	Site        SiteConfig        `yaml:"site"`
	Columns     ColumnsConfig     `yaml:"columns"`
	About       AboutConfig       `yaml:"about"`
	WorkPage    WorkPageConfig    `yaml:"work_page"`
	StatsPage   StatsPageConfig   `yaml:"stats_page"`
	MembersPage MembersPageConfig `yaml:"members_page"`
	PartyPage   PartyPageConfig   `yaml:"party_page"`
	Media       media.Config      `yaml:"-"`
}

func (c Config) EffectiveMedia() media.Config {
	if strings.TrimSpace(c.Media.Mode) != "" || c.Media.BaseURL != "" || len(c.Media.Aliases) > 0 || len(c.Media.RewriteOrigins) > 0 {
		return c.Media
	}
	mode := media.ModeSameOrigin
	baseURL := strings.TrimSpace(c.Site.MediaBaseURL)
	if baseURL != "" {
		mode = media.ModeCDN
	}
	return media.Config{
		Mode: mode, BaseURL: baseURL,
		Aliases: map[string]string{
			"caamm/uploads": "/caam/uploads",
			"caam/uploads":  "/caam/uploads",
			"uploads":       "/uploads",
		},
	}
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
	Template        string `yaml:"template"`
	Output          string `yaml:"output"`
	ListTemplate    string `yaml:"list_template"`
	ArticleTemplate string `yaml:"article_template"`
	OutputRoot      string `yaml:"output_root"`
	DistRoot        string `yaml:"dist_root"`
	PageSize        int    `yaml:"page_size"`
	MediaBaseURL    string `yaml:"media_base_url"`
	Timezone        string `yaml:"timezone"`
	LockStaleAfter  string `yaml:"lock_stale_after"`
	PageName        string `yaml:"page_name"`
	AllowEmptyStats bool   `yaml:"allow_empty_stats"`
}

type AboutConfig struct {
	Template          string `yaml:"template"`
	Output            string `yaml:"output"`
	Intro             string `yaml:"intro"`
	Leadership        string `yaml:"leadership"`
	Charter           string `yaml:"charter"`
	Organization      string `yaml:"organization"`
	Responsibilities  string `yaml:"responsibilities"`
	Honors            string `yaml:"honors"`
	RotatingPresident string `yaml:"rotating_president"`
	VicePresident     string `yaml:"vice_president"`
	ExecutiveDirector string `yaml:"executive_director"`
	Director          string `yaml:"director"`
	MemberDelegate    string `yaml:"member_delegate"`
	RegularMember     string `yaml:"regular_member"`
}

type WorkPageConfig struct {
	Template      string `yaml:"template"`
	Output        string `yaml:"output"`
	Headline      string `yaml:"headline"`
	Association   string `yaml:"association"`
	Branch        string `yaml:"branch"`
	Industry      string `yaml:"industry"`
	International string `yaml:"international"`
	Expo          string `yaml:"expo"`
	Platform      string `yaml:"platform"`
}

type StatsPageConfig struct {
	Template            string `yaml:"template"`
	Output              string `yaml:"output"`
	DomesticReports     string `yaml:"domestic_reports"`
	OverseasReports     string `yaml:"overseas_reports"`
	ProductionReports   string `yaml:"production_reports"`
	ImportExportReports string `yaml:"import_export_reports"`
	DomesticChart       string `yaml:"domestic_chart"`
	OverseasChart       string `yaml:"overseas_chart"`
	ProductionChart     string `yaml:"production_chart"`
	ImportExportChart   string `yaml:"import_export_chart"`
	Unit                string `yaml:"unit"`
}

type MembersPageConfig struct {
	Template          string `yaml:"template"`
	Output            string `yaml:"output"`
	Work              string `yaml:"work"`
	Style             string `yaml:"style"`
	Policy            string `yaml:"policy"`
	Charter           string `yaml:"charter"`
	Fees              string `yaml:"fees"`
	President         string `yaml:"president"`
	VicePresident     string `yaml:"vice_president"`
	BranchIntro       string `yaml:"branch_intro"`
	BranchRules       string `yaml:"branch_rules"`
	BranchService     string `yaml:"branch_service"`
	BranchRoster      string `yaml:"branch_roster"`
	Management        string `yaml:"management"`
	ExecutiveDirector string `yaml:"executive_director"`
	Director          string `yaml:"director"`
	Delegate          string `yaml:"delegate"`
	Member            string `yaml:"member"`
}

type PartyPageConfig struct {
	Template string `yaml:"template"`
	Output   string `yaml:"output"`
	Work     string `yaml:"work"`
	Study    string `yaml:"study"`
	News     string `yaml:"news"`
	Carousel string `yaml:"carousel"`
}

type ColumnsConfig struct {
	Headline    SlotConfig   `yaml:"headline"`
	Carousel    SlotConfig   `yaml:"carousel"`
	TopNews     []SlotConfig `yaml:"top_news"`
	Work        []SlotConfig `yaml:"work"`
	Industry    []SlotConfig `yaml:"industry"`
	Stats       SlotConfig   `yaml:"stats"`
	Topics      SlotConfig   `yaml:"topics"`
	Videos      SlotConfig   `yaml:"videos"`
	FooterLinks []SlotConfig `yaml:"footer_links"`
}

type SlotConfig struct {
	Key           string `yaml:"key"`
	Title         string `yaml:"title"`
	Name          string `yaml:"name"`
	Limit         int    `yaml:"limit"`
	Types         []int  `yaml:"types"`
	Href          string `yaml:"href"`
	FallbackCover string `yaml:"fallback_cover"`
	Unit          string `yaml:"unit"`
}

const defaultContentPlaceholder = "assets/images/content-placeholder.png"

func Default() Config {
	return Config{
		Database: DatabaseConfig{
			DSNEnv:          "CAAM_DB_DSN",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: "5m",
		},
		Server: ServerConfig{
			Addr:             "127.0.0.1:9142",
			TokenEnv:         "CAAM_STATIC_TOKEN",
			RequestTimeout:   "10m",
			BatchIdleTimeout: "15m",
			BatchMaxDuration: "6h",
		},
		Site: SiteConfig{
			Template:        "templates/home.html.tmpl",
			Output:          "../dist/caam-home/index.html",
			ListTemplate:    "templates/list.html.tmpl",
			ArticleTemplate: "templates/article.html.tmpl",
			OutputRoot:      "../caam-home",
			DistRoot:        "../dist/caam-home",
			PageSize:        10,
			Timezone:        "Asia/Shanghai",
			LockStaleAfter:  "30m",
			PageName:        "首页",
			AllowEmptyStats: true,
		},
		About: AboutConfig{
			Template:          "templates/about.html.tmpl",
			Output:            "about.html",
			Intro:             "协会概况简介",
			Leadership:        "协会概况领导团队",
			Charter:           "协会概况章程",
			Organization:      "协会概况组织",
			Responsibilities:  "协会概况职责",
			Honors:            "协会概况荣誉",
			RotatingPresident: "协会概况轮值会长",
			VicePresident:     "协会概况副会长",
			ExecutiveDirector: "协会概况常务理事",
			Director:          "协会概况理事",
			MemberDelegate:    "协会概况会员代表",
			RegularMember:     "协会概况普通会员",
		},
		WorkPage: WorkPageConfig{
			Template:      "templates/work.html.tmpl",
			Output:        "work.html",
			Headline:      "协会工作头条",
			Association:   "协会工作协会动态",
			Branch:        "协会工作分支机构动态",
			Industry:      "协会工作行业发展",
			International: "协会工作国际合作",
			Expo:          "协会工作展会信息",
			Platform:      "协会工作专业平台",
		},
		StatsPage: StatsPageConfig{
			Template: "templates/stats.html.tmpl", Output: "stats.html",
			DomesticReports: "统计数据国内数据", OverseasReports: "统计数据国外数据",
			ProductionReports: "统计数据产销", ImportExportReports: "统计数据进出口",
			DomesticChart: "国内数据汽车月度销量", OverseasChart: "国外数据汽车月度销量",
			ProductionChart: "产销汽车月度销量", ImportExportChart: "进出口汽车月度销量",
			Unit: "万辆",
		},
		MembersPage: MembersPageConfig{
			Template: "templates/members.html.tmpl", Output: "members.html",
			Work: "会员专区会员工作", Style: "会员专区会员风采", Policy: "会员专区相关制度",
			Charter: "会员专区协会章程", Fees: "会员专区会费缴纳标准与方法",
			President: "会员专区会长单位", VicePresident: "会员专区副会长单位",
			BranchIntro: "会员专区分支机构介绍", BranchRules: "会员专区分支机构管理办法",
			BranchService: "会员专区分支机构服务", BranchRoster: "会员专区分支机构名单",
			Management: "会员专区会员管理", ExecutiveDirector: "会员专区常务理事",
			Director: "会员专区理事单位", Delegate: "会员专区会员代表", Member: "会员专区会员单位",
		},
		PartyPage: PartyPageConfig{
			Template: "templates/party.html.tmpl", Output: "party.html",
			Work: "党建专区工作动态", Study: "党建专区学习教育", News: "党建专区党建要闻", Carousel: "党建专区轮播",
		},
		Columns: ColumnsConfig{
			Headline: slot("headline", "顶部头条", "首页头条", 1, []int{1, 2}, "", defaultContentPlaceholder),
			Carousel: slot("carousel", "新闻轮播", "首页轮播", 5, []int{1, 2}, "", defaultContentPlaceholder),
			TopNews: []SlotConfig{
				slot("industry-news", "行业要闻", "首页行业要闻", 5, []int{1, 2}, "list.html?category=industry-news", ""),
				slot("association-activity", "协会活动", "首页协会活动", 5, []int{1, 2}, "list.html?category=association-activity", ""),
				slot("notice", "通知公告", "首页通知公告", 5, []int{1, 2}, "list.html?category=notice", ""),
			},
			Work: []SlotConfig{
				slot("work-file", "协会文件", "首页协会文件", 6, []int{1, 2}, "list.html?category=work-file", defaultContentPlaceholder),
				slot("work-industry", "行业发展", "首页行业发展", 6, []int{1, 2}, "list.html?category=work-industry", defaultContentPlaceholder),
				slot("work-smart", "智能网联", "首页智能网联", 6, []int{1, 2}, "list.html?category=work-smart", defaultContentPlaceholder),
				slot("work-brand", "品牌服务", "首页品牌服务", 6, []int{1, 2}, "list.html?category=work-brand", defaultContentPlaceholder),
				slot("work-expo", "展会信息", "首页展会信息", 6, []int{1, 2}, "list.html?category=work-expo", defaultContentPlaceholder),
				slot("work-standard", "标准法规", "首页标准法规", 6, []int{1, 2}, "list.html?category=work-standard", defaultContentPlaceholder),
			},
			Industry: []SlotConfig{
				slot("company-news", "企业新闻", "首页企业新闻", 4, []int{1, 2}, "list.html?category=company-news", defaultContentPlaceholder),
				slot("international-cooperation", "国际合作", "首页国际合作", 4, []int{1, 2}, "list.html?category=international-cooperation", defaultContentPlaceholder),
				slot("industry-policy", "行业政策", "首页行业政策", 4, []int{1, 2}, "list.html?category=industry-policy", defaultContentPlaceholder),
				slot("laws-regulations", "法律法规", "首页法律法规", 4, []int{1, 2}, "list.html?category=laws-regulations", defaultContentPlaceholder),
			},
			Stats: func() SlotConfig {
				value := slot("stats", "统计数据", "首页统计数据", 6, []int{3}, "", "")
				value.Unit = "万辆"
				return value
			}(),
			Topics: slot("topics", "专题子站", "首页专题子站", 4, []int{1, 2}, "../first-version/topics.html", defaultContentPlaceholder),
			Videos: slot("videos", "推荐视频", "首页推荐视频", 4, []int{2}, "", defaultContentPlaceholder),
			FooterLinks: []SlotConfig{
				slot("friend-associations", "合作协会", "首页合作协会", 100, []int{1, 2}, "", ""),
				slot("friend-related", "相关链接", "首页相关链接", 100, []int{1, 2}, "", ""),
				slot("friend-media", "合作媒体", "首页合作媒体", 100, []int{1, 2}, "", ""),
			},
		},
	}
}

func slot(key, title, name string, limit int, types []int, href, fallback string) SlotConfig {
	return SlotConfig{Key: key, Title: title, Name: name, Limit: limit, Types: types, Href: href, FallbackCover: fallback}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("resolve config directory: %w", err)
	}
	if !filepath.IsAbs(cfg.Site.Template) {
		cfg.Site.Template = filepath.Join(base, cfg.Site.Template)
	}
	if !filepath.IsAbs(cfg.Site.Output) {
		cfg.Site.Output = filepath.Join(base, cfg.Site.Output)
	}
	if !filepath.IsAbs(cfg.Site.ListTemplate) {
		cfg.Site.ListTemplate = filepath.Join(base, cfg.Site.ListTemplate)
	}
	if !filepath.IsAbs(cfg.Site.ArticleTemplate) {
		cfg.Site.ArticleTemplate = filepath.Join(base, cfg.Site.ArticleTemplate)
	}
	if !filepath.IsAbs(cfg.Site.OutputRoot) {
		cfg.Site.OutputRoot = filepath.Join(base, cfg.Site.OutputRoot)
	}
	if !filepath.IsAbs(cfg.Site.DistRoot) {
		cfg.Site.DistRoot = filepath.Join(base, cfg.Site.DistRoot)
	}
	if !filepath.IsAbs(cfg.About.Template) {
		cfg.About.Template = filepath.Join(base, cfg.About.Template)
	}
	if !filepath.IsAbs(cfg.WorkPage.Template) {
		cfg.WorkPage.Template = filepath.Join(base, cfg.WorkPage.Template)
	}
	if !filepath.IsAbs(cfg.StatsPage.Template) {
		cfg.StatsPage.Template = filepath.Join(base, cfg.StatsPage.Template)
	}
	if !filepath.IsAbs(cfg.MembersPage.Template) {
		cfg.MembersPage.Template = filepath.Join(base, cfg.MembersPage.Template)
	}
	if !filepath.IsAbs(cfg.PartyPage.Template) {
		cfg.PartyPage.Template = filepath.Join(base, cfg.PartyPage.Template)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Database.DSNEnv) == "" {
		return errors.New("database.dsn_env is required")
	}
	if strings.TrimSpace(c.Site.Template) == "" || strings.TrimSpace(c.Site.Output) == "" ||
		strings.TrimSpace(c.Site.ListTemplate) == "" || strings.TrimSpace(c.Site.ArticleTemplate) == "" ||
		strings.TrimSpace(c.Site.OutputRoot) == "" || strings.TrimSpace(c.Site.DistRoot) == "" {
		return errors.New("site template, output, list_template, article_template, output_root and dist_root are required")
	}
	if filepath.Clean(c.Site.OutputRoot) == filepath.Clean(c.Site.DistRoot) {
		return errors.New("site.output_root and site.dist_root must be different")
	}
	aboutOutput := strings.TrimSpace(c.About.Output)
	if strings.TrimSpace(c.About.Template) == "" || aboutOutput != c.About.Output || filepath.Base(aboutOutput) != aboutOutput ||
		aboutOutput == "." || aboutOutput == ".." || !strings.EqualFold(filepath.Ext(aboutOutput), ".html") {
		return errors.New("about.template and a safe about.output filename are required")
	}
	for label, value := range map[string]string{
		"intro": c.About.Intro, "leadership": c.About.Leadership, "charter": c.About.Charter,
		"organization": c.About.Organization, "responsibilities": c.About.Responsibilities,
		"honors": c.About.Honors, "rotating_president": c.About.RotatingPresident,
		"vice_president": c.About.VicePresident, "executive_director": c.About.ExecutiveDirector,
		"director": c.About.Director, "member_delegate": c.About.MemberDelegate,
		"regular_member": c.About.RegularMember,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("about.%s is required", label)
		}
	}
	for page, values := range map[string][]string{
		"work_page":    {c.WorkPage.Template, c.WorkPage.Output, c.WorkPage.Headline, c.WorkPage.Association, c.WorkPage.Branch, c.WorkPage.Industry, c.WorkPage.International, c.WorkPage.Expo, c.WorkPage.Platform},
		"stats_page":   {c.StatsPage.Template, c.StatsPage.Output, c.StatsPage.DomesticReports, c.StatsPage.OverseasReports, c.StatsPage.ProductionReports, c.StatsPage.ImportExportReports, c.StatsPage.DomesticChart, c.StatsPage.OverseasChart, c.StatsPage.ProductionChart, c.StatsPage.ImportExportChart, c.StatsPage.Unit},
		"members_page": {c.MembersPage.Template, c.MembersPage.Output, c.MembersPage.Work, c.MembersPage.Style, c.MembersPage.Policy, c.MembersPage.Charter, c.MembersPage.Fees, c.MembersPage.President, c.MembersPage.VicePresident, c.MembersPage.BranchIntro, c.MembersPage.BranchRules, c.MembersPage.BranchService, c.MembersPage.BranchRoster, c.MembersPage.Management, c.MembersPage.ExecutiveDirector, c.MembersPage.Director, c.MembersPage.Delegate, c.MembersPage.Member},
		"party_page":   {c.PartyPage.Template, c.PartyPage.Output, c.PartyPage.Work, c.PartyPage.Study, c.PartyPage.News, c.PartyPage.Carousel},
	} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s configuration values are required", page)
			}
		}
	}
	for label, output := range map[string]string{"work_page.output": c.WorkPage.Output, "stats_page.output": c.StatsPage.Output, "members_page.output": c.MembersPage.Output, "party_page.output": c.PartyPage.Output} {
		if output != strings.TrimSpace(output) || filepath.Base(output) != output || !strings.EqualFold(filepath.Ext(output), ".html") {
			return fmt.Errorf("%s must be a safe html filename", label)
		}
	}
	if c.Site.PageSize < 1 || c.Site.PageSize > 100 {
		return errors.New("site.page_size must be between 1 and 100")
	}
	if strings.TrimSpace(c.Site.PageName) == "" {
		return errors.New("site.page_name is required")
	}
	if _, err := time.LoadLocation(c.Site.Timezone); err != nil {
		return fmt.Errorf("invalid site.timezone: %w", err)
	}
	for label, value := range map[string]string{
		"database.conn_max_lifetime": c.Database.ConnMaxLifetime,
		"server.request_timeout":     c.Server.RequestTimeout,
		"server.batch_idle_timeout":  c.Server.BatchIdleTimeout,
		"server.batch_max_duration":  c.Server.BatchMaxDuration,
		"site.lock_stale_after":      c.Site.LockStaleAfter,
	} {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", label, err)
		}
		if duration <= 0 {
			return fmt.Errorf("%s must be greater than zero", label)
		}
	}
	batchIdleTimeout, _ := time.ParseDuration(c.Server.BatchIdleTimeout)
	batchMaxDuration, _ := time.ParseDuration(c.Server.BatchMaxDuration)
	if batchMaxDuration <= batchIdleTimeout {
		return errors.New("server.batch_max_duration must be greater than server.batch_idle_timeout")
	}
	seen := make(map[string]struct{})
	for _, s := range c.AllSlots() {
		if strings.TrimSpace(s.Key) == "" || strings.TrimSpace(s.Name) == "" {
			return errors.New("every column slot requires key and name")
		}
		if _, ok := seen[s.Key]; ok {
			return fmt.Errorf("duplicate column key %q", s.Key)
		}
		seen[s.Key] = struct{}{}
		if s.Limit < 1 || s.Limit > 100 {
			return fmt.Errorf("column %s limit must be between 1 and 100", s.Key)
		}
		if len(s.Types) == 0 {
			return fmt.Errorf("column %s requires at least one article type", s.Key)
		}
		for _, typ := range s.Types {
			if typ < 1 || typ > 3 {
				return fmt.Errorf("column %s has unsupported article type %d", s.Key, typ)
			}
		}
	}
	if len(c.Columns.TopNews) != 3 || len(c.Columns.Work) != 6 || len(c.Columns.Industry) != 4 || len(c.Columns.FooterLinks) != 3 {
		return errors.New("top_news, work, industry and footer_links require 3, 6, 4 and 3 slots respectively")
	}
	if _, err := media.New(c.EffectiveMedia()); err != nil {
		return err
	}
	return nil
}

func (c Config) AllSlots() []SlotConfig {
	result := c.ContentSlots()
	result = append(result, c.Columns.FooterLinks...)
	return result
}

func (c Config) ContentSlots() []SlotConfig {
	result := []SlotConfig{c.Columns.Headline, c.Columns.Carousel}
	result = append(result, c.Columns.TopNews...)
	result = append(result, c.Columns.Work...)
	result = append(result, c.Columns.Industry...)
	result = append(result, c.Columns.Stats, c.Columns.Topics, c.Columns.Videos)
	return result
}

func (c Config) DSN() (string, error) {
	value := strings.TrimSpace(os.Getenv(c.Database.DSNEnv))
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", c.Database.DSNEnv)
	}
	return value, nil
}

func (c Config) Token() (string, error) {
	value := strings.TrimSpace(os.Getenv(c.Server.TokenEnv))
	if value == "" {
		return "", fmt.Errorf("environment variable %s is required", c.Server.TokenEnv)
	}
	return value, nil
}
