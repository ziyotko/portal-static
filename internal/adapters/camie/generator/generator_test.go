package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/demo"
	"portal-static/internal/adapters/camie/model"
	"portal-static/internal/contracts"
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
	tempRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Site.DistRoot = filepath.Join(tempRoot, "camie")
	cfg.Site.PreviewRoot = cfg.Site.DistRoot
	cfg.Site.PageName = "环保机械协会"
	cfg.Site.PageSize = 10
	cfg.Site.Timezone = "Asia/Shanghai"
	cfg.Site.FallbackCover = "assets/images/default-news-cover.png"
	cfg.Site.HeroColumn = "news-hot"
	cfg.Site.LockStaleAfter = "30m"
	cfg.Site.MemberLoginPath = "/member-entry/login?site=camie"
	cfg.Site.MemberRegisterPath = "/member-entry/register"
	return cfg
}

func TestRequestedOutputUsesAllowedRootInsteadOfDefaultDistRoot(t *testing.T) {
	cfg := testConfig(t)
	parent := t.TempDir()
	cfg.Site.DistRoot = filepath.Join(parent, "default-site")
	cfg.Site.AllowedOutputRoot = filepath.Join(parent, "published")
	requested := filepath.Join(cfg.Site.AllowedOutputRoot, "dia-site")
	generator := New(cfg, demo.NewSource())

	if err := generator.ValidateOutputPath(requested); err != nil {
		t.Fatalf("requested Dia output was rejected: %v", err)
	}
	if err := generator.ValidateOutputPath(filepath.Join(parent, "outside")); !errors.Is(err, contracts.ErrInvalidOutputPath) {
		t.Fatalf("outside output error = %v, want ErrInvalidOutputPath", err)
	}
	result, err := generator.GenerateSite(contracts.WithOptions(context.Background(), requested, false))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(result.Output) != filepath.Clean(requested) {
		t.Fatalf("output = %q, want %q", result.Output, requested)
	}
}

func TestHomeGroupUsesBoundHomeColumn(t *testing.T) {
	section := &Article{ID: 1, Title: "栏目文章"}
	homeArticle := &Article{ID: 2, Title: "首页文章"}
	c := &catalog{
		columns: []model.Column{{ID: 1, Code: "news-hot", Name: "热点关注"}, {ID: 2, Code: "home-hero", Name: "重点新闻轮播"}},
		byCode:  map[string]int64{"news-hot": 1, "home-hero": 2},
		groups:  map[int64][]*Article{1: {section}, 2: {homeArticle}},
		all:     []*Article{section, homeArticle},
	}
	if got := homeGroup(c, "home-hero", "news-hot", 3); len(got.Items) != 1 || got.Items[0].ID != 2 {
		t.Fatalf("home slot did not use its bound column: %+v", got)
	}
	delete(c.groups, 2)
	if got := homeGroup(c, "home-hero", "news-hot", 3); len(got.Items) != 0 {
		t.Fatalf("empty home slot unexpectedly used section content: %+v", got)
	}
	delete(c.byCode, "home-hero")
	if got := homeGroup(c, "home-hero", "news-hot", 3); len(got.Items) != 1 || got.Items[0].ID != 1 {
		t.Fatalf("legacy data set did not use the section fallback: %+v", got)
	}
}

