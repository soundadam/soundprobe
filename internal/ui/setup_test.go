package ui

import (
	"strings"
	"testing"

	"github.com/soundadam/soundprobe/internal/preferences"
)

func TestSetupChoosesLanguageAndDailyStations(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	if !strings.Contains(setup.View().Content, "soundprobe test") {
		t.Fatal("language screen does not show lowercase brand")
	}
	_, _ = setup.Update(key("enter"))
	view := setup.View().Content
	for _, expected := range []string{"南京大学校内测速服务", "公共互联网 NDT7", "同济大学 · 上海", "齐鲁工业大学 · 山东济南", "test.ustc.edu.cn"} {
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
