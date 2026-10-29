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

func TestFullSiteNavigationFollowsCurrentColumns(t *testing.T) {
	source := demo.NewSource()
	var standardsID, workID, detailColumnID int64
	columns := source.Columns[:0]
	for _, column := range source.Columns {
		switch column.Code {
		case "standards":
			column.Name = "科技标准新名称"
			column.Sort = -1
			standardsID = column.ID
		case "standards-work":
			column.Name = "标准制修订新名称"
			column.Sort = -1
			workID = column.ID
		case "standards-work-notices":
			detailColumnID = column.ID
		case "standards-green-promotion":
			continue
		}
		columns = append(columns, column)
	}
	source.Columns = append(columns, model.Column{
		ID: 99999, Name: "新增标准栏目", Code: "standards-new-nav", ParentID: standardsID, Sort: 2,
	})
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	article := source.Items[detailColumnID][0]
	detailPath := fmt.Sprintf("article/%s/%d.html", article.PublishTime.Format("2006/01"), article.ID)
	for _, path := range []string{"index.html", fmt.Sprintf("list/%d/1.html", workID), detailPath} {
		page := readGenerated(t, result.Output, path)
		nav := regexp.MustCompile(`(?s)<nav class="nav-bar".*?</nav>`).FindString(page)
		if nav == "" || !strings.Contains(nav, ">科技标准新名称</a>") || !strings.Contains(nav, ">标准制修订新名称</a>") || !strings.Contains(nav, ">新增标准栏目</a>") {
			t.Fatalf("%s navigation did not use current column names: %s", path, nav)
		}
		if strings.Contains(nav, ">绿色技术推广</a>") || strings.Contains(nav, ">会员专区</a>") {
			t.Fatalf("%s navigation included removed or private column", path)
		}
		if strings.Index(nav, ">科技标准新名称</a>") > strings.Index(nav, ">党建专栏</a>") ||
			strings.Index(nav, ">标准制修订新名称</a>") > strings.Index(nav, ">创新成果转化</a>") {
			t.Fatalf("%s navigation ignored column sort order", path)
		}
	}
	list := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", workID))
	if !regexp.MustCompile(`class="nav-item active"><a class="nav-link"[^>]*>科技标准新名称</a>`).MatchString(list) {
		t.Fatal("renamed top-level column is not active on its child list")
	}
}

func TestAboutBranchesDefaultsToFirstBranchIntro(t *testing.T) {
	g := New(testConfig(t), demo.NewSource())
	column, total, err := g.Column(context.Background(), "about-branches", 0)
	if err != nil {
		t.Fatal(err)
	}
	if column.Code != "about-branches" || total != 12 {
		t.Fatalf("about branches = %q, first branch article count = %d, want 12", column.Code, total)
	}
}

func TestSidebarUsesBackendColumnSortOrder(t *testing.T) {
	c := &catalog{
		columns: []model.Column{
			{ID: 1, Name: "关于协会", Code: "about", Sort: 0},
			{ID: 4, Name: "信息公开", Code: "about-disclosure", ParentID: 1, Sort: 40},
			{ID: 3, Name: "分支机构", Code: "about-branches", ParentID: 1, Sort: 30},
			{ID: 2, Name: "协会简介", Code: "about-introduction", ParentID: 1, Sort: 10},
			{ID: 6, Name: "大气分会", Code: "branch-atmosphere", ParentID: 3, Sort: 20},
			{ID: 5, Name: "水分会", Code: "branch-water", ParentID: 3, Sort: 10},
		},
		byCode: map[string]int64{
			"about": 1, "about-introduction": 2, "about-branches": 3,
			"about-disclosure": 4, "branch-water": 5, "branch-atmosphere": 6,
		},
		groups: map[int64][]*Article{},
	}

	links := side(c, "about-disclosure")
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.Name)
	}
	if got := strings.Join(names, ","); got != "协会简介,分支机构,信息公开" {
		t.Fatalf("sidebar order = %q", got)
	}
	if got := strings.Join([]string{links[1].Children[0].Name, links[1].Children[1].Name}, ","); got != "水分会,大气分会" {
		t.Fatalf("nested sidebar order = %q", got)
	}
	if !links[2].Current || !links[2].Active {
		t.Fatal("information disclosure link is not current and active")
	}
	if got := defaultListColumn(c, 1).Code; got != "about-introduction" {
		t.Fatalf("default column = %q", got)
	}
}

