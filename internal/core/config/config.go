package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"portal-static/internal/core/media"

	"gopkg.in/yaml.v3"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type Document struct {
	Version  int            `yaml:"version"`
	Driver   string         `yaml:"driver"`
	Site     SiteConfig     `yaml:"site"`
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	Paths    PathsConfig    `yaml:"paths"`
	Media    media.Config   `yaml:"media"`
	Adapter  yaml.Node      `yaml:"adapter"`
}

type SiteConfig struct {
	ID       string `yaml:"id"`
	Timezone string `yaml:"timezone"`
}

type DatabaseConfig struct {
	DSNEnv          string `yaml:"dsn_env"`
	Schema          string `yaml:"schema"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type ServerConfig struct {
	Addr             string `yaml:"addr"`
	TokenEnv         string `yaml:"token_env"`
	RequestTimeout   string `yaml:"request_timeout"`
	BatchIdleTimeout string `yaml:"batch_idle_timeout"`
	BatchMaxDuration string `yaml:"batch_max_duration"`
}

type PathsConfig struct {
	SourceRoot  string            `yaml:"source_root"`
	DistRoot    string            `yaml:"dist_root"`
	PreviewRoot string            `yaml:"preview_root,omitempty"`
	Templates   map[string]string `yaml:"templates"`
	Assets      []string          `yaml:"assets,omitempty"`
}

type Snapshot struct {
	Document
	Path    string
	Hash    string
	ModTime time.Time
}

type Info struct {
	Path            string    `json:"path"`
	CurrentHash     string    `json:"current_hash"`
	CurrentModTime  time.Time `json:"current_modified_at"`
	LastUsedHash    string    `json:"last_used_hash,omitempty"`
	LastUsedAt      time.Time `json:"last_used_at,omitempty"`
	RestartRequired bool      `json:"restart_required"`
	ValidationError string    `json:"validation_error,omitempty"`
}

type Manager struct {
	path      string
	bootstrap Snapshot
	mu        sync.RWMutex
	info      Info
}

func Open(path string) (*Manager, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	snapshot, err := readSnapshot(abs)
	if err != nil {
		return nil, err
	}
	return &Manager{
		path: abs, bootstrap: snapshot,
		info: Info{Path: abs, CurrentHash: snapshot.Hash, CurrentModTime: snapshot.ModTime},
	}, nil
}

func (m *Manager) Bootstrap() Snapshot { return m.bootstrap }

func (m *Manager) Load() (Snapshot, error) {
	snapshot, err := readSnapshot(m.path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.info.ValidationError = err.Error()
		return Snapshot{}, err
	}
	m.info.CurrentHash = snapshot.Hash
	m.info.CurrentModTime = snapshot.ModTime
	m.info.ValidationError = ""
	if snapshot.Driver != m.bootstrap.Driver || snapshot.Site.ID != m.bootstrap.Site.ID ||
		!reflect.DeepEqual(snapshot.Server, m.bootstrap.Server) || !reflect.DeepEqual(snapshot.Database, m.bootstrap.Database) {
		m.info.RestartRequired = true
		return Snapshot{}, errors.New("driver, site identity, server, or database configuration changed; restart required")
	}
	m.info.RestartRequired = false
	m.info.LastUsedHash = snapshot.Hash
	m.info.LastUsedAt = time.Now()
	return snapshot, nil
}

func (m *Manager) Info() Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.info
}

func readSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read config: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Snapshot{}, fmt.Errorf("parse config: %w", err)
	}
	if err := resolvePaths(filepath.Dir(path), &document.Paths); err != nil {
		return Snapshot{}, err
	}
	if err := document.Validate(); err != nil {
		return Snapshot{}, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, err
	}
	digest := sha256.Sum256(data)
	return Snapshot{Document: document, Path: path, Hash: hex.EncodeToString(digest[:]), ModTime: stat.ModTime()}, nil
}

func (d Document) Validate() error {
	if d.Version != 1 {
		return fmt.Errorf("unsupported config version %d", d.Version)
	}
	if strings.TrimSpace(d.Driver) == "" || strings.TrimSpace(d.Site.ID) == "" {
		return errors.New("driver and site.id are required")
	}
	if d.Site.Timezone == "" {
		return errors.New("site.timezone is required")
	}
	if _, err := time.LoadLocation(d.Site.Timezone); err != nil {
		return fmt.Errorf("invalid site.timezone: %w", err)
	}
	if d.Server.Addr == "" || d.Server.TokenEnv == "" {
		return errors.New("server.addr and server.token_env are required")
	}
	for label, raw := range map[string]string{
		"server.request_timeout":    d.Server.RequestTimeout,
		"server.batch_idle_timeout": d.Server.BatchIdleTimeout,
		"server.batch_max_duration": d.Server.BatchMaxDuration,
	} {
		if _, err := time.ParseDuration(raw); err != nil {
			return fmt.Errorf("parse %s: %w", label, err)
		}
	}
	if d.Paths.SourceRoot == "" || d.Paths.DistRoot == "" || len(d.Paths.Templates) == 0 {
		return errors.New("paths.source_root, paths.dist_root and paths.templates are required")
	}
	if filepath.Clean(d.Paths.SourceRoot) == filepath.Clean(d.Paths.DistRoot) {
		return errors.New("paths.source_root and paths.dist_root must differ")
	}
	if _, err := media.New(d.Media); err != nil {
		return err
	}
	return nil
}

// ValidateMySQL validates the optional shared CMS MySQL configuration. It is
// deliberately separate from Document.Validate so preview and future adapters
// backed by other data sources do not inherit a MySQL requirement.
func (d DatabaseConfig) ValidateMySQL() error {
	if strings.TrimSpace(d.DSNEnv) == "" || !identifierPattern.MatchString(d.Schema) {
		return errors.New("database.dsn_env and a safe database.schema are required for portal CMS MySQL")
	}
	if d.MaxOpenConns <= 0 {
		return errors.New("database.max_open_conns must be positive")
	}
	if d.MaxIdleConns < 0 || d.MaxIdleConns > d.MaxOpenConns {
		return errors.New("database.max_idle_conns must be between zero and max_open_conns")
	}
	if _, err := time.ParseDuration(d.ConnMaxLifetime); err != nil {
		return fmt.Errorf("parse database.conn_max_lifetime: %w", err)
	}
	return nil
}

func resolvePaths(base string, paths *PathsConfig) error {
	resolve := func(value string, relativeTo string) string {
		if value == "" || filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Clean(filepath.Join(relativeTo, value))
	}
	paths.SourceRoot = resolve(paths.SourceRoot, base)
	paths.DistRoot = resolve(paths.DistRoot, base)
	paths.PreviewRoot = resolve(paths.PreviewRoot, base)
	for name, value := range paths.Templates {
		paths.Templates[name] = resolve(value, paths.SourceRoot)
	}
	for index, value := range paths.Assets {
		paths.Assets[index] = resolve(value, paths.SourceRoot)
	}
	return nil
}
