package generator

import (
	"strings"
	"testing"
)

func TestArticleContentPreservesSafeAlignment(t *testing.T) {
	g := &Generator{}
	content := string(g.content(`<p style="text-align: center; color: red">居中标题</p><section style="text-align:center"><img src="/uploads/photo.jpg"></section><div align="right">右对齐</div><p style="text-align: expression(alert(1))">不安全样式</p>`, "../../"))
	for _, want := range []string{
		`data-text-align="center"`,
		`data-text-align="right"`,
		`居中标题`,
		`<section data-text-align="center"><img`,
		`右对齐`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("sanitized content missing %q: %s", want, content)
		}
	}
	for _, forbidden := range []string{` style=`, ` align=`, `expression(`} {
		if strings.Contains(content, forbidden) {
			t.Errorf("unsafe editor markup retained %q: %s", forbidden, content)
		}
	}
}
