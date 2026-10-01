package camie

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/demo"
	"portal-static/internal/adapters/camie/generator"
	"portal-static/internal/contracts"
)

func TestServiceImplementsStandardOperations(t *testing.T) {
	root, _ := filepath.Abs("testdata")
	var cfg config.Config
	cfg.Site.SourceRoot, cfg.Site.TemplateRoot = filepath.Join(root, "site"), filepath.Join(root, "templates")
	distRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	previewRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Site.DistRoot, cfg.Site.PreviewRoot = filepath.Join(distRoot, "site"), filepath.Join(previewRoot, "preview")
	cfg.Site.PageName, cfg.Site.HeroColumn, cfg.Site.FallbackCover = "环保机械协会", "news-hot", "assets/images/hero-building.png"
	cfg.Site.PageSize, cfg.Site.Timezone, cfg.Site.LockStaleAfter = 10, "Asia/Shanghai", "30m"
	service := NewService(generator.New(cfg, demo.NewSource()), cfg.Site.PageSize)
	if _, err := service.GenerateSite(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GenerateListByName(context.Background(), "news-hot"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GenerateListByName(context.Background(), "member-reports"); !errors.Is(err, contracts.ErrColumnNotFound) {
		t.Fatalf("private column error = %v", err)
	}
	if _, err := service.GenerateArticle(context.Background(), 999999); !errors.Is(err, contracts.ErrArticleNotPublished) {
		t.Fatalf("article error = %v", err)
	}
}
