package generator

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapStaticPageErrorUsesFriendlyChinesePageName(t *testing.T) {
	cause := errors.New("栏目名称必须且只能匹配一个栏目：“首页头条”，匹配到 2 条记录")
	err := wrapStaticPageError("home", cause)
	if !errors.Is(err, cause) {
		t.Fatalf("wrapped error does not preserve cause: %v", err)
	}
	if want := "生成首页失败：栏目名称必须且只能匹配一个栏目"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func TestStaticPageDisplayName(t *testing.T) {
	tests := map[string]string{
		"home": "首页", "about": "协会概况页", "work": "协会工作页",
		"stats": "统计数据页", "members": "会员专区页", "party": "党建专区页",
	}
	for name, want := range tests {
		if got := staticPageDisplayName(name); got != want {
			t.Errorf("staticPageDisplayName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestNormalizeStaticPageNameAcceptsChineseAndLegacyEnglishNames(t *testing.T) {
	tests := map[string]string{
		"首页": "home", "协会概况": "about", "协会工作": "work",
		"统计数据": "stats", "会员专区": "members", "党建专区": "party",
		"home": "home", "about": "about", "work": "work",
		"stats": "stats", "members": "members", "party": "party",
	}
	for input, want := range tests {
		got, ok := NormalizeStaticPageName(input)
		if !ok || got != want {
			t.Errorf("NormalizeStaticPageName(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
	}
	if _, ok := NormalizeStaticPageName("不存在页面"); ok {
		t.Fatal("expected unknown Chinese page name to be rejected")
	}
}
