package config

import "testing"

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
