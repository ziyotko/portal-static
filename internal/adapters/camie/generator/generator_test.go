package generator

import (
	"context"
	"encoding/json"
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
	cfg.Site.HeroColumn = "home-hero"
	cfg.Site.LockStaleAfter = "30m"
	cfg.Site.MemberLoginPath = "/member-entry/login?site=camie"
	cfg.Site.MemberRegisterPath = "/member-entry/register"
	return cfg
}

func TestRequestedOutputUsesAllowedRootInsteadOfDefaultDistRoot(t *testing.T) {
	cfg := testConfig(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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

func TestHomeReusesPublicSectionsAndKeepsIndependentHeroPlacement(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var homeColumn, hotColumn, ministryColumn, noticeColumn, topicColumn, legacyPolicyColumn, publicPolicyColumn, heroColumn, aboutColumn, expertColumn, expertParentColumn model.Column
	for _, column := range source.Columns {
		switch column.Code {
		case "home":
			homeColumn = column
		case "news-hot":
			hotColumn = column
		case "ministry-work":
			ministryColumn = column
		case "news-notice":
			noticeColumn = column
		case "training-meetings":
			topicColumn = column
		case "home-policy-reports":
			legacyPolicyColumn = column
		case "policy-research":
			publicPolicyColumn = column
		case "home-hero":
			heroColumn = column
		case "about":
			aboutColumn = column
		case "about-expert-insights":
			expertColumn = column
		case "about-experts":
			expertParentColumn = column
		}
	}
	if noticeColumn.ID == 0 || hotColumn.ID == 0 || ministryColumn.ID == 0 {
		t.Fatal("missing public news column fixture")
	}
	if topicColumn.ID == 0 || len(source.Items[topicColumn.ID]) == 0 {
		t.Fatal("missing home topic fixture")
	}
	if homeColumn.ID == 0 || publicPolicyColumn.ID == 0 {
		t.Fatal("missing independent homepage fixtures")
	}
	if legacyPolicyColumn.ID == 0 {
		legacyPolicyColumn = model.Column{ID: 9003, ParentID: homeColumn.ID, Name: "政策研究", Code: "home-policy-reports"}
		source.Columns = append(source.Columns, legacyPolicyColumn)
	}
	topicItems := source.Items[topicColumn.ID]
	topicItems[0].Cover = "assets/images/hero-water-treatment.jpg"
	topicTitle := topicItems[0].Title
	source.Items[topicColumn.ID] = topicItems
	if heroColumn.ID == 0 || len(source.Items[heroColumn.ID]) == 0 {
		t.Fatal("missing hero fixture")
	}
	if aboutColumn.ID == 0 || expertColumn.ID == 0 || expertParentColumn.ID == 0 {
		t.Fatal("missing expert fixture")
	}
	notices := model.Column{ID: 9001, ParentID: homeColumn.ID, Name: "通知公告", Code: "home-news-notice"}
	experts := model.Column{ID: 9002, ParentID: homeColumn.ID, Name: "专家委员会", Code: "home-experts"}
	source.Columns = append(source.Columns, notices, experts)
	shared := source.Items[noticeColumn.ID][0]
	onlyHome := model.Article{ID: 990001, Type: model.ArticleTypeContent, Title: "仅投放首页的通知", Content: "公开测试正文", PublishTime: time.Date(2026, 9, 28, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))}
	expert := model.Article{ID: 990002, Type: model.ArticleTypeContent, Title: "正式专家栏目文章", Content: "公开专家正文", Cover: "assets/images/video-thumb.png", PublishTime: onlyHome.PublishTime}
	parentExpert := model.Article{ID: 990003, Type: model.ArticleTypeContent, Title: "直接发布在专家委员会的文章", Content: "专家委员会正文", PublishTime: time.Date(2026, 9, 27, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))}
	crossPostedExpert := model.Article{ID: 990007, Type: model.ArticleTypeContent, Title: "新闻与专家视野同时发布的文章", Content: "多栏目正文", PublishTime: time.Date(2026, 9, 26, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))}
	crossPostedParent := model.Article{ID: 990008, Type: model.ArticleTypeContent, Title: "新闻与专家委员会同时发布的文章", Content: "多栏目正文", PublishTime: time.Date(2026, 9, 25, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))}
	heroOnly := model.Article{ID: 990004, Type: model.ArticleTypeContent, Title: "首页独立轮播", Content: "轮播正文", PublishTime: onlyHome.PublishTime}
	legacyPolicyOnly := model.Article{ID: 990005, Type: model.ArticleTypeContent, Title: "旧首页独立政策研究", Content: "政策正文", PublishTime: onlyHome.PublishTime}
	publicOnly := model.Article{ID: 990006, Type: model.ArticleTypeContent, Title: "仅投放热点关注和部委动态", Content: "公开正文", Cover: "assets/images/hero-building.png", PublishTime: onlyHome.PublishTime}
	source.Items[notices.ID] = []model.Article{shared, onlyHome}
	source.Items[experts.ID] = []model.Article{onlyHome}
	source.Items[heroColumn.ID] = []model.Article{heroOnly}
	source.Items[hotColumn.ID] = append(source.Items[hotColumn.ID], publicOnly, crossPostedExpert, crossPostedParent)
	source.Items[ministryColumn.ID] = append(source.Items[ministryColumn.ID], publicOnly)
	source.Items[legacyPolicyColumn.ID] = []model.Article{legacyPolicyOnly}
	source.Items[expertColumn.ID] = []model.Article{expert, crossPostedExpert}
	source.Items[expertParentColumn.ID] = []model.Article{parentExpert, crossPostedParent}

	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	homePage := readGenerated(t, result.Output, "index.html")
	heroMarkup := regexp.MustCompile(`(?s)<article class="hero-slider".*?</article>`).FindString(homePage)
	if heroMarkup == "" || !strings.Contains(heroMarkup, heroOnly.Title) || strings.Contains(heroMarkup, publicOnly.Title) {
		t.Fatalf("homepage carousel must contain only home-hero placements: %s", heroMarkup)
	}
	for _, check := range []struct {
		id       int64
		code     string
		title    string
		category string
	}{
		{shared.ID, "news-notice", shared.Title, noticeColumn.Name},
		{expert.ID, "about-expert-insights", expert.Title, expertColumn.Name},
		{parentExpert.ID, "about-experts", parentExpert.Title, expertParentColumn.Name},
	} {
		detail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", check.id))
		breadcrumb := regexp.MustCompile(`<nav class="breadcrumb"[^>]*>.*?</nav>`).FindString(detail)
		if strings.Contains(breadcrumb, check.title) || !strings.Contains(breadcrumb, `<span aria-current="page">`+check.category+`</span>`) {
			t.Fatalf("article %d breadcrumb should end at its category: %s", check.id, breadcrumb)
		}
		if !strings.Contains(detail, `data-default-column-code="`+check.code+`"`) {
			t.Fatalf("article %d did not use public section %s", check.id, check.code)
		}
		if strings.Contains(detail, `>首页内容</`) || strings.Contains(detail, `>重点新闻轮播</`) {
			t.Fatalf("article %d still shows home placement as navigation", check.id)
		}
		if check.code == "about-experts" {
			continue
		}
		if !strings.Contains(homePage, fmt.Sprintf("%d.html?from=%s", check.id, check.code)) {
			t.Fatalf("home link for article %d did not point to public section %s", check.id, check.code)
		}
	}
	if strings.Contains(homePage, parentExpert.Title) {
		t.Fatal("article published directly in the expert parent column leaked into homepage expert section")
	}
	expertLinks := regexp.MustCompile(`(?s)<ul class="expert-links">.*?</ul>`).FindString(homePage)
	if !strings.Contains(expertLinks, fmt.Sprintf("%d.html?from=about-expert-insights", crossPostedExpert.ID)) || strings.Contains(expertLinks, fmt.Sprintf("%d.html?from=news-hot", crossPostedExpert.ID)) {
		t.Fatal("cross-posted expert article lost its expert context on homepage")
	}
	if strings.Contains(expertLinks, crossPostedParent.Title) {
		t.Fatal("cross-posted expert parent article leaked into homepage expert section")
	}
	expertParentList := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", expertParentColumn.ID))
	if !strings.Contains(expertParentList, fmt.Sprintf("%d.html?from=about-experts", crossPostedParent.ID)) {
		t.Fatal("cross-posted parent article lost its expert context in the expert committee list")
	}
	if !strings.Contains(readGenerated(t, result.Output, "article/2026/09/990002.html"), `>专家委员会`) {
		t.Fatal("formal expert detail lost the public expert sidebar")
	}
	expertDetail := readGenerated(t, result.Output, "article/2026/09/990002.html")
	hrefJSON := regexp.MustCompile(`<script id="column-path-hrefs" type="application/json">(.*?)</script>`).FindStringSubmatch(expertDetail)
	if len(hrefJSON) != 2 {
		t.Fatal("expert detail is missing breadcrumb column links")
	}
	var columnHrefs map[string][]string
	if err := json.Unmarshal([]byte(hrefJSON[1]), &columnHrefs); err != nil {
		t.Fatalf("parse breadcrumb column links: %v", err)
	}
	wantExpertHrefs := fmt.Sprintf("list/%d/1.html,list/%d/1.html,list/%d/1.html", aboutColumn.ID, expertParentColumn.ID, expertColumn.ID)
	if got := strings.Join(columnHrefs["about-expert-insights"], ","); got != wantExpertHrefs {
		t.Fatalf("expert breadcrumb links = %q, want %q", got, wantExpertHrefs)
	}
	if strings.Contains(homePage, onlyHome.Title) {
		t.Fatal("legacy home-only article leaked into homepage")
	}
	if !strings.Contains(homePage, fmt.Sprintf("%d.html?from=home-hero", heroOnly.ID)) {
		t.Fatalf("independent homepage hero article %d was not rendered", heroOnly.ID)
	}
	heroDetail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", heroOnly.ID))
	if !strings.Contains(heroDetail, `data-default-column-code="home-hero"`) || strings.Contains(heroDetail, `>首页内容</a>`) {
		t.Fatalf("independent homepage hero article %d has wrong section path", heroOnly.ID)
	}
	if strings.Contains(homePage, legacyPolicyOnly.Title) {
		t.Fatal("legacy homepage policy placement leaked into homepage")
	}
	for _, id := range []int64{onlyHome.ID, legacyPolicyOnly.ID} {
		if _, err := os.Stat(filepath.Join(result.Output, fmt.Sprintf("article/2026/09/%d.html", id))); !os.IsNotExist(err) {
			t.Fatalf("legacy home-only article %d was generated: %v", id, err)
		}
	}
	for _, legacy := range []string{
		fmt.Sprintf("list/%d/1.html", homeColumn.ID),
		fmt.Sprintf("list/%d/1.html", heroColumn.ID),
		fmt.Sprintf("list/%d/1.html", legacyPolicyColumn.ID),
		"list/9001/1.html", "list/9002/1.html",
	} {
		if _, err := os.Stat(filepath.Join(result.Output, legacy)); !os.IsNotExist(err) {
			t.Fatalf("homepage placement list was generated: %s: %v", legacy, err)
		}
	}
	heroDetail = readGenerated(t, result.Output, "article/2026/09/990004.html")
	if !strings.Contains(heroDetail, "home-placement-detail") || strings.Contains(heroDetail, `<ul class="side-menu">`) {
		t.Fatal("independent hero detail still shows homepage placements as a sidebar")
	}
	policyChildren := 0
	for _, column := range source.Columns {
		if column.ParentID != publicPolicyColumn.ID || len(source.Items[column.ID]) == 0 {
			continue
		}
		policyChildren++
		if title := source.Items[column.ID][0].Title; !strings.Contains(homePage, title) {
			t.Fatalf("homepage policy aggregation lost %s article %q", column.Code, title)
		}
	}
	if policyChildren != 5 {
		t.Fatalf("policy child fixture count = %d, want 5", policyChildren)
	}
	if !strings.Contains(homePage, fmt.Sprintf(`data-href="./list/%d/1.html"`, noticeColumn.ID)) || !strings.Contains(homePage, fmt.Sprintf(`data-href="./list/%d/1.html"`, topicColumn.ID)) {
		t.Fatal("homepage reused section labels did not link to public column lists")
	}
	policyButton := fmt.Sprintf(`data-href="./list/%d/1.html" type="button">政策研究</button>`, publicPolicyColumn.ID)
	if !strings.Contains(homePage, policyButton) {
		t.Fatalf("homepage policy research tab did not link to the public section: %s", policyButton)
	}
	expertFeature := regexp.MustCompile(`<a class="expert-feature"[^>]*><img src="([^"]+)" alt="正式专家栏目文章">`).FindStringSubmatch(homePage)
	if len(expertFeature) != 2 || expertFeature[1] != "./assets/images/video-thumb.png" {
		t.Fatalf("expert feature did not use the first article cover: %v", expertFeature)
	}
	updatesImage := regexp.MustCompile(`<a class="updates-image" href="([^"]+)"><img src="([^"]+)" alt="` + regexp.QuoteMeta(topicTitle) + `">`).FindStringSubmatch(homePage)
	expectedUpdatesHref := fmt.Sprintf("./article/%s/%d.html?from=%s", topicItems[0].PublishTime.Format("2006/01"), topicItems[0].ID, topicColumn.Code)
	if len(updatesImage) != 3 || updatesImage[1] != expectedUpdatesHref || updatesImage[2] != "./assets/images/hero-water-treatment.jpg" {
		t.Fatalf("updates image did not use the first article cover: %v", updatesImage)
	}
	if !regexp.MustCompile(`<a class="hero-slide active"[^>]*>\s*<div class="hero-media"><img src="\./assets/images/default-news-cover\.png"`).MatchString(homePage) {
		t.Fatal("hero without an article cover did not use the configured default news cover")
	}
}

