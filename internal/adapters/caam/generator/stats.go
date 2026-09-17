package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/model"
)

var reportPeriodPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type StatView struct {
	Title       string
	Label       string
	Unit        string
	LegendA     string
	LegendB     string
	ValueA      string
	ValueB      string
	BarsA       string
	BarsB       string
	ReportMonth int
	Bars        []BarView
}

type BarView struct {
	Label  string
	Active bool
	Style  template.CSS
}

func parseStat(article model.Article) (StatView, error) {
	if !reportPeriodPattern.MatchString(strings.TrimSpace(article.Summary)) {
		return StatView{}, fmt.Errorf("article %s summary must use YYYY-MM", article.ID)
	}
	var payload model.StatsPayload
	decoder := json.NewDecoder(strings.NewReader(article.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return StatView{}, fmt.Errorf("article %s has invalid stats JSON: %w", article.ID, err)
	}
	if strings.TrimSpace(payload.Label) == "" || strings.TrimSpace(payload.Unit) == "" {
		return StatView{}, fmt.Errorf("article %s stats label and unit are required", article.ID)
	}
	if len(payload.Series) != 2 {
		return StatView{}, fmt.Errorf("article %s stats requires exactly two series", article.ID)
	}
	if len(payload.Series[0].Months) < 1 || len(payload.Series[0].Months) > 12 || len(payload.Series[0].Months) != len(payload.Series[1].Months) {
		return StatView{}, fmt.Errorf("article %s stats month arrays must have the same length between 1 and 12", article.ID)
	}
	if strings.TrimSpace(payload.Series[0].Name) == "" || strings.TrimSpace(payload.Series[1].Name) == "" {
		return StatView{}, fmt.Errorf("article %s stats series names are required", article.ID)
	}

	maxValue := 0.0
	for _, series := range payload.Series {
		if math.IsNaN(series.Value) || math.IsInf(series.Value, 0) || series.Value < 0 {
			return StatView{}, fmt.Errorf("article %s has invalid series value", article.ID)
		}
		for _, value := range series.Months {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
				return StatView{}, fmt.Errorf("article %s has invalid monthly value", article.ID)
			}
			if value > maxValue {
				maxValue = value
			}
		}
	}
	if maxValue == 0 {
		maxValue = 1
	}

	percentA := make([]string, 12)
	percentB := make([]string, 12)
	monthLabels := []string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"}
	reportMonth, _ := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(article.Summary)[4:], "-"))
	bars := make([]BarView, 0, 12)
	for i := 0; i < 12; i++ {
		a, b := 0.0, 0.0
		if i < len(payload.Series[0].Months) {
			a = payload.Series[0].Months[i] / maxValue * 100
			b = payload.Series[1].Months[i] / maxValue * 100
		}
		percentA[i] = formatNumber(a)
		percentB[i] = formatNumber(b)
		bars = append(bars, BarView{
			Label:  monthLabels[i],
			Active: i+1 == reportMonth,
			Style:  template.CSS(fmt.Sprintf("--a:%s%%;--b:%s%%", percentA[i], percentB[i])),
		})
	}

	return StatView{
		Title:       strings.TrimSpace(article.Summary) + " " + strings.TrimSpace(article.Title),
		Label:       strings.TrimSpace(payload.Label),
		Unit:        strings.TrimSpace(payload.Unit),
		LegendA:     strings.TrimSpace(payload.Series[0].Name),
		LegendB:     strings.TrimSpace(payload.Series[1].Name),
		ValueA:      formatNumber(payload.Series[0].Value),
		ValueB:      formatNumber(payload.Series[1].Value),
		BarsA:       strings.Join(percentA, ","),
		BarsB:       strings.Join(percentB, ","),
		ReportMonth: reportMonth,
		Bars:        bars,
	}, nil
}

