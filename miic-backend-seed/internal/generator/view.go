package generator

import (
	"bytes"
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"miic-portal/backend/internal/config"
	"miic-portal/backend/internal/model"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func articleRelativePath(article model.Article) string {
	return path.Join("article", article.PublishTime.Format("2006/01"), fmt.Sprintf("%d.html", article.ID))
}

func safeExternal(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return parsed.String(), true
}

func mediaURL(cfg config.Config, raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		raw = cfg.Site.FallbackCover
	}
	if parsed, err := url.Parse(raw); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return parsed.String()
	}
	if normalized, ok := normalizeUploadURL(raw); ok {
		return normalized
	}
	raw = strings.TrimPrefix(raw, "./")
	if cfg.Site.MediaBaseURL == "" {
		return raw
	}
	base := strings.TrimRight(cfg.Site.MediaBaseURL, "/")
	return base + "/" + strings.TrimLeft(raw, "/")
}

// normalizeUploadURL maps legacy editor-relative upload paths to the public
// path served by Nginx. Article pages are nested several directories deep, so
// keeping values such as ../../../mic/uploads/... would make the same content
// resolve differently depending on the generated page location.
func normalizeUploadURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")))
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return raw, false
	}
	clean := parsed.Path
	for strings.HasPrefix(clean, "../") {
		clean = strings.TrimPrefix(clean, "../")
	}
	clean = strings.TrimLeft(strings.TrimPrefix(clean, "./"), "/")

	var suffix string
	switch {
	case clean == "mic/uploads", clean == "miic/uploads":
	case strings.HasPrefix(clean, "mic/uploads/"):
		suffix = strings.TrimPrefix(clean, "mic/uploads")
	case strings.HasPrefix(clean, "miic/uploads/"):
		suffix = strings.TrimPrefix(clean, "miic/uploads")
	default:
		return raw, false
	}
	parsed.Path = "/miic/uploads" + suffix
	parsed.RawPath = ""
	return parsed.String(), true
}

func normalizeContentMediaURLs(content string) string {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(content), context)
	if err != nil {
		return content
	}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for i := range node.Attr {
				switch strings.ToLower(node.Attr[i].Key) {
				case "href", "src", "poster":
					if normalized, ok := normalizeUploadURL(node.Attr[i].Val); ok {
						node.Attr[i].Val = normalized
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}

	var output bytes.Buffer
	for _, node := range nodes {
		visit(node)
		if err := html.Render(&output, node); err != nil {
			return content
		}
	}
	return output.String()
}

func makeView(cfg config.Config, category string, article model.Article) ArticleView {
	href, external := safeExternal(article.URL)
	if !external && model.HasStaticDetail(article.Type) {
		href = articleRelativePath(article)
	} else if !external {
		href = ""
	}
	return ArticleView{ID: article.ID, Category: category, Title: article.Title, Summary: article.Summary, Cover: mediaURL(cfg, article.Cover), Href: href,
		Author: article.Author, Source: article.Source, DateISO: article.PublishTime.Format("2006-01-02"), DateDot: article.PublishTime.Format("2006.01.02"),
		DateCN: fmt.Sprintf("%d年%d月%d日", article.PublishTime.Year(), article.PublishTime.Month(), article.PublishTime.Day()), External: external, Bold: article.IsBold, Color: article.DefaultColor}
}

func mergeArticles(groups [][]model.Article) []model.Article {
	seen := map[int64]bool{}
	var result []model.Article
	for _, group := range groups {
		for _, item := range group {
			if !seen[item.ID] {
				seen[item.ID] = true
				result = append(result, item)
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].IsTop != result[j].IsTop {
			return result[i].IsTop
		}
		if !result[i].PublishTime.Equal(result[j].PublishTime) {
			return result[i].PublishTime.After(result[j].PublishTime)
		}
		return result[i].ID > result[j].ID
	})
	return result
}

func roundDuration(started time.Time) float64 {
	seconds := math.Round(time.Since(started).Seconds()*1000) / 1000
	if seconds < 0.001 {
		return 0.001
	}
	return seconds
}