func TestHomeHeroKeepsCarouselOrderForSharedArticles(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var heroID, hotID int64
	for _, column := range source.Columns {
		switch column.Code {
		case "home-hero":
			heroID = column.ID
		case "news-hot":
			hotID = column.ID
		}
	}
	if heroID == 0 || hotID == 0 || len(source.Items[hotID]) < 2 {
		t.Fatal("missing carousel or ordinary-column fixture")
	}
	first, second := source.Items[hotID][0], source.Items[hotID][1]
	first.IsTop, second.IsTop = false, true
	first.Source = "旧栏目发布来源"
	source.Items[hotID][0], source.Items[hotID][1] = first, second
	first.IsTop, second.IsTop = true, false
	first.Source = "文章当前来源"
	source.Items[heroID] = []model.Article{first, second}

	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	homePage := readGenerated(t, result.Output, "index.html")
	hero := regexp.MustCompile(`(?s)<article class="hero-slider".*?</article>`).FindString(homePage)
	firstAt, secondAt := strings.Index(hero, first.Title), strings.Index(hero, second.Title)
	if firstAt < 0 || secondAt < 0 || firstAt >= secondAt {
		t.Fatalf("carousel order changed by ordinary-column placement: %s", hero)
	}
	if !strings.Contains(hero, "文章当前来源") || strings.Contains(hero, "旧栏目发布来源") {
		t.Fatalf("carousel source came from an old publication snapshot: %s", hero)
	}
}

