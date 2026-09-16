package ui

import (
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/preferences"
	"github.com/soundadam/soundprobe/internal/target"
)

func TestSelectorRecommendsCampusWhenReachable(t *testing.T) {
	selector := newSelectorModel("test", []target.ProbeResult{
		{StationID: "nju-campus", Family: "ipv4", Status: target.ProbeReachable},
		{StationID: "nju-edge", Family: "ipv4", Status: target.ProbeReachable},
	})
	appleExpected := runtime.GOOS == "darwin"
	if !selector.selected["nju-campus"] || !selector.selected["mlab"] || selector.selected["apple"] != appleExpected || selector.selected["ookla"] || selector.selected["nju-edge"] {
		t.Fatalf("selection = %#v", selector.selected)
	}
	view := selector.View().Content
	for _, expected := range []string{"soundprobe", "select measurement targets", "NJU Campus", "NJU Edge", "M-Lab", "ipv4", "space", "enter", "q"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Error:") || strings.Contains(view, "Space toggle") || strings.Contains(view, "[4] IPv4") {
		t.Fatalf("view still uses the pre-Charm chrome:\n%s", view)
	}
}

func TestSelectorDoesNotRecommendUnsupportedEdge(t *testing.T) {
	selector := newSelectorModel("test", []target.ProbeResult{
		{StationID: "nju-campus", Family: "ipv4", Status: target.ProbeUnreachable},
		{StationID: "nju-edge", Family: "ipv4", Status: target.ProbeReachable},
	})
	appleExpected := runtime.GOOS == "darwin"
	if selector.selected["nju-campus"] || selector.selected["nju-edge"] || !selector.selected["mlab"] || selector.selected["apple"] != appleExpected {
		t.Fatalf("selection = %#v", selector.selected)
	}
}

func TestConfiguredSelectorShowsOnlyDailyStationsInPriorityOrder(t *testing.T) {
	selector := newSelectorModelConfigured("test", []target.ProbeResult{
		{StationID: "tongji", Family: "ipv4", Status: target.ProbeReachable},
	}, preferences.Config{SchemaVersion: preferences.SchemaVersion, Language: preferences.LanguageChinese, DailyStations: []string{"tongji", "qlu"}})
	if len(selector.stations) != 2 || selector.stations[0].ID != "tongji" || selector.stations[1].ID != "qlu" {
		t.Fatalf("stations = %#v", selector.stations)
	}
	view := selector.View().Content
	if !strings.Contains(view, "选择测速站") || !strings.Contains(view, "同济大学 · 上海") || strings.Contains(view, "M-Lab") {
		t.Fatalf("configured view:\n%s", view)
	}
}

func TestSelectorBuildsDualStackPlan(t *testing.T) {
	selector := newSelectorModel("test", nil)
	selector.selected = map[string]bool{"nju-campus": true, "mlab": true}
	_, _ = selector.Update(key("d"))
	modelValue, command := selector.Update(key("enter"))
	if command == nil {
		t.Fatal("enter did not quit")
	}
	result := modelValue.(*selectorModel)
	want := []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderNJUCampusIPv6, model.ProviderMLab}
	if !result.done || len(result.plan.Providers) != len(want) {
		t.Fatalf("plan = %#v", result.plan)
	}
	for index := range want {
		if result.plan.Providers[index] != want[index] {
			t.Fatalf("provider[%d] = %q, want %q", index, result.plan.Providers[index], want[index])
		}
	}
	if result.View().Content != "" {
		t.Fatalf("completed selector was not cleared: %q", result.View().Content)
	}
}

func TestSelectorDeselectsIPv4OnlyStationForIPv6(t *testing.T) {
	selector := newSelectorModel("test", nil)
	selector.selected["qlu"] = true
	_, _ = selector.Update(key("6"))
	if selector.selected["qlu"] {
		t.Fatalf("IPv4-only station remained selected: %#v", selector.selected)
	}
}

func TestSelectorCyclesFamilyWithArrows(t *testing.T) {
	selector := newSelectorModel("test", nil)
	if selector.family != target.FamilyIPv4 {
		t.Fatalf("family = %s", selector.family)
	}
	_, _ = selector.Update(key("right"))
	if selector.family != target.FamilyIPv6 {
		t.Fatalf("right: family = %s", selector.family)
	}
	_, _ = selector.Update(key("left"))
	if selector.family != target.FamilyIPv4 {
		t.Fatalf("left: family = %s", selector.family)
	}
	_, _ = selector.Update(key("left"))
	if selector.family != target.FamilyDual {
		t.Fatalf("wrap left: family = %s", selector.family)
	}
}

func TestSelectorHomeEndAndEmptySelectionError(t *testing.T) {
	selector := newSelectorModel("test", nil)
	if len(selector.stations) < 2 {
		t.Fatal("need at least two stations")
	}
	_, _ = selector.Update(key("end"))
	if selector.cursor != len(selector.stations)-1 {
		t.Fatalf("end: cursor = %d", selector.cursor)
	}
	_, _ = selector.Update(key("home"))
	if selector.cursor != 0 {
		t.Fatalf("home: cursor = %d", selector.cursor)
	}
	selector.selected = map[string]bool{}
	_, _ = selector.Update(key("enter"))
	view := selector.View().Content
	if selector.done {
		t.Fatal("empty selection should not start")
	}
	if !strings.Contains(view, "select at least one measurement target") {
		t.Fatalf("missing validation sentence:\n%s", view)
	}
	if strings.Contains(view, "Error:") {
		t.Fatalf("error should be a short sentence, not a labeled dump:\n%s", view)
	}
}

func TestSelectorNarrowWidthKeepsKeysAndTruncates(t *testing.T) {
	selector := newSelectorModel("test", nil)
	_, _ = selector.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	view := selector.View().Content
	if !strings.Contains(view, "space") || !strings.Contains(view, "enter") {
		t.Fatalf("narrow view dropped keys:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 48 {
			t.Fatalf("narrow line too wide (%d): %q", lipgloss.Width(line), line)
		}
	}
}

func TestSelectorCancelClearsView(t *testing.T) {
	selector := newSelectorModel("test", nil)
	modelValue, command := selector.Update(key("esc"))
	result := modelValue.(*selectorModel)
	if command == nil || !result.cancelled {
		t.Fatal("esc should cancel")
	}
	if result.View().Content != "" {
		t.Fatalf("cancelled selector was not cleared: %q", result.View().Content)
	}
}

func key(value string) tea.KeyPressMsg {
	switch value {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "left":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft})
	case "right":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyRight})
	case "home":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyHome})
	case "end":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
	case "space":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeySpace, Text: " "})
	default:
		runeValue := rune(value[0])
		return tea.KeyPressMsg(tea.Key{Text: value, Code: runeValue})
	}
}