func TestHomeBranchTreeGeneratesDedicatedListsAndParent(t *testing.T) {
	source := demo.NewSource()
	parentID := branchColumnID(source, "home-fenzhijigou")
	waterID := branchColumnID(source, "main-branch-water")
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatalf("dedicated home branch tree broke site generation: %v", err)
	}
	for _, id := range []int64{parentID, waterID} {
		if _, err := os.Stat(filepath.Join(result.Output, "list", fmt.Sprint(id), "1.html")); err != nil {
			t.Fatalf("dedicated branch page %d missing: %v", id, err)
		}
	}
	waterList := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", waterID))
	if !strings.Contains(waterList, `class="news-list"`) || !strings.Contains(waterList, `class="side-menu"`) || strings.Contains(waterList, `class="article-body"`) {
		t.Fatal("homepage water branch did not render a dedicated list")
	}
	waterDetail := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", source.Items[waterID][0].ID))
	if !strings.Contains(waterDetail, `data-default-column-code="main-branch-water"`) ||
		strings.Contains(waterDetail, `?from=home-fenzhijigou`) ||
		!strings.Contains(waterDetail, `?from=main-branch-water`) {
		t.Fatal("branch article detail did not keep its own branch context")
	}
}

func TestCrossPostedDetailKeepsEachPublishedSidebarContext(t *testing.T) {
	source := demo.NewSource()
	aboutID := branchColumnID(source, "about")
	waterID := branchColumnID(source, "main-branch-water")
	expertID := branchColumnID(source, "home-expert-insights")
	water := source.Items[waterID][0]
	expert := source.Items[expertID][0]
	// About is loaded before these sections, so it becomes the static default.
	source.Items[aboutID] = []model.Article{water, expert}
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		article model.Article
		code    string
		label   string
	}{
		{water, "main-branch-water", "大气分会"},
		{expert, "home-expert-insights", "专家视野"},
	} {
		page := readGenerated(t, result.Output, fmt.Sprintf("article/2026/09/%d.html", tc.article.ID))
		if !strings.Contains(page, `data-default-column-code="about"`) {
			t.Fatal("fixture did not reproduce the About default context")
		}
		payload := regexp.MustCompile(`(?s)<script id="detail-contexts" type="application/json">(.*?)</script>`).FindStringSubmatch(page)
		if len(payload) != 2 {
			t.Fatal("detail page is missing publication contexts")
		}
		var contexts map[string]detailContext
		if err := json.Unmarshal([]byte(payload[1]), &contexts); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(contexts[tc.code].Sidebar, tc.label) || strings.Contains(contexts[tc.code].Sidebar, "协会简介") {
			t.Fatalf("%s detail did not include its dedicated sidebar", tc.code)
		}
		if _, ok := contexts["news-hot"]; ok {
			t.Fatal("detail accepted a column without a publication relationship")
		}
	}
}

func TestMeetingParentAndChildrenKeepTheirOwnArticles(t *testing.T) {
	source := demo.NewSource()
	parentID := branchColumnID(source, "training-meetings")
	conferenceID := branchColumnID(source, "training-meetings-conference")
	otherID := branchColumnID(source, "training-meetings-other")
	disclosureID := branchColumnID(source, "about-disclosure")
	if len(source.Items[parentID]) == 0 || len(source.Items[conferenceID]) == 0 || len(source.Items[otherID]) == 0 {
		t.Fatal("missing direct parent or child fixture")
	}
	source.Items[conferenceID] = append(source.Items[conferenceID], model.Article{
		ID: 20163, ColumnID: conferenceID, Type: model.ArticleTypeContent,
		Title:       "第十一届环保装备高质量发展大会的通知",
		PublishTime: time.Date(2026, 9, 23, 9, 0, 0, 0, time.Local),
	})
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   int64
		own  string
		away string
	}{
		{parentID, source.Items[parentID][0].Title, source.Items[conferenceID][0].Title},
		{conferenceID, source.Items[conferenceID][0].Title, source.Items[parentID][0].Title},
		{otherID, source.Items[otherID][0].Title, source.Items[conferenceID][0].Title},
	} {
		page := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", tc.id))
		if !strings.Contains(page, tc.own) || strings.Contains(page, tc.away) {
			t.Fatalf("column %d did not render only its own articles", tc.id)
		}
	}
	home := readGenerated(t, result.Output, "index.html")
	if !strings.Contains(home, fmt.Sprintf(`class="conference-banner" href="./list/%d/1.html"`, conferenceID)) {
		t.Fatal("conference banner does not link to the conference column")
	}
	disclosure := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", disclosureID))
	if !strings.Contains(disclosure, `class="news-list"`) || !strings.Contains(disclosure, source.Items[disclosureID][0].Title) {
		t.Fatal("about disclosure did not render as an article list")
	}
}

