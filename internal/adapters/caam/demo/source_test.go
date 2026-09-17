package demo

import (
	"context"
	"testing"

	"portal-static/internal/adapters/caam/config"
)

func TestSourceProvidesCompletePreviewData(t *testing.T) {
	t.Parallel()
	source := NewSource()
	cfg := config.Default()
	for _, slot := range cfg.ContentSlots() {
		items, err := source.FetchByColumn(context.Background(), slot)
		if err != nil {
			t.Fatalf("fetch %s: %v", slot.Key, err)
		}
		if len(items) == 0 {
			t.Fatalf("preview column %s is empty", slot.Key)
		}
		if len(items) > slot.Limit {
			t.Fatalf("preview column %s returned %d items, limit is %d", slot.Key, len(items), slot.Limit)
		}
	}

	stats, err := source.FetchMonthlyStatistics(context.Background(), "新能源汽车销量分析", "2026-06")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 18 {
		t.Fatalf("monthly statistics count = %d, want 18", len(stats))
	}

	titles, err := source.FetchStatisticsTitles(context.Background(), "首页统计数据")
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 6 || titles[0] != "新能源汽车销量分析" {
		t.Fatalf("unexpected statistics titles: %#v", titles)
	}
}
