package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/demo"
	"portal-static/internal/adapters/camie/model"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	root, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.Site.SourceRoot = filepath.Join(root, "site")
	cfg.Site.TemplateRoot = filepath.Join(root, "templates")
	cfg.Site.DistRoot = filepath.Join(t.TempDir(), "camie")
	cfg.Site.PreviewRoot = cfg.Site.DistRoot
	cfg.Site.PageName = "环保机械协会"
	cfg.Site.PageSize = 10
	cfg.Site.Timezone = "Asia/Shanghai"
	cfg.Site.FallbackCover = "assets/images/hero-building.png"
	cfg.Site.HeroColumn = "news-hot"
	cfg.Site.LockStaleAfter = "30m"
	return cfg
}

func TestGenerateCompletePublicSiteAndMemberShells(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var privateID int64
	for _, column := range source.Columns {
		if column.Code == "member-reports" {
			privateID = column.ID
			break
		}
	}
	source.Items[privateID] = []model.Article{{ID: 999999, ColumnID: privateID, Type: model.ArticleTypeContent, Title: "不得静态化的会员报告", Content: "会员秘密正文", PublishTime: time.Now().Add(-time.Hour)}}
	var sharedID int64
	for columnID, items := range source.Items {
		if columnID != privateID && len(items) > 0 {
			shared := items[0]
			sharedID = shared.ID
			shared.ColumnID = privateID
			source.Items[privateID] = append(source.Items[privateID], shared)
			break
		}
	}
	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "news.html", "videos.html", "search.html", "pages/member.html", "pages/member-detail.html", "generated-content.js"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	var all strings.Builder
	err = filepath.WalkDir(result.Output, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".html" || filepath.Base(path) == "generated-content.js" {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			all.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(all.String(), "不得静态化的会员报告") || strings.Contains(all.String(), "会员秘密正文") {
		t.Fatal("member content leaked into public output")
	}
	searchIndex, err := os.ReadFile(filepath.Join(result.Output, "generated-content.js"))
	if err != nil {
		t.Fatal(err)
	}
	if sharedID == 0 || strings.Contains(string(searchIndex), `"id":`+fmt.Sprint(sharedID)+`,`) {
		t.Fatal("article assigned to both public and member columns must remain private")
	}
	publicDetail := readGenerated(t, result.Output, "detail.html")
	if strings.Contains(publicDetail, `"member-reports"`) || strings.Contains(publicDetail, `"videos-members"`) {
		t.Fatal("member column structure leaked into public page metadata")
	}
	if !strings.Contains(all.String(), "data-member-list-shell") {
		t.Fatal("member runtime shell was not generated")
	}
	home := readGenerated(t, result.Output, "index.html")
	for _, marker := range []string{
		`class="home-tabs" data-group="notice"`,
		`<button class="active" data-tab="notice-1"`,
		`class="hero-dots"`,
		`class="quick-rail"`,
		`class="updates-image"`,
		`class="conference-banner"`,
		`class="feature-links"`,
		`class="branch-card"`,
		`class="partners"`,
	} {
		if !strings.Contains(home, marker) {
			t.Fatalf("home page lost styled component markup %q", marker)
		}
	}
	if err := validateSite(result.Output); err != nil {
		t.Fatal(err)
	}
}

func readGenerated(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDuplicateNamesAllowedButCodesRemainUnique(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	source.Columns[1].Name = source.Columns[0].Name
	if _, err := New(cfg, source).load(context.Background()); err != nil {
		t.Fatalf("duplicate display names must be allowed: %v", err)
	}
	source.Columns[1].Code = source.Columns[0].Code
	if _, err := New(cfg, source).load(context.Background()); err == nil {
		t.Fatal("duplicate stable codes accepted")
	}
}