func TestConferenceBannerLinksToColumnWhenArticleMissing(t *testing.T) {
	source := demo.NewSource()
	conferenceID := branchColumnID(source, "training-meetings-conference")
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	home := readGenerated(t, result.Output, "index.html")
	if !strings.Contains(home, fmt.Sprintf(`class="conference-banner" href="./list/%d/1.html"`, conferenceID)) {
		t.Fatal("conference banner does not link to the conference column")
	}
}

func TestSecondLevelColumnDoesNotAggregateThirdLevelArticles(t *testing.T) {
	g := New(testConfig(t), demo.NewSource())
	_, total, err := g.Column(context.Background(), "standards-innovation", 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("second-level column aggregated %d third-level articles, want 0 direct articles", total)
	}
	_, total, err = g.Column(context.Background(), "standards-achievement-evaluation", 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 12 {
		t.Fatalf("third-level column total = %d, want 12 direct articles", total)
	}
}

func TestListRendersCurrentTitleStyles(t *testing.T) {
	source := demo.NewSource()
	var columnID int64
	for _, column := range source.Columns {
		if column.Code == "training-meetings" {
			columnID = column.ID
			break
		}
	}
	if columnID == 0 || len(source.Items[columnID]) < 2 {
		t.Fatal("missing meeting articles")
	}
	styled := source.Items[columnID][0]
	styled.IsBold, styled.IsTop, styled.DefaultColor = true, true, "rgb(255, 34, 0)"
	source.Items[columnID][0] = styled
	unsafe := source.Items[columnID][1]
	unsafe.DefaultColor = "red;position:absolute"
	source.Items[columnID][1] = unsafe

	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", columnID))
	styledLink := regexp.MustCompile(`<a[^>]*class="is-bold"[^>]*style="color:#ff2200"[^>]*>` + regexp.QuoteMeta(styled.Title) + `</a>`)
	if !styledLink.MatchString(page) {
		t.Fatalf("styled title missing from list: %s", styled.Title)
	}
	if strings.Contains(page, "position:absolute") {
		t.Fatal("unsafe title color reached list HTML")
	}
	home := readGenerated(t, result.Output, "index.html")
	if !styledLink.MatchString(home) {
		t.Fatalf("styled title missing from homepage: %s", styled.Title)
	}
}

func TestCrossPostedColumnKeepsItsOwnPriorityAndStyles(t *testing.T) {
	source := demo.NewSource()
	var hotID, noticeID int64
	for _, column := range source.Columns {
		switch column.Code {
		case "news-hot":
			hotID = column.ID
		case "news-notice":
			noticeID = column.ID
		}
	}
	if hotID == 0 || noticeID == 0 || len(source.Items[hotID]) < 2 {
		t.Fatal("missing news columns or articles")
	}
	newer, older := source.Items[hotID][0], source.Items[hotID][1]
	newer.IsTop, older.IsTop = true, false
	source.Items[hotID] = []model.Article{newer, older}
	older.ColumnID, older.IsTop, older.IsBold, older.DefaultColor = noticeID, true, true, "#008000"
	newer.ColumnID, newer.IsTop, newer.IsBold, newer.DefaultColor = noticeID, false, false, "#0000ff"
	// The repository returns this column in publication-specific top/date order.
	source.Items[noticeID] = []model.Article{older, newer}

	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	list := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", noticeID))
	if first, second := strings.Index(list, older.Title), strings.Index(list, newer.Title); first < 0 || second < 0 || first >= second {
		t.Fatalf("column list ignored its own top order: first=%d second=%d", first, second)
	}
	styled := regexp.MustCompile(`<a[^>]*class="is-bold"[^>]*style="color:#008000"[^>]*>` + regexp.QuoteMeta(older.Title) + `</a>`)
	if !styled.MatchString(list) {
		t.Fatal("column list ignored its own bold/color settings")
	}
	home := readGenerated(t, result.Output, "index.html")
	notice := regexp.MustCompile(`(?s)<ul id="notice-1".*?</ul>`).FindString(home)
	if first, second := strings.Index(notice, older.Title), strings.Index(notice, newer.Title); first < 0 || second < 0 || first >= second || !styled.MatchString(notice) {
		t.Fatal("homepage notice tab ignored column-specific order or styles")
	}
}

func TestPinnedColumnArticlesUseActualPublicationOrder(t *testing.T) {
	source := demo.NewSource()
	var noticeID int64
	for _, column := range source.Columns {
		if column.Code == "news-notice" {
			noticeID = column.ID
			break
		}
	}
	if noticeID == 0 || len(source.Items[noticeID]) < 2 {
		t.Fatal("missing notice articles")
	}
	laterPublished, earlierPublished := source.Items[noticeID][0], source.Items[noticeID][1]
	laterPublished.IsTop, earlierPublished.IsTop = true, true
	laterPublished.PublishTime = time.Date(2026, 10, 19, 9, 0, 0, 0, time.Local)
	earlierPublished.PublishTime = time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local)
	laterPublished.ColumnPublishedAt = time.Date(2026, 10, 22, 9, 0, 0, 0, time.Local)
	earlierPublished.ColumnPublishedAt = time.Date(2026, 10, 21, 9, 0, 0, 0, time.Local)
	source.Items[noticeID] = []model.Article{earlierPublished, laterPublished}

	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	list := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", noticeID))
	if first, second := strings.Index(list, laterPublished.Title), strings.Index(list, earlierPublished.Title); first < 0 || second < 0 || first >= second {
		t.Fatalf("later published pinned article should come first: first=%d second=%d", first, second)
	}
}

