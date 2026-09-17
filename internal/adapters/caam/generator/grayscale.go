package generator

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"portal-static/internal/contracts"
)

type grayscaleContextKey struct{}

const grayscaleMarker = `data-site-grayscale="true"`

var grayscaleStyle = []byte(`<style id="site-grayscale">html[data-site-grayscale="true"]{-webkit-filter:grayscale(100%);filter:grayscale(100%)}</style>`)

func WithGrayscale(ctx context.Context, enabled bool) context.Context {
	value := enabled
	return context.WithValue(ctx, grayscaleContextKey{}, &value)
}

func Grayscale(ctx context.Context) bool {
	value, ok := grayscaleOption(ctx)
	return ok && value
}

func grayCode(enabled bool) string {
	if enabled {
		return "1"
	}
	return "2"
}

func grayscaleOption(ctx context.Context) (bool, bool) {
	if ctx == nil {
		return false, false
	}
	value, ok := ctx.Value(grayscaleContextKey{}).(*bool)
	if ok && value != nil {
		return *value, true
	}
	options := contracts.OptionsFrom(ctx)
	return options.Grayscale, options.Grayscale
}

func withoutGrayscale(ctx context.Context) context.Context {
	return context.WithValue(ctx, grayscaleContextKey{}, (*bool)(nil))
}

func applyGrayscale(page []byte, enabled bool) ([]byte, error) {
	if !enabled || bytes.Contains(page, []byte(grayscaleMarker)) {
		return page, nil
	}
	htmlIndex := bytes.Index(page, []byte("<html"))
	if htmlIndex < 0 {
		return nil, fmt.Errorf("rendered page is missing html required for grayscale")
	}
	htmlEnd := bytes.IndexByte(page[htmlIndex:], '>')
	if htmlEnd < 0 {
		return nil, fmt.Errorf("rendered page has invalid html tag required for grayscale")
	}
	htmlEnd += htmlIndex
	page = insertBytes(page, htmlEnd, []byte(" "+grayscaleMarker))
	headEnd := bytes.Index(page, []byte("</head>"))
	if headEnd < 0 {
		return nil, fmt.Errorf("rendered page is missing head required for grayscale")
	}
	page = insertBytes(page, headEnd, grayscaleStyle)
	return page, nil
}

func insertBytes(source []byte, index int, value []byte) []byte {
	result := make([]byte, 0, len(source)+len(value))
	result = append(result, source[:index]...)
	result = append(result, value...)
	return append(result, source[index:]...)
}

func applyGrayscaleToHTMLTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
			return nil
		}
		page, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		page, err = applyGrayscale(page, true)
		if err != nil {
			return fmt.Errorf("apply grayscale to %s: %w", path, err)
		}
		return os.WriteFile(path, page, 0o644)
	})
}
