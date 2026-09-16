package ui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/soundadam/soundprobe/internal/preferences"
)

func TestSetupChoosesLanguageAndDailyStations(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	view := stripANSI(setup.View().Content)
	if !strings.Contains(view, "soundprobe") || !strings.Contains(view, "Language / 语言") {
		t.Fatalf("language screen:\n%s", view)
	}
	_, _ = setup.Update(key("enter"))
	view = stripANSI(setup.View().Content)
	for _, expected := range []string{"南京大学校内测速服务", "公共互联网 NDT7", "同济大学 · 上海", "齐鲁工业大学 · 山东济南", "test.ustc.edu.cn", "选择日常测速站", "↑  ↓"} {
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

func TestSetupFollowsBackgroundColor(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	if !setup.dark {
		t.Fatal("default theme should be dark")
	}
	_, _ = setup.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}})
	if setup.dark {
		t.Fatal("white background should select the light theme")
	}
	_, _ = setup.Update(tea.BackgroundColorMsg{Color: color.RGBA{A: 255}})
	if !setup.dark {
		t.Fatal("black background should select the dark theme")
	}
}
