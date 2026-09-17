package generator

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	nethtml "golang.org/x/net/html"
)

// validateSiteInternalLinks checks links emitted by generated main and list
// pages before the staging directory is published. Static scaffold links are
// intentionally ignored here; this validates the generated article, list and
// six main-page targets owned by the generator.
func validateSiteInternalLinks(root string, mainPages []string) (int, error) {
	root = filepath.Clean(root)
	mainTargets := make(map[string]struct{}, len(mainPages))
	sources := make([]string, 0, len(mainPages))
	for _, name := range mainPages {
		clean := filepath.Clean(name)
		mainTargets[clean] = struct{}{}
		sources = append(sources, filepath.Join(root, clean))
	}

	listRoot := filepath.Join(root, "list")
	if err := filepath.WalkDir(listRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
			sources = append(sources, path)
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("walk generated list pages: %w", err)
	}

	targetSources := make(map[string]string)
	for _, source := range sources {
		file, err := os.Open(source)
		if err != nil {
			return 0, fmt.Errorf("open generated page %s: %w", source, err)
		}
		doc, parseErr := nethtml.Parse(file)
		closeErr := file.Close()
		if parseErr != nil {
			return 0, fmt.Errorf("parse generated page %s: %w", source, parseErr)
		}
		if closeErr != nil {
			return 0, fmt.Errorf("close generated page %s: %w", source, closeErr)
		}

		var visit func(*nethtml.Node) error
		visit = func(node *nethtml.Node) error {
			if node.Type == nethtml.ElementNode && node.Data == "a" {
				for _, attribute := range node.Attr {
					if attribute.Key != "href" {
						continue
					}
					target, owned, resolveErr := generatedLinkTarget(root, source, attribute.Val, mainTargets)
					if resolveErr != nil {
						return resolveErr
					}
					if owned {
						targetSources[target] = source
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := visit(doc); err != nil {
			return 0, err
		}
	}

	for target, source := range targetSources {
		info, err := os.Stat(target)
		if err != nil {
			return 0, fmt.Errorf("generated link from %s points to missing target %s: %w", source, target, err)
		}
		if info.IsDir() || info.Size() == 0 {
			return 0, fmt.Errorf("generated link from %s points to invalid target %s", source, target)
		}
	}
	return len(targetSources), nil
}

func generatedLinkTarget(root, source, rawHref string, mainTargets map[string]struct{}) (string, bool, error) {
	rawHref = strings.TrimSpace(rawHref)
	if rawHref == "" || strings.HasPrefix(rawHref, "#") || strings.HasPrefix(rawHref, "//") {
		return "", false, nil
	}
	parsed, err := urlParse(rawHref)
	if err != nil || parsed.scheme != "" || parsed.host != "" || parsed.path == "" || strings.HasPrefix(parsed.path, "/") {
		return "", false, nil
	}
	target := filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(parsed.path)))
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", false, fmt.Errorf("resolve generated link %q from %s: %w", rawHref, source, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("generated link %q from %s escapes site root", rawHref, source)
	}
	owned := relative == "article" || strings.HasPrefix(relative, "article"+string(filepath.Separator)) ||
		relative == "list" || strings.HasPrefix(relative, "list"+string(filepath.Separator))
	if _, exists := mainTargets[relative]; exists {
		owned = true
	}
	return target, owned, nil
}

// Keeping URL parsing behind a tiny value type makes the link-validation
// traversal independent from query strings and fragments.
type parsedLink struct {
	scheme string
	host   string
	path   string
}

func urlParse(raw string) (parsedLink, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return parsedLink{}, err
	}
	return parsedLink{scheme: parsed.Scheme, host: parsed.Host, path: parsed.Path}, nil
}