func TestHomeArticleDetailsUsePublicSectionSidebar(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var noticeColumn, topicColumn, heroColumn model.Column
	for _, column := range source.Columns {
		switch column.Code {
		case "news-notice":
			noticeColumn = column
		case "training-meetings":
			topicColumn = column
		case "news-hot":
			heroColumn = column
		}
	}
	if noticeColumn.ID == 0 {
		t.Fatal("missing notice column fixture")
	}
	if topicColumn.ID == 0 || len(source.Items[topicColumn.ID]) == 0 {
		t.Fatal("missing home topic fixture")
	}
	topicItems := source.Items[topicColumn.ID]
	topicItems[0].Cover = "assets/images/hero-water-treatment.jpg"
	topicTitle := topicItems[0].Title
	source.Items[topicColumn.ID] = topicItems
	if heroColumn.ID == 0 || len(source.Items[heroColumn.ID]) == 0 {
		t.Fatal("missing hero fixture")
	}
	heroItems := source.Items[heroColumn.ID]
	for i := range heroItems {
		heroItems[i].Cover = ""
	}
	source.Items[heroColumn.ID] = heroItems
	home := model.Column{ID: 9000, Name: "首页内容", Code: "home"}
	notices := model.Column{ID: 9001, ParentID: home.ID, Name: "通知公告", Code: "home-news-notice"}
	experts := model.Column{ID: 9002, ParentID: home.ID, Name: "专家委员会", Code: "home-experts"}
	source.Columns = append([]model.Column{home, notices, experts}, source.Columns...)
	shared := source.Items[noticeColumn.ID][0]
	onlyHome := model.Article{ID: 990001, Type: model.ArticleTypeContent, Title: "仅投放首页的通知", Content: "公开测试正文", PublishTime: time.Date(2026, 9, 28, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))}
	expert := model.Article{ID: 990002, Type: model.ArticleTypeContent, Title: "仅投放首页的专家文章", Content: "公开专家正文", Cover: "assets/images/video-thumb.png", PublishTime: onlyHome.PublishTime}
	source.Items[notices.ID] = []model.Article{shared, onlyHome}
	source.Items[experts.ID] = []model.Article{expert}

	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	homePage := readGenerated(t, result.Output, "index.html")
	for _, check := range []struct {
		id   int64
		code string
	}{
		{shared.ID, "news-notice"},
		{onlyHome.ID, "news-notice"},
		{expert.ID, "about-expert-insights"},
	} {
		detail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", check.id))
		if !strings.Contains(detail, `data-default-column-code="`+check.code+`"`) {
			t.Fatalf("article %d did not use public section %s", check.id, check.code)
		}
		if strings.Contains(detail, `>首页内容</`) || strings.Contains(detail, `>重点新闻轮播</`) {
			t.Fatalf("article %d still shows home placement as navigation", check.id)
		}
		if !strings.Contains(homePage, fmt.Sprintf("%d.html?from=%s", check.id, check.code)) {
			t.Fatalf("home link for article %d did not point to public section %s", check.id, check.code)
		}
	}
	if !strings.Contains(readGenerated(t, result.Output, "article/2026/09/990001.html"), `>通知公告</a>`) {
		t.Fatal("home-only notice detail lost the public notice sidebar")
	}
	if !strings.Contains(readGenerated(t, result.Output, "article/2026/09/990002.html"), `>专家委员会`) {
		t.Fatal("home-only expert detail lost the public expert sidebar")
	}
	expertFeature := regexp.MustCompile(`<a class="expert-feature"[^>]*><img src="([^"]+)" alt="仅投放首页的专家文章">`).FindStringSubmatch(homePage)
	if len(expertFeature) != 2 || expertFeature[1] != "./assets/images/video-thumb.png" {
		t.Fatalf("expert feature did not use the first article cover: %v", expertFeature)
	}
	updatesImage := regexp.MustCompile(`<a class="updates-image"[^>]*><img src="([^"]+)" alt="` + regexp.QuoteMeta(topicTitle) + `">`).FindStringSubmatch(homePage)
	if len(updatesImage) != 2 || updatesImage[1] != "./assets/images/hero-water-treatment.jpg" {
		t.Fatalf("updates image did not use the first article cover: %v", updatesImage)
	}
	if !regexp.MustCompile(`<a class="hero-slide active"[^>]*>\s*<div class="hero-media"><img src="\./assets/images/default-news-cover\.png"`).MatchString(homePage) {
		t.Fatal("hero without an article cover did not use the configured default news cover")
	}
}

