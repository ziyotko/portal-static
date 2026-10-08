package camie

import (
	"path/filepath"
	"testing"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/demo"
	"portal-static/internal/adapters/camie/generator"
	"portal-static/internal/platform/contracttest"
)

func TestCAMIEHTTPContract(t *testing.T) {
	root, _ := filepath.Abs("testdata")
	var cfg config.Config
	cfg.Site.SourceRoot, cfg.Site.TemplateRoot = filepath.Join(root, "site"), filepath.Join(root, "templates")
	cfg.Site.DistRoot, cfg.Site.PreviewRoot = filepath.Join(t.TempDir(), "site"), filepath.Join(t.TempDir(), "preview")
	cfg.Site.PageName, cfg.Site.HeroColumn, cfg.Site.FallbackCover = "环保机械协会", "news-hot", "assets/images/default-news-cover.png"
	cfg.Site.PageSize, cfg.Site.Timezone, cfg.Site.LockStaleAfter = 10, "Asia/Shanghai", "30m"
	service := NewService(generator.New(cfg, demo.NewSource()), cfg.Site.PageSize)
	contracttest.Run(t, operations(service), []contracttest.PageAlias{
		{Input: "首页", Normalized: "home"}, {Input: "普通栏目", Normalized: "list"},
		{Input: "视频详情", Normalized: "video"}, {Input: "会员动态壳", Normalized: "member"},
	})
}
