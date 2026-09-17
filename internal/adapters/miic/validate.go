package miic

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var localReference = regexp.MustCompile(`(?i)(?:href|src)="([^"]+)"`)

func validateSite(root string) error {
	for _, required := range []string{"index.html", "news.html", "business.html", "platforms.html", "about.html", "platform-detail.html", "service-detail.html", "detail.html", "styles.css", "script.js", "generated-content.js"} {
		info, err := os.Stat(filepath.Join(root, required))
		if err != nil {
			return fmt.Errorf("required output %s: %w", required, err)
		}
		if info.IsDir() || info.Size() == 0 {
			return fmt.Errorf("required output %s is empty", required)
		}
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(path)) != ".html" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range localReference.FindAllStringSubmatch(string(data), -1) {
			raw := strings.TrimSpace(match[1])
			if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") || strings.HasPrefix(raw, "data:") {
				continue
			}
			parsed, err := url.Parse(raw)
			if err != nil || parsed.IsAbs() {
				continue
			}
			clean := filepath.FromSlash(parsed.Path)
			if clean == "" {
				continue
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(path), clean))
			if !strings.HasPrefix(strings.ToLower(target), strings.ToLower(filepath.Clean(root)+string(filepath.Separator))) {
				return fmt.Errorf("internal link escapes site: %s in %s", raw, path)
			}
			if _, err := os.Stat(target); err != nil {
				return fmt.Errorf("broken internal link %s in %s", raw, path)
			}
		}
		return nil
	})
}