func TestSafeTitleColor(t *testing.T) {
	cases := map[string]string{
		"#f00": "#f00", "rgb(255, 34, 0)": "#ff2200",
		"rgba(255, 34, 0, 0.5)": "rgba(255,34,0,0.5)",
		"rgb(300, 34, 0)":       "", "red;position:absolute": "",
		"rgba(255, 34, 0, NaN)": "",
	}
	for input, want := range cases {
		if got := string(safeTitleColor(input)); got != want {
			t.Errorf("safeTitleColor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHomeUpdatesUseSectionFallbackOnlyForMissingCover(t *testing.T) {
	source := demo.NewSource()
	chosen := map[string]string{
		"standards-work":         "assets/images/hero-water-treatment.jpg",
		"training-international": updatesFallbackCover["training-international"],
		"training-talent":        updatesFallbackCover["training-talent"],
		"policy-research":        updatesFallbackCover["policy-research"],
	}
	for _, column := range source.Columns {
		want, ok := chosen[column.Code]
		if !ok || len(source.Items[column.ID]) == 0 {
			continue
		}
		items := source.Items[column.ID]
		items[0].Cover = ""
		if column.Code == "standards-work" {
			items[0].Cover = want
		} else if column.Code == "training-international" {
			items[0].Cover = "assets/images/hero-building.png"
		}
		source.Items[column.ID] = items
	}
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	home := readGenerated(t, result.Output, "index.html")
	for _, column := range source.Columns {
		want, ok := chosen[column.Code]
		if !ok || len(source.Items[column.ID]) == 0 {
			continue
		}
		title := regexp.QuoteMeta(source.Items[column.ID][0].Title)
		pattern := regexp.MustCompile(`<a class="updates-image"[^>]*><img src="\./` + regexp.QuoteMeta(want) + `" alt="` + title + `">`)
		if !pattern.MatchString(home) {
			t.Errorf("%s first article did not use expected cover %s", column.Code, want)
		}
	}
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
		case "home-expert-insights":
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
		{expert.ID, "home-expert-insights", expert.Title, expertColumn.Name},
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
	if !strings.Contains(expertLinks, fmt.Sprintf("%d.html?from=home-expert-insights", crossPostedExpert.ID)) || strings.Contains(expertLinks, fmt.Sprintf("%d.html?from=news-hot", crossPostedExpert.ID)) {
		t.Fatal("cross-posted expert article lost its expert context on homepage")
	}
	expertHomeList := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", expertColumn.ID))
	if !strings.Contains(expertHomeList, expert.Title) || strings.Contains(expertHomeList, parentExpert.Title) {
		t.Fatal("homepage expert list did not use its dedicated column")
	}
	if strings.Contains(expertLinks, crossPostedParent.Title) {
		t.Fatal("cross-posted expert parent article leaked into homepage expert section")
	}
	expertParentPage := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", expertParentColumn.ID))
	if !strings.Contains(expertParentPage, parentExpert.Content) || strings.Contains(expertParentPage, expert.Title) {
		t.Fatal("expert committee introduction did not render its own article body")
	}
	if !strings.Contains(expertParentPage, `>关于协会</a>`) || !strings.Contains(expertParentPage, `>专家委员会</span>`) {
		t.Fatal("expert committee introduction lost its About breadcrumb")
	}
	if !strings.Contains(expertParentPage, `class="side-menu"`) || !strings.Contains(expertParentPage, `>协会简介</a>`) || !strings.Contains(expertParentPage, `aria-current="page">专家委员会</a>`) {
		t.Fatal("expert committee introduction lost its About sidebar")
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
	wantExpertHrefs := fmt.Sprintf("list/%d/1.html", expertColumn.ID)
	if got := strings.Join(columnHrefs["home-expert-insights"], ","); got != wantExpertHrefs {
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
		title := source.Items[column.ID][0].Title
		if column.Code == "policy-reports" && !strings.Contains(homePage, title) {
			t.Fatalf("homepage policy tab did not show the first second-level column article %q", title)
		}
		if column.Code != "policy-reports" && strings.Contains(homePage, title) {
			t.Fatalf("homepage policy tab aggregated child column %s article %q", column.Code, title)
		}
	}
	if policyChildren != 5 {
		t.Fatalf("policy child fixture count = %d, want 5", policyChildren)
	}
	if !strings.Contains(homePage, fmt.Sprintf(`data-href="./list/%d/1.html"`, noticeColumn.ID)) || !strings.Contains(homePage, fmt.Sprintf(`data-href="./list/%d/1.html"`, topicColumn.ID)) {
		t.Fatal("homepage reused section labels did not link to public column lists")
	}
	var firstPolicyChild model.Column
	for _, column := range source.Columns {
		if column.ParentID == publicPolicyColumn.ID {
			firstPolicyChild = column
			break
		}
	}
	policyButton := fmt.Sprintf(`data-href="./list/%d/1.html" type="button">政策研究</button>`, firstPolicyChild.ID)
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
}

func TestHomeHeroShowsEveryPublishedPlacement(t *testing.T) {
	source := demo.NewSource()
	var heroID int64
	for _, column := range source.Columns {
		if column.Code == "home-hero" {
			heroID = column.ID
			break
		}
	}
	if heroID == 0 {
		t.Fatal("missing carousel column fixture")
	}
	items := make([]model.Article, 5)
	for i := range items {
		items[i] = model.Article{
			ID:          int64(991000 + i),
			Type:        model.ArticleTypeContent,
			Title:       fmt.Sprintf("轮播内容 %d", i+1),
			Content:     "测试正文",
			PublishTime: time.Date(2026, 10, 1+i, 9, 0, 0, 0, time.Local),
		}
	}
	source.Items[heroID] = items
	result, err := New(testConfig(t), source).GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	homePage := readGenerated(t, result.Output, "index.html")
	hero := regexp.MustCompile(`(?s)<article class="hero-slider".*?</article>`).FindString(homePage)
	if got := strings.Count(hero, `class="hero-slide `); got != 5 {
		t.Fatalf("carousel has %d slides, want 5", got)
	}
	for _, item := range items {
		if !strings.Contains(hero, item.Title) {
			t.Fatalf("carousel missing %q", item.Title)
		}
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
	noticeID := branchColumnID(source, "news-notice")
	noticeItems := source.Items[noticeID]
	if len(noticeItems) < 3 {
		t.Fatal("missing notice article fixture")
	}
	noticeItems[2].Cover = "https://example.com/conference-cover.jpg"
	source.Items[noticeID] = noticeItems
	waterID := branchColumnID(source, "branch-water")
	waterItems := source.Items[waterID]
	waterItems[0].Content = "<p>水分会专属简介</p>"
	source.Items[waterID] = waterItems
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
	for _, name := range []string{"index.html", "news.html", "videos.html", "search.html", "pages/member.html", "pages/member-statistics.html", "pages/member-detail.html", "css/member-statistics.css", "js/member.js", "js/member-statistics-dashboard.js", "js/member-statistics.js", "assets/images/member-statistics-banner.svg", "generated-content.js"} {
		if _, err := os.Stat(filepath.Join(result.Output, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	homePage := readGenerated(t, result.Output, "index.html")
	if !strings.Contains(homePage, `class="language-switch" href="#coming-soon" aria-label="中英文切换敬请期待"`) {
		t.Fatal("language switch does not show the coming-soon state")
	}
	englishPage := readGenerated(t, result.Output, "en/about.html")
	for _, marker := range []string{`<html lang="en">`, `class="about-page english-about-page"`, `class="language-switch" href="#coming-soon"`, `The China Association of Machinery Industry`, `src="../assets/images/about-english.png"`} {
		if !strings.Contains(englishPage, marker) {
			t.Fatalf("English page missing %q", marker)
		}
	}
	if strings.Contains(englishPage, `<aside><ul class="side-menu">`) {
		t.Fatal("English page rendered the Chinese sidebar")
	}
	buildID := result.GeneratedAt.Format("20060102T150405.000000000-0700")
	generatedAt := result.GeneratedAt.Format(time.RFC3339Nano)
	htmlBuildID := strings.ReplaceAll(buildID, "+", "&#43;")
	htmlGeneratedAt := strings.ReplaceAll(generatedAt, "+", "&#43;")
	jsBuildLabel := result.GeneratedAt.Format("20060102-150405.000")
	jsGeneratedAtLabel := strings.ReplaceAll(result.GeneratedAt.Format("2006-01-02 15:04:05 -07:00"), "+", `\u002b`)
	urlBuildID := strings.ReplaceAll(buildID, "+", "%2b")
	for _, marker := range []string{
		`data-static-build="` + htmlBuildID + `"`,
		`data-generated-at="` + htmlGeneratedAt + `"`,
		`console.info("[CAMIE 静态化] 构建版本：` + jsBuildLabel + ` | 生成时间：` + jsGeneratedAtLabel + `")`,
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
		if !strings.Contains(page, `<div class="article-body">`) || strings.Contains(page, `<h1>`+first.Title+`</h1>`) ||
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
		"index.html":        `class="quick-rail quick-rail-global"`,
		"news.html":         `class="quick-rail quick-rail-global"`,
		"detail.html":       `class="quick-rail quick-rail-global"`,
		"pages/about.html":  `class="quick-rail quick-rail-global"`,
		"pages/member.html": `class="quick-rail quick-rail-global"`,
	} {
		page := readGenerated(t, result.Output, pageName)
		if strings.Count(page, `aria-label="快捷服务"`) != 1 || !strings.Contains(page, railClass) {
			t.Fatalf("%s did not render exactly one shared quick rail", pageName)
		}
		if pageName == "index.html" && strings.Index(page, `class="quick-rail quick-rail-global"`) < strings.Index(page, `</main>`) {
			t.Fatal("homepage quick rail must stay outside the content grid")
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
	for _, name := range []string{"pages/about.html", fmt.Sprintf("list/%d/1.html", branchColumnID(source, "about-introduction"))} {
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
	for _, label := range []string{"党建要闻", "政策文件", "会员动态", "国际交流与合作", "绿色技术推广", "协会章程"} {
		if !regexp.MustCompile(`<div class="nav-submenu">.*?<a href="[^"]+">` + label + `</a>`).MatchString(homePage) {
			t.Fatalf("primary navigation submenu missing linked item %q", label)
		}
	}
	aboutMenu := regexp.MustCompile(`(?s)<a class="nav-link"[^>]*>关于协会</a><div class="nav-submenu">(.*?)</div>`).FindStringSubmatch(homePage)
	if len(aboutMenu) != 2 || !strings.Contains(aboutMenu[1], "分支机构") || !strings.Contains(aboutMenu[1], "专家委员会") {
		t.Fatal("about navigation must include branches and expert committee")
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
		"index.html":                   {`href="#coming-soon"`, `>会员中心</a>`, `href="./videos.html"`},
		"videos.html":                  {`href="#coming-soon"`, `>会员专享</a>`},
		"pages/member.html":            {`data-member-list-shell`, `data-member-columns`, `data-member-login-path="/member-entry/login?site=camie"`, `data-member-auth-gate`, `js/member.js`},
		"pages/member-statistics.html": {`data-member-statistics-dashboard`, `data-member-login-path="/member-entry/login?site=camie"`, `data-member-auth-gate`, `js/member-statistics.js`},
		"pages/member-detail.html":     {`data-member-detail-shell`, `data-member-breadcrumb`, `data-member-login-path="/member-entry/login?site=camie"`, `data-member-auth-gate`, `js/member.js`},
		"js/main.js":                   {`a[href="#coming-soon"]`, `showToast('敬请期待')`},
		"js/member.js":                 {`memberSpaceItem`, `我的空间`, `/business_member/member/dashboard`, `member-statistics.html`, `a.target = '_blank'`, `location.replace(loginURL())`},
		"js/member-statistics.js":      {`/business_portal/api/member-zone/statistics/dashboard`, `/business_member/api/member/profile`, `window.mountEnvironmentDashboard`},
	} {
		page := readGenerated(t, result.Output, name)
		for _, marker := range markers {
			if !strings.Contains(page, marker) {
				t.Fatalf("%s missing %q", name, marker)
			}
		}
	}
	if got := strings.Count(readGenerated(t, result.Output, "index.html"), `href="#coming-soon"`); got != 3 {
		t.Fatalf("homepage coming-soon links = %d, want 3", got)
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
	for _, column := range source.Columns {
		if !strings.HasPrefix(column.Code, "main-branch-") {
			continue
		}
		want := fmt.Sprintf("./list/%d/1.html", column.ID)
		if !seenBranchLinks[want] {
			t.Fatalf("homepage branch %s does not link to its own list page %s", column.Name, want)
		}
	}
	if !strings.Contains(branchHome, fmt.Sprintf(`<h2 class="section-title"><a href="./list/%d/1.html"><span class="section-title-logo" aria-hidden="true"></span>分支机构</a>`, branchColumnID(source, "main-branch-water"))) ||
		!strings.Contains(branchHome, fmt.Sprintf(`<h2 class="section-title"><a href="./list/%d/1.html"><span class="section-title-logo" aria-hidden="true"></span>专家视野</a>`, branchColumnID(source, "home-expert-insights"))) {
		t.Fatal("homepage section headings do not open their section pages")
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
	if !strings.Contains(noticeDetail, `<div class="article-cover"><img src="https://example.com/conference-cover.jpg"`) {
		t.Fatal("article detail did not render its real cover before the body")
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
	expectedCurrentBranch := fmt.Sprintf(`class="current"><a href="../../../list/%d/1.html" aria-current="page">%s</a>`, branchColumn.ID, branchColumn.Name)
	branchBreadcrumb := regexp.MustCompile(`<nav class="breadcrumb"[^>]*>.*?</nav>`).FindString(branchDetail)
	if !strings.Contains(branchDetail, expectedCurrentBranch) || !strings.Contains(branchBreadcrumb, `>关于协会</a>`) {
		t.Fatal("branch article did not retain the about association hierarchy")
	}
	branchDefault := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", branchParent.ID))
	if !strings.Contains(branchDefault, `<div class="article-body"><p>水分会专属简介</p>`) || !strings.Contains(branchDefault, fmt.Sprintf(`class="current"><a href="../../list/%d/1.html" aria-current="page">%s</a>`, branchColumn.ID, branchColumn.Name)) || strings.Contains(branchDefault, `class="branch-directory"`) {
		t.Fatal("about branches page did not default to the first branch introduction")
	}
	branchIntro := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", branchColumn.ID))
	if !strings.Contains(branchIntro, `<div class="article-body"><p>水分会专属简介</p>`) || strings.Contains(branchIntro, `<h1>`) || strings.Contains(branchIntro, `class="news-list"`) || strings.Contains(branchIntro, waterItems[1].Title) {
		t.Fatal("branch page did not render only its first article body like the association introduction")
	}
	listPage := readGenerated(t, result.Output, fmt.Sprintf("list/%d/1.html", noticeColumn.ID))
	for _, marker := range []string{`<nav class="pagination" aria-label="分页">`, `class="plain disabled"`, `class="page-number active"`, `class="page-jump"`, `data-page-prefix="../../list/`} {
		if !strings.Contains(listPage, marker) {
			t.Fatalf("column pagination lost original structure %q", marker)
		}
	}
	topNews := readGenerated(t, result.Output, "list/13/1.html")
	if strings.Contains(topNews, `共 0 项数据`) || !strings.Contains(topNews, `class="news-row searchable"`) {
		t.Fatal("top-level column page did not show its first second-level column")
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
	if count := strings.Count(noticePanel[1], `<time datetime="`); count != 8 {
		t.Fatalf("home notice publication dates = %d, want 8", count)
	}
	for _, marker := range []string{
		`class="home-tabs" data-group="notice"`,
		`<button class="active" data-tab="notice-1"`,
		`class="hero-dots"`,
		`class="quick-rail quick-rail-global"`,
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
	for _, name := range []string{"中华人民共和国国家发展和改革委员会", "中共中央社会工作部", "中华人民共和国财政部", "中华人民共和国民政部", "中华人民共和国工业和信息化部", "中华人民共和国生态环境部", "中华人民共和国科学技术部", "国家知识产权局"} {
		if !strings.Contains(friendLinks, name) {
			t.Fatalf("fixed friend links lost %q", name)
		}
	}
	if count := strings.Count(home, `class="partner-logo"`); count != 12 {
		t.Fatalf("rendered partner logo count = %d, want 12", count)
	}
	for _, viewBox := range []string{"12 8 219 50", "0 0 183 48", "0 0 785 189", "58 18 136 30", "9 13 224 40", "61 7 122 52", "48 10 148 46", "5 10 166 60", "17 18 209 29", "0 0 418 55", "55 11 136 44", "0 0 169 55"} {
		if count := strings.Count(home, `viewBox="`+viewBox+`"`); count != 1 {
			t.Fatalf("partner logo crop %q rendered %d times, want 1", viewBox, count)
		}
	}
	if !strings.Contains(home, `href="https://www.bdc.cn/"`) || !strings.Contains(home, `partner-logos/beijing-drainage.png`) {
		t.Fatal("Beijing Drainage partner logo or target is missing")
	}
	if strings.Contains(home, `partner-logos/longking.png`) || strings.Contains(home, `访问龙净环保股份有限公司官网`) {
		t.Fatal("obsolete Longking partner logo remains")
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
				if check.directory == "list" && strings.Contains(string(page), `class="about-page english-about-page"`) && (marker == `<aside><ul class="side-menu">` || marker == `class="is-active"`) {
					continue
				}
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

func branchColumnID(source *demo.Source, code string) int64 {
	for _, column := range source.Columns {
		if column.Code == code {
			return column.ID
		}
	}
	return 0
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
	if !strings.Contains(page, `<p>暂无公开内容</p>`) || strings.Contains(page, `<h1>协会章程</h1>`) ||
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
