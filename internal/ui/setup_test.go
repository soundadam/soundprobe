package ui

import (
	"strings"
	"testing"

	"github.com/soundadam/soundprobe/internal/preferences"
)

func TestSetupChoosesLanguageAndDailyStations(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	view := setup.View().Content
	if !strings.Contains(view, "soundprobe") || !strings.Contains(view, "test") {
		t.Fatal("language screen does not show lowercase brand")
	}
	if !strings.Contains(view, "Choose interface language") || !strings.Contains(view, "中文") || !strings.Contains(view, "English") {
		t.Fatalf("language screen missing choices:\n%s", view)
	}
	if strings.Contains(view, "welcome / 欢迎") {
		t.Fatalf("language screen still uses stacked welcome chrome:\n%s", view)
	}
	_, _ = setup.Update(key("enter"))
	view = setup.View().Content
	for _, expected := range []string{"南京大学校内测速服务", "公共互联网 NDT7", "同济大学 · 上海", "齐鲁工业大学 · 山东济南", "test.ustc.edu.cn", "space", "enter"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("setup view missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "CERNET") {
		t.Fatalf("setup view includes unavailable CERNET station:\n%s", view)
	}
	setup.selected = map[string]bool{"tongji": true}
	modelValue, command := setup.Update(key("enter"))
	result := modelValue.(*setupModel)
	if command == nil || !result.done || result.config().DailyStations[0] != "tongji" {
		t.Fatalf("result = %#v", result.config())
	}
}

func TestSetupEmptySelectionShowsShortError(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	_, _ = setup.Update(key("enter"))
	setup.selected = map[string]bool{}
	_, _ = setup.Update(key("enter"))
	view := setup.View().Content
	if !strings.Contains(view, "请至少选择一个日常测速站") {
		t.Fatalf("missing validation sentence:\n%s", view)
	}
	if strings.Contains(view, "Error:") {
		t.Fatalf("error should be a short sentence:\n%s", view)
	}
}

func TestSetupHomeEndMovesStationCursor(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	_, _ = setup.Update(key("enter"))
	_, _ = setup.Update(key("end"))
	if setup.cursor != len(setup.stations)-1 {
		t.Fatalf("end: cursor = %d", setup.cursor)
	}
	_, _ = setup.Update(key("home"))
	if setup.cursor != 0 {
		t.Fatalf("home: cursor = %d", setup.cursor)
	}
}
