package ui

import (
	"strings"
	"testing"
)

func TestThemeUsesCharmPalette(t *testing.T) {
	dark := NewTheme(true)
	light := NewTheme(false)
	if dark.Title.Render("soundprobe") == "soundprobe" {
		t.Fatal("dark title should apply color")
	}
	if !strings.Contains(dark.Accent.Render("x"), "\x1b[") {
		t.Fatal("accent should emit ANSI")
	}
	if dark.Title.Render("soundprobe") == light.Title.Render("soundprobe") {
		t.Fatal("light and dark indigo should differ")
	}
	if dark.Value.Render("IPv4") != light.Value.Render("IPv4") {
		t.Fatal("fuchsia accent should be the same on light and dark backgrounds")
	}
}

func TestRenderFrameIndentsBodyAndHelp(t *testing.T) {
	theme := NewTheme(true)
	view := stripANSI(renderFrame(theme, []string{"Select stations", "IPv4"}, []string{theme.help("enter", "q")}))
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) < 5 || lines[0] != "soundprobe" || lines[1] != "" {
		t.Fatalf("frame header:\n%s", view)
	}
	if lines[2] != "  Select stations" || lines[3] != "  IPv4" {
		t.Fatalf("body indent:\n%s", view)
	}
	if !strings.HasPrefix(lines[len(lines)-1], "  ") || !strings.Contains(lines[len(lines)-1], "enter") {
		t.Fatalf("help:\n%s", view)
	}
}
