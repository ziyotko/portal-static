package generator

import (
	"strings"
	"testing"
)

func TestArticleContentPreservesSafeAlignment(t *testing.T) {
	g := &Generator{}
	content := string(g.content(`<h1><span style="font-size:17px;font-weight:normal;line-height:1.6">正文标题</span></h1><p style="text-align: center; color: red">居中标题</p><section style="text-align:center"><img src="/uploads/photo.jpg"></section><div align="right">右对齐</div><p style="text-align: expression(alert(1))">不安全样式</p>`, "../../"))
	for _, want := range []string{
		`data-text-align="center"`,
		`data-text-align="right"`,
		`居中标题`,
		`font-size: 17px`,
		`font-weight: normal`,
		`<section style="text-align: center" data-text-align="center"><img`,
		`右对齐`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("sanitized content missing %q: %s", want, content)
		}
	}
	for _, forbidden := range []string{` align=`, `expression(`} {
		if strings.Contains(content, forbidden) {
			t.Errorf("unsafe editor markup retained %q: %s", forbidden, content)
		}
	}
}

func TestArticleContentImageLink(t *testing.T) {
	g := &Generator{}
	content := string(g.content(`<p><img src="/uploads/photo.jpg" data-href="https://example.com/article"></p><p><img src="/uploads/local.jpg" data-href="/list/2/1.html"></p><p><img src="/uploads/plain.jpg"></p><p><img src="/uploads/unsafe.jpg" data-href="javascript:alert(1)"></p>`, "../../"))
	for _, want := range []string{
		`<a href="https://example.com/article" rel="nofollow"><img src="/uploads/photo.jpg"`,
		`<a href="/list/2/1.html" rel="nofollow"><img src="/uploads/local.jpg"`,
		`<p><img src="/uploads/plain.jpg"`,
		`<p><img src="/uploads/unsafe.jpg"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("generated article missing %q: %s", want, content)
		}
	}
	if strings.Contains(content, `javascript:`) || strings.Contains(content, `data-href=`) {
		t.Errorf("unsafe or editor-only link retained: %s", content)
	}
}