func TestHomeNoticeTabsFillEightRowsWithFullTitleHints(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	home := readGenerated(t, result.Output, "index.html")
	for index, code := range []string{"news-notice", "news-association", "news-member"} {
		var columnID int64
		for _, column := range source.Columns {
			if column.Code == code {
				columnID = column.ID
				break
			}
		}
		items := source.Items[columnID]
		if columnID == 0 || len(items) < 9 {
			t.Fatalf("%s needs at least nine preview articles to verify the eight-item limit", code)
		}
		pattern := fmt.Sprintf(`(?s)<ul id="notice-%d" class="notice-list"[^>]*>(.*?)</ul>`, index+1)
		matches := regexp.MustCompile(pattern).FindStringSubmatch(home)
		if len(matches) != 2 || strings.Count(matches[1], `<li class="searchable">`) != 8 {
			t.Fatalf("%s homepage tab did not render exactly eight items", code)
		}
		for _, item := range items[:8] {
			if !strings.Contains(matches[1], `title="`+item.Title+`">`+item.Title+`</a>`) {
				t.Fatalf("%s homepage tab lost the full title %q", code, item.Title)
			}
		}
		if strings.Contains(matches[1], items[8].Title) {
			t.Fatalf("%s homepage tab exceeded the eight-item limit", code)
		}
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

func TestVideoHubOnlyUsesVideoNewsColumn(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var videoColumnID, otherColumnID int64
	var expectedTitle string
	for _, column := range source.Columns {
		switch column.Code {
		case "videos-news":
			videoColumnID = column.ID
			if len(source.Items[column.ID]) > 0 {
				expectedTitle = source.Items[column.ID][0].Title
			}
		case "training-talent":
			otherColumnID = column.ID
		}
	}
	if videoColumnID == 0 || otherColumnID == 0 || expectedTitle == "" {
		t.Fatal("missing video or training fixture")
	}
	roamingVideo := model.Article{
		ID:          990007,
		ColumnID:    otherColumnID,
		Type:        model.ArticleTypeVideo,
		Title:       "不属于视频资讯栏目的培训视频",
		Content:     "培训视频正文",
		PublishTime: time.Now().Add(-time.Hour),
	}
	source.Items[otherColumnID] = append(source.Items[otherColumnID], roamingVideo)
	sharedVideo := model.Article{
		ID:          990008,
		ColumnID:    otherColumnID,
		Type:        model.ArticleTypeVideo,
		Title:       "同时发布到培训和视频资讯的视频",
		Content:     "共享视频正文",
		PublishTime: time.Now().Add(-2 * time.Hour),
	}
	source.Items[otherColumnID] = append(source.Items[otherColumnID], sharedVideo)
	sharedVideo.ColumnID = videoColumnID
	source.Items[videoColumnID] = append(source.Items[videoColumnID], sharedVideo)

	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	videoHub := readGenerated(t, result.Output, "videos.html")
	if !strings.Contains(videoHub, expectedTitle) {
		t.Fatal("video hub lost content published in videos-news")
	}
	if strings.Contains(videoHub, roamingVideo.Title) {
		t.Fatal("video hub included a video published outside videos-news")
	}
	if !strings.Contains(videoHub, fmt.Sprintf(`%d.html?from=videos-news`, sharedVideo.ID)) {
		t.Fatal("video hub did not use the videos-news publication context")
	}
	defaultVideo := readGenerated(t, result.Output, "video-detail.html")
	if strings.Contains(defaultVideo, roamingVideo.Title) || !strings.Contains(defaultVideo, expectedTitle) {
		t.Fatal("default video detail did not follow the videos-news collection")
	}
	searchPage := readGenerated(t, result.Output, "search.html")
	if !strings.Contains(searchPage, roamingVideo.Title) {
		t.Fatal("video outside videos-news should remain available to global search")
	}
}

func TestGenerateCompletePublicSiteAndMemberShells(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var privateID, introductionID, organizationID, charterID, sharedColumnID int64
	for _, column := range source.Columns {
		switch column.Code {
		case "member-reports":
			privateID = column.ID
		case "about-introduction":
			introductionID = column.ID
		case "about-organization":
			organizationID = column.ID
		case "about-charter":
			charterID = column.ID
		case "news-hot":
			sharedColumnID = column.ID
		}
	}
	if privateID == 0 || introductionID == 0 || organizationID == 0 || charterID == 0 || sharedColumnID == 0 || len(source.Items[organizationID]) == 0 {
		t.Fatal("missing member or about section fixtures")
	}
	source.Items[privateID] = []model.Article{{ID: 999999, ColumnID: privateID, Type: model.ArticleTypeContent, Title: "不得静态化的会员报告", Content: "会员秘密正文", PublishTime: time.Now().Add(-time.Hour)}}
	shared := model.Article{ID: 999998, ColumnID: sharedColumnID, Type: model.ArticleTypeContent, Title: "同时投放公开与会员栏目的文章", Content: "不得公开的共享正文", PublishTime: time.Now().Add(-time.Hour)}
	sharedID := shared.ID
	source.Items[sharedColumnID] = append(source.Items[sharedColumnID], shared)
	shared.ColumnID = privateID
	source.Items[privateID] = append(source.Items[privateID], shared)
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
	buildID := result.GeneratedAt.Format("20060102T150405.000000000-0700")
	generatedAt := result.GeneratedAt.Format(time.RFC3339Nano)
	htmlBuildID := strings.ReplaceAll(buildID, "+", "&#43;")
	htmlGeneratedAt := strings.ReplaceAll(generatedAt, "+", "&#43;")
	jsBuildID := strings.ReplaceAll(buildID, "+", `\u002b`)
	jsGeneratedAt := strings.ReplaceAll(generatedAt, "+", `\u002b`)
	urlBuildID := strings.ReplaceAll(buildID, "+", "%2b")
	for _, marker := range []string{
		`data-static-build="` + htmlBuildID + `"`,
		`data-generated-at="` + htmlGeneratedAt + `"`,
		`console.info("[CAMIE 静态化] 构建版本：` + jsBuildID + ` | 生成时间：` + jsGeneratedAt + `")`,
		`generated-content.js?v=` + urlBuildID,
	} {
		if !strings.Contains(homePage, marker) {
			t.Fatalf("homepage missing static build marker %q", marker)
		}
	}
	organization := source.Items[organizationID][0]
	organizationHref := fmt.Sprintf("list/%d/1.html", organizationID)
	if !strings.Contains(homePage, `href="./`+organizationHref+`">组织架构</a>`) {
		t.Fatal("primary navigation organization link did not point to its content page")
	}
	aboutPage := readGenerated(t, result.Output, "pages/about.html")
	if !strings.Contains(aboutPage, `href="../`+organizationHref+`">组织架构</a>`) {
		t.Fatal("about sidebar organization link did not point to its content page")
	}
	for _, section := range []struct {
		name string
		id   int64
	}{
		{"协会简介", introductionID},
		{"组织架构", organizationID},
		{"协会章程", charterID},
	} {
		page := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", section.id))
		first := source.Items[section.id][0]
		if !strings.Contains(page, `<section class="article-card"><h1>`+first.Title+`</h1><div class="article-body">`) ||
			strings.Contains(page, `class="pagination-wrap"`) || strings.Contains(page, `class="article-header"`) {
			t.Fatalf("%s did not use the about content layout", section.name)
		}
		if !strings.Contains(page, `<span aria-current="page">`+section.name+`</span>`) {
			t.Fatalf("%s content page has wrong breadcrumb", section.name)
		}
		if strings.Contains(page, source.Items[section.id][1].Title) {
			t.Fatalf("%s content page showed more than the first article", section.name)
		}
	}
	organizationDetail := fmt.Sprintf("article/%s/%d.html", organization.PublishTime.Format("2006/01"), organization.ID)
	if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(organizationDetail))); err != nil {
		t.Fatalf("existing organization detail link was removed: %v", err)
	}
	for pageName, railClass := range map[string]string{
		"index.html":        `class="quick-rail"`,
		"news.html":         `class="quick-rail quick-rail-global"`,
		"detail.html":       `class="quick-rail quick-rail-global"`,
		"pages/about.html":  `class="quick-rail quick-rail-global"`,
		"pages/member.html": `class="quick-rail quick-rail-global"`,
	} {
		page := readGenerated(t, result.Output, pageName)
		if strings.Count(page, `aria-label="快捷服务"`) != 1 || !strings.Contains(page, railClass) {
			t.Fatalf("%s did not render exactly one shared quick rail", pageName)
		}
	}
	for _, text := range []string{"京公网安备11010202009494", "京ICP备09071629号", "技术支持：机械工业信息中心"} {
		if !strings.Contains(homePage, text) {
			t.Fatalf("home footer missing %q", text)
		}
	}
	const beianEmblem = `<img src="/assets/images/beian-emblem.png" alt="" width="20" height="20">京公网安备11010202009494`
	if !strings.Contains(homePage, beianEmblem) {
		t.Fatal("home footer missing the public security emblem before its filing number")
	}
	if _, err := os.Stat(filepath.Join(result.Output, "assets/images/beian-emblem.png")); err != nil {
		t.Fatalf("missing public security emblem asset: %v", err)
	}
	for _, name := range []string{"pages/about.html", "list/36/1.html"} {
		page := readGenerated(t, result.Output, name)
		if !strings.Contains(page, beianEmblem) {
			t.Fatalf("%s footer does not use the root-relative public security emblem path", name)
		}
		breadcrumb := regexp.MustCompile(`<nav class="breadcrumb"[^>]*>.*?</nav>`).FindString(page)
		if got := strings.Count(breadcrumb, "协会简介"); got != 1 {
			t.Fatalf("%s breadcrumb repeats 协会简介 %d times: %s", name, got, breadcrumb)
		}
	}
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
		"index.html":               {`href="/member-entry/register"`, `href="./pages/member.html" data-member-entry`, `href="./videos.html"`},
		"videos.html":              {`member.html?column=`, `mode=video`},
		"pages/member.html":        {`data-member-list-shell`, `data-member-columns`, `data-member-login-path="/member-entry/login?site=camie"`, `data-member-auth-gate`, `js/member.js`},
		"pages/member-detail.html": {`data-member-detail-shell`, `data-member-breadcrumb`, `data-member-login-path="/member-entry/login?site=camie"`, `data-member-auth-gate`, `js/member.js`},
		"js/main.js":               {`[data-member-entry]`, `memberLoginURL(link.href)`},
		"js/member.js":             {`memberSpaceItem`, `我的空间`, `/business_member/member/dashboard`, `location.replace(loginURL())`},
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
	noticePanel := regexp.MustCompile(`(?s)<ul id="notice-1"[^>]*>(.*?)</ul>`).FindStringSubmatch(home)
	if len(noticePanel) != 2 {
		t.Fatal("home page lost the notice panel")
	}
	if count := strings.Count(noticePanel[1], `<li `); count != 8 {
		t.Fatalf("home notice item count = %d, want 8", count)
	}
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
		`>政策研究</button>`,
		`国家鼓励发展的重大环保<br>技术装备目录`,
		`重大环保技术装备创新任务<br>揭榜挂帅`,
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
				if check.directory == "article" && strings.Contains(string(page), "home-placement-detail") && (marker == `<aside><ul class="side-menu">` || marker == `class="is-active"`) {
					continue
				}
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

func TestEmptyAboutCharterUsesContentPage(t *testing.T) {
	cfg := testConfig(t)
	source := demo.NewSource()
	var charterID int64
	for _, column := range source.Columns {
		if column.Code == "about-charter" {
			charterID = column.ID
			break
		}
	}
	if charterID == 0 {
		t.Fatal("missing charter column fixture")
	}
	source.Items[charterID] = nil
	result, err := New(cfg, source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", charterID))
	if !strings.Contains(page, `<section class="article-card"><h1>协会章程</h1><p>暂无公开内容</p></section>`) ||
		strings.Contains(page, `class="pagination-wrap"`) ||
		!strings.Contains(page, `href="../../list/`+fmt.Sprint(charterID)+`/1.html" aria-current="page">协会章程</a>`) {
		t.Fatal("empty charter did not retain the shared about content layout and active sidebar")
	}
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
