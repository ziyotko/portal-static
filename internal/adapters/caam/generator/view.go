package generator

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
	"portal-static/internal/core/media"
)

var (
	colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)
	htmlPattern  = regexp.MustCompile(`<[^>]*>`)
)

type PageData struct {
	GeneratedAt string
	Headline    *ArticleView
	Carousel    []ArticleView
	TopNews     []NewsGroupView
	Work        []TabView
	Industry    []TabView
	Stats       []StatView
	InitialStat StatView
	Topics      []ArticleView
	Videos      []ArticleView
	FooterLinks []LinkGroupView
}

type ArticleView struct {
	Index        int
	ID           string
	Title        string
	Summary      string
	Cover        string
	Source       string
	Href         string
	Clickable    bool
	Bold         bool
	Color        template.CSS
	DateISO      string
	DateCN       string
	DateShort    string
	Published    bool
	ExternalLink bool
}

type NewsGroupView struct {
	Key   string
	Title string
	Href  string
	Lead  *ArticleView
	Items []ArticleView
}

type TabView struct {
	Key      string
	Title    string
	Href     string
	Cover    string
	CoverAlt string
	Items    []ArticleView
}

type LinkGroupView struct {
	Title string
	Items []ArticleView
}

type viewBuilder struct {
	media    *media.Resolver
	location *time.Location
}

func newViewBuilder(mediaBaseURL string, location *time.Location) viewBuilder {
	cfg := media.Config{Mode: media.ModeSameOrigin, Aliases: defaultMediaAliases()}
	if baseURL := strings.TrimSpace(mediaBaseURL); baseURL != "" {
		cfg.Mode, cfg.BaseURL = media.ModeCDN, baseURL
	}
	resolver, _ := media.New(cfg)
	return viewBuilder{media: resolver, location: location}
}

func newViewBuilderForConfig(cfg config.Config, location *time.Location) viewBuilder {
	resolver, _ := media.New(cfg.EffectiveMedia())
	return viewBuilder{media: resolver, location: location}
}

func defaultMediaAliases() map[string]string {
	return map[string]string{
		"caamm/uploads": "/caam/uploads",
		"caam/uploads":  "/caam/uploads",
		"uploads":       "/uploads",
	}
}

func (b viewBuilder) article(article model.Article, index int, fallbackCover string) ArticleView {
	view := ArticleView{
		Index:   index + 1,
		ID:      article.ID,
		Title:   strings.TrimSpace(article.Title),
		Summary: truncateText(article.Summary, 110),
		Cover:   b.resolveCover(article.Cover, fallbackCover),
		Source:  strings.TrimSpace(article.Source),
		Bold:    article.IsBold,
	}
	if colorPattern.MatchString(strings.TrimSpace(article.DefaultColor)) {
		view.Color = template.CSS(strings.TrimSpace(article.DefaultColor))
	}
	if href, external, ok := homepageArticleHref(article, b.location); ok {
		view.Href = href
		view.Clickable = true
		view.ExternalLink = external
	}
	if !article.PublishTime.IsZero() {
		published := article.PublishTime.In(b.location)
		view.Published = true
		view.DateISO = published.Format("2006-01-02")
		view.DateCN = published.Format("2006年1月2日")
		view.DateShort = published.Format("2006.1.2")
	}
	return view
}

func homepageArticleHref(article model.Article, location *time.Location) (string, bool, bool) {
	if !model.IsSupportedArticleType(article.Type) {
		return "", false, false
	}
	rawHref := strings.TrimSpace(article.URL)
	if href, external, ok := safeHref(rawHref); ok && external {
		return href, true, true
	}
	if rawHref != "" {
		if _, _, ok := safeHref(rawHref); !ok {
			return "", false, false
		}
	}
	if strings.TrimSpace(article.ID) != "" && model.HasStaticDetail(article.Type) {
		if href, ok := articleArchiveHref(article, location); ok {
			return href + "?from=home", false, true
		}
		return "", false, false
	}
	return safeHref(rawHref)
}

func articleArchiveHref(article model.Article, location *time.Location) (string, bool) {
	articleID := strings.TrimSpace(article.ID)
	if articleID == "" || article.PublishTime.IsZero() {
		return "", false
	}
	published := article.PublishTime
	if location != nil {
		published = published.In(location)
	}
	return "article/" + published.Format("2006/01") + "/" + url.PathEscape(articleID) + ".html", true
}