func TestSectionPageUsesItsOwnTemplate(t *testing.T) {
	cfg := testConfig(t)
	sectionTemplates := t.TempDir()
	entries, err := os.ReadDir(cfg.Site.TemplateRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tmpl" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(cfg.Site.TemplateRoot, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sectionTemplates, entry.Name()), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sectionTemplates, "party.html.tmpl"), []byte(`{{define "party-list"}}<span data-page-template="party" hidden></span>{{template "list" .}}{{end}}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Site.TemplateRoot = sectionTemplates
	result, err := New(cfg, demo.NewSource()).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readGenerated(t, result.Output, "pages/party.html"), `data-page-template="party"`) {
		t.Fatal("party page did not use its page template")
	}
	if strings.Contains(readGenerated(t, result.Output, "pages/news.html"), `data-page-template="party"`) {
		t.Fatal("party page template was applied to another section")
	}
}

type videoAttachmentSource struct {
	*demo.Source
	articleID int64
}

func (s videoAttachmentSource) FetchAttachments(ctx context.Context, articleID int64) ([]model.Attachment, error) {
	if articleID == s.articleID {
		return []model.Attachment{
			{Name: "说明.pdf", URL: "/business_portal/uploads/article/guide.pdf"},
			{Name: "演示视频.mp4", URL: "/business_portal/uploads/article/demo.mp4"},
		}, nil
	}
	return s.Source.FetchAttachments(ctx, articleID)
}

