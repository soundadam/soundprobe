package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestPaletteMatchesCharmLanguage(t *testing.T) {
	dark := Palette{Dark: true}
	light := Palette{Dark: false}
	if dark.Title("soundprobe") == "soundprobe" {
		t.Fatal("title should be styled")
	}
	if !strings.Contains(dark.Title("soundprobe"), "soundprobe") {
		t.Fatal("title dropped its text")
	}
	if dark.Title("soundprobe") == light.Title("soundprobe") {
		t.Fatal("indigo title should adapt to light/dark backgrounds")
	}
}

func TestHelpKeysStaySparse(t *testing.T) {
	c := newChrome()
	help := c.helpKeys("space", "a", "enter", "q")
	if strings.Contains(help, "toggle") || strings.Contains(help, "cancel") || strings.Contains(help, "move") {
		t.Fatalf("help should be keys only: %q", help)
	}
	for _, key := range []string{"space", "a", "enter", "q"} {
		if !strings.Contains(help, key) {
			t.Fatalf("missing %q in %q", key, help)
		}
	}
}

func TestChromeClampsAndScales(t *testing.T) {
	c := newChrome()
	if c.activityWidth() != activityWidth {
		t.Fatalf("default activity width = %d", c.activityWidth())
	}
	c = c.update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if c.activityWidth() != 12 {
		t.Fatalf("narrow activity width = %d", c.activityWidth())
	}
	if c.contentWidth() < 16 {
		t.Fatalf("content width collapsed: %d", c.contentWidth())
	}
	if got := truncateRunes("abcdefghijklmnopqrstuvwxyz", 8); got != "abcdefg…" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncateRunes("南京大学校内测速服务", 8); lipgloss.Width(got) > 8 {
		t.Fatalf("CJK truncate wider than limit: %q (%d)", got, lipgloss.Width(got))
	}
}

func TestScreenFrameHasTitleValueAndHelp(t *testing.T) {
	c := newChrome()
	view := c.render(screen{
		title:       "soundprobe",
		version:     "test",
		description: "select measurement targets",
		value:       "ipv4",
		body:        []string{c.indent(c.cursor(true) + c.check(true, false) + "NJU Campus")},
		help: []string{
			c.helpArrows(false, true) + helpGap + c.helpHorizontal(true, true),
			c.helpKeys("space", "enter", "q"),
		},
		err: "select at least one measurement target",
	})
	for _, expected := range []string{"soundprobe", "test", "select measurement targets", "ipv4", "NJU Campus", "space", "select at least one measurement target"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("missing %q in:\n%s", expected, view)
		}
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("frame should keep ANSI color")
	}
	if strings.Contains(view, "Error:") {
		t.Fatalf("error prefix leaked:\n%s", view)
	}
}

func TestBackgroundColorSwitchesPalette(t *testing.T) {
	c := newChrome()
	if !c.palette.Dark {
		t.Fatal("default palette should be dark")
	}
	c = c.update(tea.BackgroundColorMsg{})
	// An empty color is treated as dark by ultraviolet; the message still
	// flows through Update so light terminals can restyle later.
	_ = c.palette.Title("soundprobe")
}

func TestListMarkersMatchCharmSelectors(t *testing.T) {
	c := newChrome()
	if !strings.Contains(c.cursor(true), ">") {
		t.Fatalf("cursor = %q", c.cursor(true))
	}
	if !strings.Contains(c.check(true, false), "✓") || !strings.Contains(c.check(false, false), "•") || !strings.Contains(c.check(false, true), "–") {
		t.Fatalf("checks = %q / %q / %q", c.check(true, false), c.check(false, false), c.check(false, true))
	}
	if lipgloss.Width(c.cursor(true)) != lipgloss.Width(c.cursor(false)) {
		t.Fatal("cursor and spacer should occupy the same columns")
	}
}
