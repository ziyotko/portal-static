package generator

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrBusy = errors.New("static generation is already running")

type fileLock struct{ path string }

func acquireLock(path string, stale time.Duration) (*fileLock, error) {
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > stale {
		_ = os.Remove(path)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrBusy
	}
	if err != nil {
		return nil, fmt.Errorf("acquire generation lock: %w", err)
	}
	_ = file.Close()
	return &fileLock{path: path}, nil
}
func (l *fileLock) release() {
	if l != nil {
		_ = os.Remove(l.path)
	}
}

func publishFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".miic-file-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(name, 0o644); err != nil {
		return fmt.Errorf("set static file permissions: %w", err)
	}
	_ = os.Remove(path)
	if err = os.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func replaceDirectory(staging, target string) error {
	if err := normalizeStaticTreePermissions(staging); err != nil {
		return fmt.Errorf("set staged static permissions: %w", err)
	}
	backup := target + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup current directory: %w", err)
		}
	}
	if err := os.Rename(staging, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("publish directory: %w", err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

// normalizeStaticTreePermissions prevents MkdirTemp's private 0700 mode from
// becoming the live website root after an atomic directory rename.
func normalizeStaticTreePermissions(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		mode := os.FileMode(0o644)
		if entry.IsDir() {
			mode = 0o755
		} else if !entry.Type().IsRegular() {
			return nil
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("chmod %s to %04o: %w", path, mode, err)
		}
		return nil
	})
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return publishFile(dest, data)
	})
}

func copyScaffold(source, target string) error {
	return copyScaffoldFiles(source, target, false)
}

// ensureScaffold makes an empty output directory usable without rewriting any
// file that has already been published. Incremental generation must preserve
// live pages and shared data; otherwise a list-only refresh can replace the
// generated home-page data with the empty source placeholder.
func ensureScaffold(source, target string) error {
	return copyScaffoldFiles(source, target, true)
}

// Related refreshes are incremental too, so they share the same preservation
// rule as page, list, and article refreshes.
func copyRelatedScaffold(source, target string) error {
	return ensureScaffold(source, target)
}

func copyScaffoldFiles(source, target string, onlyMissing bool) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".html" && name != "styles.css" && name != "script.js" && name != "generated-content.js" {
			continue
		}
		if onlyMissing {
			if _, err := os.Stat(filepath.Join(target, name)); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		if err := publishFile(filepath.Join(target, name), data); err != nil {
			return err
		}
	}
	assets := filepath.Join(source, "assets")
	if _, err := os.Stat(assets); err == nil {
		if onlyMissing {
			return copyTreeMissing(assets, filepath.Join(target, "assets"))
		}
		return copyTree(assets, filepath.Join(target, "assets"))
	}
	return nil
}

func copyTreeMissing(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if _, err := os.Stat(dest); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return publishFile(dest, data)
	})
}

func grayscaleHTML(data []byte, enabled bool) []byte {
	marker := []byte(`<style id="miic-grayscale">html{filter:grayscale(1)}</style>`)
	data = bytes.ReplaceAll(data, marker, nil)
	if !enabled {
		return data
	}
	needle := []byte("</head>")
	return bytes.Replace(data, needle, append(marker, needle...), 1)
}

func grayscaleTree(root string, enabled bool) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
		return publishFile(path, grayscaleHTML(data, enabled))
	})
}