func TestVideoDetailPlaysUploadedMP4Attachment(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var videoID int64
	for _, column := range source.Columns {
		if column.Code == "videos-news" {
			videoID = source.Items[column.ID][0].ID
			break
		}
	}
	if videoID == 0 {
		t.Fatal("missing public video fixture")
	}
	result, err := New(cfg, videoAttachmentSource{Source: source, articleID: videoID}).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", videoID))
	for _, marker := range []string{`<video controls`, `preload="metadata"`, `playsinline`, `<source src="/business_portal/uploads/article/demo.mp4" type="video/mp4">`} {
		if !strings.Contains(page, marker) {
			t.Fatalf("uploaded video detail missing %q", marker)
		}
	}
	if strings.Contains(page, `class="video-unavailable"`) || strings.Contains(page, `<source src="/business_portal/uploads/article/guide.pdf"`) {
		t.Fatal("video detail selected a non-video attachment or fallback")
	}
	if strings.Contains(page, "附件下载") || strings.Contains(page, `href="/business_portal/uploads/article/demo.mp4"`) {
		t.Fatal("video detail should play its attachment without a download section")
	}
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
	for _, name := range []string{"index.html", "news.html", "videos.html", "search.html", "pages/member.html", "pages/member-detail.html", "js/member.js", "generated-content.js"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	homePage := readGenerated(t, result.Output, "index.html")
	if got := strings.Count(homePage, `class="nav-submenu"`); got != 6 {
		t.Fatalf("want 6 primary navigation submenus, got %d", got)
	}
	for _, label := range []string{"党建要闻", "政策文件", "会员动态", "国际交流与合作", "绿色技术推广", "专家委员会"} {
		if !regexp.MustCompile(`<div class="nav-submenu">.*?<a href="[^"]+">` + label + `</a>`).MatchString(homePage) {
			t.Fatalf("primary navigation submenu missing linked item %q", label)
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
	for name, markers := range map[string][]string{
		"index.html":               {`href="/member-entry/register"`, `href="./pages/member.html"`, `href="./videos.html"`},
		"videos.html":              {`member.html?column=`, `mode=video`},
		"pages/member.html":        {`data-member-list-shell`, `data-member-columns`, `data-member-login-path="/member-entry/login?site=camie"`, `js/member.js`},
		"pages/member-detail.html": {`data-member-detail-shell`, `data-member-breadcrumb`, `data-member-login-path="/member-entry/login?site=camie"`, `js/member.js`},
	} {
		page := readGenerated(t, result.Output, name)
		for _, marker := range markers {
			if !strings.Contains(page, marker) {
				t.Fatalf("%s missing %q", name, marker)
			}
		}
	}
	branchHome := readGenerated(t, result.Output, "index.html")
	branchLinks := regexp.MustCompile(`class="branch-card" href="([^"]+)"`).FindAllStringSubmatch(branchHome, -1)
	if len(branchLinks) != 9 {
		t.Fatalf("want 9 branch links, got %d", len(branchLinks))
	}
	seenBranchLinks := make(map[string]bool, len(branchLinks))
	for _, match := range branchLinks {
		if seenBranchLinks[match[1]] {
			t.Fatalf("branch cards share a destination: %s", match[1])
		}
		seenBranchLinks[match[1]] = true
	}
	assertPublicListAndDetailShells(t, result.Output)
	var noticeColumn model.Column
	for _, column := range source.Columns {
		if column.Code == "news-notice" {
			noticeColumn = column
			break
		}
	}
	noticeDetail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", noticeColumn.ID*1000+3))
	if !strings.Contains(noticeDetail, `class="article-pager"`) {
		t.Fatal("article detail lost adjacent article navigation")
	}
	videoDetail := readGenerated(t, result.Output, "video-detail.html")
	if !strings.Contains(videoDetail, `class="video-unavailable"`) || strings.Contains(videoDetail, `<video controls`) {
		t.Fatal("video without a playable source should show an unavailable message")
	}
	for _, marker := range []string{
		`class="content-grid"`,
		`<aside><ul class="side-menu">`,
		`class="article-card video-detail-card"`,
		`class="video-stage"`,
	} {
		if !strings.Contains(videoDetail, marker) {
			t.Fatalf("video detail lost original two-column structure %q", marker)
		}
	}
	var branchColumn, branchParent model.Column
	for _, column := range source.Columns {
		if column.Code == "branch-water" {
			branchColumn = column
		}
	}
	for _, column := range source.Columns {
		if column.ID == branchColumn.ParentID {
			branchParent = column
		}
	}
	branchDetail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", branchColumn.ID*1000+1))
	expectedActiveBranch := fmt.Sprintf(`class="is-active"><button type="button" aria-expanded="true">%s `, branchParent.Name)
	expectedCurrentBranch := fmt.Sprintf(`class="current"><a href="../../../list/%d/1.html" aria-current="page">%s</a>`, branchColumn.ID, branchColumn.Name)
	if !strings.Contains(branchDetail, expectedActiveBranch) || !strings.Contains(branchDetail, expectedCurrentBranch) {
		t.Fatal("nested article did not activate its ancestor item in the side menu")
	}
	listPage := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", noticeColumn.ID))
	for _, marker := range []string{`<nav class="pagination" aria-label="分页">`, `class="plain disabled"`, `class="page-number active"`, `class="page-jump"`, `data-page-prefix="../../list/`} {
		if !strings.Contains(listPage, marker) {
			t.Fatalf("column pagination lost original structure %q", marker)
		}
	}
	topNews := readGenerated(t, result.Output, "list/13/1.html")
	if strings.Contains(topNews, `共 0 项数据`) || !strings.Contains(topNews, `class="news-row searchable"`) {
		t.Fatal("parent column page did not aggregate descendant articles")
	}
	videoList := readGenerated(t, result.Output, "pages/videos.html")
	for _, marker := range []string{`class="video-grid-card"`, `class="video-grid"`, `class="video-card searchable"`, `class="video-thumb"`, `class="play-button"`} {
		if !strings.Contains(videoList, marker) {
			t.Fatalf("video column lost original card layout %q", marker)
		}
	}
	partyList := readGenerated(t, result.Output, "pages/party.html")
	if !strings.Contains(partyList, `class="page-shell party-theme"`) {
		t.Fatal("party column lost its themed page shell")
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
	friendStart := strings.Index(home, `<div class="friend-links">`)
	if friendStart < 0 {
		t.Fatal("home page lost fixed friend links")
	}
	friendEnd := strings.Index(home[friendStart:], `</div>`)
	if friendEnd < 0 {
		t.Fatal("home page friend links are not closed")
	}
	friendLinks := home[friendStart : friendStart+friendEnd]
	if count := strings.Count(friendLinks, `<a href=`); count != 8 {
		t.Fatalf("fixed friend links count = %d, want 8", count)
	}
	for _, name := range []string{"中华人民共和国国家发展和改革委员会", "中共中央社会工作部", "中华人民共和国民政部", "中华人民共和国工业和信息化部", "中华人民共和国生态环境部", "中华人民共和国科学技术部", "国家知识产权局", "中国机经网"} {
		if !strings.Contains(friendLinks, name) {
			t.Fatalf("fixed friend links lost %q", name)
		}
	}
	if count := strings.Count(home, `class="partner-logo"`); count != 24 {
		t.Fatalf("rendered partner logo count = %d, want 24 including the accessible duplicate tracks", count)
	}
	for _, viewBox := range []string{"40 14 526 521", "35 35 280 282", "0 78 438 125", "85 385 809 162", "0 40 1178 716", "0 0 167 63", "0 8 192 176", "0 175 536 190", "10 45 1045 376", "0 0 418 55", "25 105 441 303", "20 112 560 208"} {
		if count := strings.Count(home, `viewBox="`+viewBox+`"`); count != 2 {
			t.Fatalf("partner logo crop %q rendered %d times, want 2", viewBox, count)
		}
	}
	search := readGenerated(t, result.Output, "search.html")
	for _, marker := range []string{
		`class="page-shell search-page"`,
		`class="search-card"`,
		`class="search-result-list"`,
		`class="search-result-item" data-search-result hidden`,
		`class="search-result-meta"`,
		`class="search-pagination pagination" data-search-pagination`,
		`data-search-prev`,
		`data-search-pages`,
		`data-search-next`,
	} {
		if !strings.Contains(search, marker) {
			t.Fatalf("search page lost original result or pagination structure %q", marker)
		}
	}
	searchScript := readGenerated(t, result.Output, "js/main.js")
	for _, marker := range []string{`const pageSize = 10;`, `const visiblePages = [];`, `button.className = 'page-number';`, `previousButton.disabled = currentPage === 1`, `nextButton.disabled = currentPage === pageCount`} {
		if !strings.Contains(searchScript, marker) {
			t.Fatalf("search pagination lost bounded ten-item behavior %q", marker)
		}
	}
	if strings.Contains(searchScript, `const pageSize = 6;`) {
		t.Fatal("search pagination reverted to six results per page")
	}
	commonCSS := readGenerated(t, result.Output, "css/common.css")
	if regexp.MustCompile(`(?s)\.article-card:not\(\.video-detail-card\)\s*\{[^}]*min-height`).MatchString(commonCSS) {
		t.Fatal("article card height should follow its detail content")
	}
	if err := validateSite(result.Output); err != nil {
		t.Fatal(err)
	}
}

func assertPublicListAndDetailShells(t *testing.T, root string) {
	t.Helper()
	checks := []struct {
		directory string
		markers   []string
	}{
		{"list", []string{`class="breadcrumb"`, `class="content-grid"`, `<aside><ul class="side-menu">`, `class="is-active"`}},
		{"article", []string{`data-detail-page`, `data-detail-breadcrumb`, `class="content-grid"`, `<aside><ul class="side-menu">`, `class="is-active"`, `class="article-card`, `class="article-meta"`}},
	}
	for _, check := range checks {
		count := 0
		err := filepath.WalkDir(filepath.Join(root, check.directory), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".html" {
				return err
			}
			count++
			page, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, marker := range check.markers {
				if !strings.Contains(string(page), marker) {
					return fmt.Errorf("%s lost required page structure %q", path, marker)
				}
			}
			if check.directory == "list" && !strings.Contains(string(page), `class="article-card"`) {
				for _, marker := range []string{`<nav class="pagination" aria-label="分页">`, `class="page-jump"`, `data-page-prefix=`} {
					if !strings.Contains(string(page), marker) {
						return fmt.Errorf("%s lost required pagination structure %q", path, marker)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("no generated %s pages were audited", check.directory)
		}
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
