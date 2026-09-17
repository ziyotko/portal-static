package media

import (
	"strings"
	"testing"
)

func TestSameOriginAliasesAndLegacyOrigin(t *testing.T) {
	resolver, err := New(Config{
		Mode: ModeSameOrigin,
		Aliases: map[string]string{
			"mic/uploads":  "/miic/uploads",
			"miic/uploads": "/miic/uploads",
		},
		RewriteOrigins: []string{"https://demo.miic.com.cn"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for input, expected := range map[string]string{
		"../../../mic/uploads/a.png?x=1#p":          "/miic/uploads/a.png?x=1#p",
		"/miic/uploads/a.png":                     "/miic/uploads/a.png",
		"https://demo.miic.com.cn/miic/uploads/a": "/miic/uploads/a",
		"https://cdn.example.com/a.png":            "https://cdn.example.com/a.png",
	} {
		if actual := resolver.Resolve(input); actual != expected {
			t.Errorf("Resolve(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestCDNMode(t *testing.T) {
	resolver, err := New(Config{Mode: ModeCDN, BaseURL: "https://cdn.example.com/media", Aliases: map[string]string{"caamm/uploads": "/caam/uploads"}})
	if err != nil {
		t.Fatal(err)
	}
	if actual := resolver.Resolve("../../caamm/uploads/a.jpg"); actual != "https://cdn.example.com/media/caam/uploads/a.jpg" {
		t.Fatalf("unexpected CDN URL: %s", actual)
	}
}

func TestRewriteHTMLUsesSameResolver(t *testing.T) {
	resolver, _ := New(Config{Mode: ModeSameOrigin, Aliases: map[string]string{"mic/uploads": "/miic/uploads"}})
	actual := resolver.RewriteHTML(`<p><img src="../../../mic/uploads/a.png"><a href="../../mic/uploads/a.pdf">附件</a><a href="https://example.com">外链</a></p>`)
	for _, expected := range []string{`src="/miic/uploads/a.png"`, `href="/miic/uploads/a.pdf"`, `href="https://example.com"`} {
		if !strings.Contains(actual, expected) {
			t.Fatalf("missing %s in %s", expected, actual)
		}
	}
}