func (b viewBuilder) articles(articles []model.Article, fallbackCover string) []ArticleView {
	result := make([]ArticleView, 0, len(articles))
	for i, article := range articles {
		result = append(result, b.article(article, i, fallbackCover))
	}
	return result
}

func (b viewBuilder) resolveCover(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fallback
	}
	if parsed.IsAbs() {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return b.media.Resolve(raw)
		}
		return fallback
	}
	if strings.HasPrefix(raw, "//") || strings.Contains(raw, `\`) {
		return fallback
	}
	return b.media.Resolve(raw)
}

func normalizeUploadPath(raw string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return normalized
	}
	clean := parsed.Path
	for strings.HasPrefix(clean, "../") {
		clean = strings.TrimPrefix(clean, "../")
	}
	clean = strings.TrimLeft(strings.TrimPrefix(clean, "./"), "/")
	switch {
	case clean == "caam/uploads", clean == "caamm/uploads":
		parsed.Path = "/caam/uploads"
	case strings.HasPrefix(clean, "caam/uploads/"):
		parsed.Path = "/caam/uploads/" + strings.TrimPrefix(clean, "caam/uploads/")
	case strings.HasPrefix(clean, "caamm/uploads/"):
		parsed.Path = "/caam/uploads/" + strings.TrimPrefix(clean, "caamm/uploads/")
	case clean == "uploads":
		parsed.Path = "/uploads"
	case strings.HasPrefix(clean, "uploads/"):
		parsed.Path = "/" + clean
	default:
		return normalized
	}
	parsed.RawPath = ""
	return parsed.String()
}

func safeHref(raw string) (string, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "//") || strings.Contains(raw, `\`) {
		return "", false, false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, false
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", false, false
		}
		return raw, true, true
	}
	if parsed.Host != "" {
		return "", false, false
	}
	return raw, false, true
}

func truncateText(value string, maxRunes int) string {
	value = html.UnescapeString(htmlPattern.ReplaceAllString(value, ""))
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "…"
}

func buildGroups(slots []config.SlotConfig, data map[string][]model.Article, builder viewBuilder, listColumnIDs map[string]int64) []NewsGroupView {
	groups := make([]NewsGroupView, 0, len(slots))
	for _, slot := range slots {
		items := builder.articles(data[slot.Key], slot.FallbackCover)
		group := NewsGroupView{Key: slot.Key, Title: slot.Title, Href: slotListHref(slot, data[slot.Key], listColumnIDs[slot.Key])}
		if len(items) > 0 {
			lead := items[0]
			group.Lead = &lead
			group.Items = items[1:]
		}
		groups = append(groups, group)
	}
	return groups
}

func buildTabs(slots []config.SlotConfig, data map[string][]model.Article, builder viewBuilder, listColumnIDs map[string]int64) []TabView {
	tabs := make([]TabView, 0, len(slots))
	for _, slot := range slots {
		items := builder.articles(data[slot.Key], slot.FallbackCover)
		cover := slot.FallbackCover
		coverAlt := slot.Title + "栏目封面"
		if len(items) > 0 && items[0].Cover != "" {
			cover = items[0].Cover
			coverAlt = items[0].Title
		}
		tabs = append(tabs, TabView{Key: slot.Key, Title: slot.Title, Href: slotListHref(slot, data[slot.Key], listColumnIDs[slot.Key]), Cover: cover, CoverAlt: coverAlt, Items: items})
	}
	return tabs
}

func slotListHref(slot config.SlotConfig, articles []model.Article, resolvedColumnID int64) string {
	columnID := resolvedColumnID
	if columnID <= 0 {
		for _, article := range articles {
			if article.ColumnID > 0 {
				columnID = article.ColumnID
				break
			}
		}
	}
	if columnID > 0 {
		return "list/" + strconv.FormatInt(columnID, 10) + "/1.html"
	}
	return ""
}

func buildLinkGroups(slots []config.SlotConfig, data map[string][]model.Article, builder viewBuilder) []LinkGroupView {
	groups := make([]LinkGroupView, 0, len(slots))
	for _, slot := range slots {
		items := builder.articles(data[slot.Key], "")
		links := make([]ArticleView, 0, len(items))
		for _, item := range items {
			if item.Clickable {
				links = append(links, item)
			}
		}
		groups = append(groups, LinkGroupView{Title: slot.Title, Items: links})
	}
	return groups
}