func parseStats(articles []model.Article, unit string) ([]StatView, error) {
	if monthly, err := parseMonthlyStat(articles, unit); err == nil {
		return []StatView{monthly}, nil
	}

	stats := make([]StatView, 0, len(articles))
	var parseErrors []error
	for _, article := range articles {
		stat, err := parseStat(article)
		if err != nil {
			parseErrors = append(parseErrors, err)
			continue
		}
		stats = append(stats, stat)
	}
	if len(stats) == 0 {
		if len(parseErrors) > 0 {
			return nil, fmt.Errorf("statistics column has no valid records: %w", errors.Join(parseErrors...))
		}
		return nil, errors.New("statistics column has no records")
	}
	return stats, nil
}

func parseMonthlyStat(articles []model.Article, unit string) (StatView, error) {
	if len(articles) == 0 {
		return StatView{}, errors.New("statistics column has no records")
	}

	valuesByYear := make(map[int][]float64)
	latestYear := 0
	latestMonth := 0
	latestSummary := ""
	latestTitle := ""
	valid := 0
	for _, article := range articles {
		period, err := time.Parse("2006-01", strings.TrimSpace(article.Summary))
		if err != nil {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(article.Content), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			continue
		}
		year, month := period.Year(), int(period.Month())
		if valuesByYear[year] == nil {
			valuesByYear[year] = make([]float64, 12)
		}
		valuesByYear[year][month-1] = value
		valid++
		if year > latestYear || (year == latestYear && month > latestMonth) {
			latestYear = year
			latestMonth = month
			latestSummary = strings.TrimSpace(article.Summary)
			latestTitle = strings.TrimSpace(article.Title)
		}
	}
	if valid == 0 {
		return StatView{}, errors.New("statistics column has no valid monthly numeric records")
	}

	previousYear := latestYear - 1
	previous := valuesByYear[previousYear]
	current := valuesByYear[latestYear]
	if previous == nil {
		previous = make([]float64, 12)
	}
	if current == nil {
		current = make([]float64, 12)
	}

	maxValue := 0.0
	valuePrevious := 0.0
	valueCurrent := 0.0
	for i := 0; i < 12; i++ {
		valuePrevious += previous[i]
		valueCurrent += current[i]
		if previous[i] > maxValue {
			maxValue = previous[i]
		}
		if current[i] > maxValue {
			maxValue = current[i]
		}
	}
	if maxValue == 0 {
		maxValue = 1
	}

	monthLabels := []string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"}
	percentPrevious := make([]string, 12)
	percentCurrent := make([]string, 12)
	bars := make([]BarView, 0, 12)
	for i := 0; i < 12; i++ {
		percentPrevious[i] = formatNumber(previous[i] / maxValue * 100)
		percentCurrent[i] = formatNumber(current[i] / maxValue * 100)
		bars = append(bars, BarView{
			Label:  monthLabels[i],
			Active: i+1 == latestMonth,
			Style:  template.CSS(fmt.Sprintf("--a:%s%%;--b:%s%%", percentPrevious[i], percentCurrent[i])),
		})
	}
	unit = strings.TrimSpace(unit)
	if unit == "" {
		unit = "万辆"
	}
	return StatView{
		Title:       strings.TrimSpace(latestSummary + " " + latestTitle),
		Label:       latestTitle,
		Unit:        unit,
		LegendA:     fmt.Sprintf("%d年", previousYear),
		LegendB:     fmt.Sprintf("%d年", latestYear),
		ValueA:      formatNumber(valuePrevious),
		ValueB:      formatNumber(valueCurrent),
		BarsA:       strings.Join(percentPrevious, ","),
		BarsB:       strings.Join(percentCurrent, ","),
		ReportMonth: latestMonth,
		Bars:        bars,
	}, nil
}

func emptyStatView() StatView {
	labels := []string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"}
	bars := make([]BarView, 0, len(labels))
	for _, label := range labels {
		bars = append(bars, BarView{Label: label, Style: template.CSS("--a:0%;--b:0%")})
	}
	return StatView{
		Title:   "暂无统计数据",
		Label:   "暂无统计数据",
		Unit:    "-",
		LegendA: "本期",
		LegendB: "同期",
		ValueA:  "0.0",
		ValueB:  "0.0",
		BarsA:   "0,0,0,0,0,0,0,0,0,0,0,0",
		BarsB:   "0,0,0,0,0,0,0,0,0,0,0,0",
		Bars:    bars,
	}
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}
