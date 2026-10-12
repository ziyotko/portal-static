package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	coreconfig "portal-static/internal/core/config"
)

func TestMemberRoutesMustStayOnSiteOrigin(t *testing.T) {
	var cfg Config
	cfg.Site.SourceRoot = "site"
	cfg.Site.DistRoot = "dist"
	cfg.Site.TemplateRoot = "templates"
	cfg.Site.PageName = "环保机械协会"
	cfg.Site.PageSize = 10
	cfg.Site.HeroColumn = "news-hot"
	cfg.Site.Timezone = "Asia/Shanghai"
	cfg.Site.LockStaleAfter = "30m"
	cfg.Site.MemberRegisterPath = "/business_member/register"
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"/business_member/login", true},
		{"/business_member/login?site=camie", true},
		{"https://other.example/login", false},
		{"//other.example/login", false},
		{"business_member/login", false},
		{"/business_member/login#fragment", false},
		{"/business_member\\login", false},
	} {
		cfg.Site.MemberLoginPath = tc.path
		err := cfg.Validate()
		if (err == nil) != tc.valid {
			t.Errorf("path %q: valid=%v, error=%v", tc.path, tc.valid, err)
		}
	}
}

func TestTemplateSourceMustBeExplicitlySupported(t *testing.T) {
	var cfg Config
	cfg.Site.SourceRoot = "site"
	cfg.Site.DistRoot = "dist"
	cfg.Site.TemplateRoot = "templates"
	cfg.Site.PageName = "环保机械协会"
	cfg.Site.PageSize = 10
	cfg.Site.HeroColumn = "news-hot"
	cfg.Site.Timezone = "Asia/Shanghai"
	cfg.Site.LockStaleAfter = "30m"
	for _, source := range []string{"file", "database"} {
		cfg.Site.TemplateSource = source
		if err := cfg.Validate(); err != nil {
			t.Errorf("source %q: %v", source, err)
		}
	}
	cfg.Site.TemplateSource = "auto"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unexpected template source was accepted")
	}
}

func TestTemplateSourceDefaultsToFileForExistingCAMIEConfig(t *testing.T) {
	example, err := os.ReadFile("../../../../configs/camie.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(string(example), "\r\n", "\n")
	withoutSetting := strings.Replace(normalized, "    template_source: file\n", "", 1)
	if withoutSetting == normalized {
		t.Fatal("example config has no template_source setting to remove")
	}
	path := filepath.Join(t.TempDir(), "camie.yaml")
	if err := os.WriteFile(path, []byte(withoutSetting), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := coreconfig.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := FromSnapshot(manager.Bootstrap())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.TemplateSource != "file" {
		t.Fatalf("default template source = %q, want file", cfg.Site.TemplateSource)
	}
}
