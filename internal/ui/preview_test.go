package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/preferences"
	"github.com/soundadam/soundprobe/internal/provider"
	"github.com/soundadam/soundprobe/internal/target"
)

func TestPreviewSelectorLooksLikeTheClient(t *testing.T) {
	selector := newSelectorModel("test", []target.ProbeResult{
		{StationID: "nju-campus", Family: "ipv4", Status: target.ProbeReachable},
	})
	view := selector.View().Content
	for _, want := range []string{
		"soundprobe",
		"select measurement targets",
		"ipv4",
		"NJU Campus",
		"space",
		"enter",
		"q",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("selector preview should keep ANSI color")
	}
	if strings.Contains(view, "filter") || strings.Contains(view, "[x]") || strings.Contains(view, "Space toggle") {
		t.Fatalf("selector still exposes the old chrome:\n%s", view)
	}
}

func TestPreviewSetupLooksLikeTheClient(t *testing.T) {
	setup := newSetupModel("test", preferences.DefaultConfig())
	view := setup.View().Content
	for _, want := range []string{"soundprobe", "中文", "English", "enter", "q"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("setup preview should keep ANSI color")
	}
}

func TestPreviewProgressLooksLikeTheClient(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}, ready)
	progress.now = progress.startedAt.Add(3 * time.Second)
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider:     model.ProviderNJUCampusIPv4,
		Phase:        provider.ProgressComplete,
		Server:       "speed.nju.edu.cn",
		DownloadMbps: model.Pointer(100.0),
		UploadMbps:   model.Pointer(20.0),
	}))
	view := progress.View().Content
	for _, want := range []string{"soundprobe", target.Label(model.ProviderNJUCampusIPv4), "complete", "↓ 100.00 Mbps", "ctrl+c"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("progress preview should keep ANSI color")
	}
}
