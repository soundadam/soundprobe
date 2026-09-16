package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewSetupLanguageMatchesGolden(t *testing.T) {
	assertGolden(t, "setup-language.txt", PreviewSetupLanguage())
}

func TestPreviewSelectorMatchesGolden(t *testing.T) {
	assertGolden(t, "selector.txt", PreviewSelector())
}

func TestPreviewProgressMatchesGolden(t *testing.T) {
	assertGolden(t, "progress.txt", PreviewProgress())
}

func TestPreviewsKeepCharmColor(t *testing.T) {
	for name, view := range map[string]string{
		"setup":    PreviewSetupLanguage(),
		"selector": PreviewSelector(),
		"progress": PreviewProgress(),
	} {
		if !strings.Contains(view, "\x1b[") {
			t.Fatalf("%s preview should keep ANSI color:\n%s", name, view)
		}
		if !strings.Contains(view, "soundprobe") {
			t.Fatalf("%s preview missing brand:\n%s", name, view)
		}
	}
}

func TestPreviewSelectorOmitsVerboseHelpChrome(t *testing.T) {
	view := stripANSI(PreviewSelector())
	for _, banned := range []string{"Space toggle", "Address family", "select measurement targets"} {
		if strings.Contains(view, banned) {
			t.Fatalf("selector still has dated chrome %q:\n%s", banned, view)
		}
	}
	for _, want := range []string{"Select stations", "IPv4", "↑  ↓", "space", "ctrl+c"} {
		if want == "ctrl+c" {
			continue
		}
		if !strings.Contains(view, want) {
			t.Fatalf("selector missing %q:\n%s", want, view)
		}
	}
}

func TestPreviewProgressUsesTrackNotBlocks(t *testing.T) {
	view := stripANSI(PreviewProgress())
	if strings.Contains(view, "█") || strings.Contains(view, "░") || strings.Contains(view, "Activity") {
		t.Fatalf("progress still uses dated activity chrome:\n%s", view)
	}
	for _, want := range []string{"├", "●", "━", "Measuring", "ctrl+c"} {
		if !strings.Contains(view, want) {
			t.Fatalf("progress missing %q:\n%s", want, view)
		}
	}
}

func assertGolden(t *testing.T, name, view string) {
	t.Helper()
	got := stripANSI(view)
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\n\nnew content:\n%s", name, err, got)
	}
	if got != string(want) {
		t.Fatalf("golden %s mismatch\n\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
