package media

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	ModeSameOrigin = "same_origin"
	ModeCDN        = "cdn"
)

type Config struct {
	Mode           string            `yaml:"mode"`
	BaseURL        string            `yaml:"base_url,omitempty"`
	Aliases        map[string]string `yaml:"aliases,omitempty"`
	RewriteOrigins []string          `yaml:"rewrite_origins,omitempty"`
}

type Resolver struct {
	mode           string
	base           *url.URL
	aliases        []alias
	rewriteOrigins map[string]struct{}
}

type alias struct {
	from string
	to   string
}

func New(cfg Config) (*Resolver, error) {
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = ModeSameOrigin
	}
	if mode != ModeSameOrigin && mode != ModeCDN {
		return nil, fmt.Errorf("unsupported media mode %q", mode)
	}
	resolver := &Resolver{mode: mode, rewriteOrigins: make(map[string]struct{})}
	if mode == ModeCDN {
		base, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
		if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
			return nil, errors.New("media.base_url must be an absolute HTTP(S) URL in cdn mode")
		}
		base.RawQuery, base.Fragment = "", ""
		if !strings.HasSuffix(base.Path, "/") {
			base.Path += "/"
		}
		resolver.base = base
	}
	for rawFrom, rawTo := range cfg.Aliases {
		from := cleanPrefix(rawFrom)
		to := "/" + cleanPrefix(rawTo)
		if from == "" || to == "/" {
			return nil, errors.New("media aliases require non-empty source and destination paths")
		}
		resolver.aliases = append(resolver.aliases, alias{from: from, to: strings.TrimRight(to, "/")})
	}
	sort.Slice(resolver.aliases, func(i, j int) bool { return len(resolver.aliases[i].from) > len(resolver.aliases[j].from) })
	for _, raw := range cfg.RewriteOrigins {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("invalid media rewrite origin %q", raw)
		}
		resolver.rewriteOrigins[strings.ToLower(parsed.Scheme+"://"+parsed.Host)] = struct{}{}
	}
	return resolver, nil
}

func (r *Resolver) Resolve(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return raw
		}
		origin := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
		if _, ok := r.rewriteOrigins[origin]; !ok {
			return raw
		}
		parsed.Scheme, parsed.Host = "", ""
		raw = parsed.String()
	}
	if strings.HasPrefix(raw, "//") {
		return raw
	}
	parsed, err = url.Parse(raw)
	if err != nil {
		return raw
	}
	clean := cleanRelative(parsed.Path)
	for _, item := range r.aliases {
		if clean == item.from || strings.HasPrefix(clean, item.from+"/") {
			suffix := strings.TrimPrefix(clean, item.from)
			parsed.Path = item.to + suffix
			parsed.RawPath = ""
			return r.applyMode(parsed)
		}
	}
	if strings.HasPrefix(parsed.Path, "/") {
		return r.applyMode(parsed)
	}
	return parsed.String()
}

func (r *Resolver) applyMode(parsed *url.URL) string {
	if r.mode == ModeSameOrigin || r.base == nil {
		if !strings.HasPrefix(parsed.Path, "/") {
			parsed.Path = "/" + parsed.Path
		}
		return parsed.String()
	}
	reference := *parsed
	reference.Path = strings.TrimPrefix(reference.Path, "/")
	return r.base.ResolveReference(&reference).String()
}

func (r *Resolver) RewriteHTML(content string) string {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(content), context)
	if err != nil {
		return content
	}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for index := range node.Attr {
				key := strings.ToLower(node.Attr[index].Key)
				isMedia := (node.Data == "img" || node.Data == "video" || node.Data == "source") && key == "src"
				isPoster := node.Data == "video" && key == "poster"
				isLink := node.Data == "a" && key == "href" && r.isManaged(node.Attr[index].Val)
				if isMedia || isPoster || isLink {
					node.Attr[index].Val = r.Resolve(node.Attr[index].Val)
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

func (r *Resolver) isManaged(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.IsAbs() {
		_, ok := r.rewriteOrigins[strings.ToLower(parsed.Scheme+"://"+parsed.Host)]
		return ok
	}
	clean := cleanRelative(parsed.Path)
	for _, item := range r.aliases {
		if clean == item.from || strings.HasPrefix(clean, item.from+"/") {
			return true
		}
	}
	return false
}

func cleanPrefix(raw string) string { return strings.Trim(cleanRelative(raw), "/") }

func cleanRelative(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), `\`, "/")
	for strings.HasPrefix(raw, "../") {
		raw = strings.TrimPrefix(raw, "../")
	}
	raw = strings.TrimLeft(strings.TrimPrefix(raw, "./"), "/")
	clean := path.Clean("/" + raw)
	return strings.TrimPrefix(clean, "/")
}
