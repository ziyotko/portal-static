package camie

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coreconfig "portal-static/internal/core/config"
)

func TestFileTemplateSourceUsesPackagedTemplatesWithoutDatabase(t *testing.T) {
	root, err := filepath.Abs("testdata/templates")
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, key := range []string{"layout", "home", "list", "article", "about"} {
		paths[key] = filepath.Join(root, key+".html.tmpl")
	}
	snapshot := coreconfig.Snapshot{Document: coreconfig.Document{Paths: coreconfig.PathsConfig{Templates: paths}}}
	got, cleanup, err := productionTemplateSnapshot(context.Background(), snapshot, nil, "file")
	if err != nil {
		t.Fatal(err)
	}
	if got.Paths.Templates["home"] != paths["home"] {
		t.Fatalf("file template path changed: %q", got.Paths.Templates["home"])
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestFileTemplateSourceRejectsBrokenBundle(t *testing.T) {
	root := t.TempDir()
	paths := map[string]string{}
	for _, key := range []string{"layout", "home", "list", "article", "about"} {
		path := filepath.Join(root, key+".html.tmpl")
		if err := os.WriteFile(path, []byte(`{{define "`+key+`"}}ok{{end}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		paths[key] = path
	}
	snapshot := coreconfig.Snapshot{Document: coreconfig.Document{Paths: coreconfig.PathsConfig{Templates: paths}}}
	_, _, err := productionTemplateSnapshot(context.Background(), snapshot, nil, "file")
	if err == nil || !strings.Contains(err.Error(), `definition "head" is missing`) {
		t.Fatalf("missing layout definition error = %v", err)
	}
}
