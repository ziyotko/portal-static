package caamdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPortalDatabaseKeepsPreambleAndOnlyCAAMPortal(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dump.sql")
	destination := filepath.Join(dir, "portal.sql")
	dump := "SET @OLD=1;\r\n-- Current Database: `other`\r\nCREATE DATABASE other;\r\n-- Current Database: `caam_portal`\r\nCREATE DATABASE caam_portal;\r\nUSE caam_portal;\r\n-- Current Database: `later`\r\nCREATE DATABASE later;\r\n"
	if err := os.WriteFile(source, []byte(dump), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractPortalDatabase(source, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "SET @OLD=1") || !strings.Contains(text, "CREATE DATABASE caam_portal") {
		t.Fatalf("missing preamble or portal section: %s", text)
	}
	if strings.Contains(text, "CREATE DATABASE other") || strings.Contains(text, "CREATE DATABASE later") {
		t.Fatalf("unrelated database leaked into extraction: %s", text)
	}
}

func TestExtractPortalDatabaseRejectsMissingSection(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dump.sql")
	if err := os.WriteFile(source, []byte("-- Current Database: `other`\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractPortalDatabase(source, filepath.Join(dir, "portal.sql")); err == nil {
		t.Fatal("expected missing caam_portal error")
	}
}

func TestValidateOutputRejectsLegacyOriginsAndDraftArtifacts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html", "list/1/1.html", "article/2026/09/1.html"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("safe"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateOutput(root, 1, []int64{2}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("https://demo.miic.com.cn/caamm/uploads/a.jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateOutput(root, 1, []int64{2}); err == nil {
		t.Fatal("expected legacy origin rejection")
	}
}
