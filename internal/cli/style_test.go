package cli

import (
	"strings"
	"testing"
)

func TestStyleSetEmitsCharmColorsWhenEnabled(t *testing.T) {
	styles := newStyleSet(true)
	if got := styles.Title("soundprobe"); got == "soundprobe" || !strings.Contains(got, "\x1b[") {
		t.Fatalf("title = %q", got)
	}
	if got := styles.Accent("IPv4"); !strings.Contains(got, "\x1b[") {
		t.Fatalf("accent = %q", got)
	}
	if got := styles.Status("success"); got == "success" {
		t.Fatal("success status should be colored")
	}
	if got := styles.Status("failed"); got == "failed" {
		t.Fatal("failed status should be colored")
	}
}

func TestStyleSetStaysPlainWhenDisabled(t *testing.T) {
	styles := newStyleSet(false)
	if got := styles.Title("soundprobe"); got != "soundprobe" {
		t.Fatalf("disabled title = %q", got)
	}
	if got := styles.Status("success"); got != "success" {
		t.Fatalf("disabled status = %q", got)
	}
}
