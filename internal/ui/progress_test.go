package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
)

func TestProgressModelRendersFixedProviderState(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", model.CommandRun, ready)
	progress.startedAt = time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	progress.now = progress.startedAt.Add(12 * time.Second)
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Network: &model.NetworkContext{
			ActiveInterface: model.Pointer("en0"),
			InterfaceKind:   model.Pointer("wifi"),
			SSID:            model.Pointer("NJU-WLAN"),
		},
	}))
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider:     model.ProviderCampus,
		Phase:        provider.ProgressComplete,
		DownloadMbps: model.Pointer(876.54),
		UploadMbps:   model.Pointer(345.67),
	}))
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressDownloading,
		Test:     "download",
		LiveMbps: model.Pointer(80.0),
	}))
	_, _ = progress.Update(tickMessage(progress.now))

	view := progress.View().Content
	for _, expected := range []string{
		"NJUProbe test",
		"Network   en0 · wifi · NJU-WLAN",
		"Campus    complete · ↓ 876.54 Mbps · ↑ 345.67 Mbps",
		"M-Lab     downloading · download 80.00 Mbps",
		"Elapsed   00:12",
		"Ctrl-C    cancel",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
	if len(strings.Split(view, "\n")) != 7 {
		t.Fatalf("view lines = %d, want 7", len(strings.Split(view, "\n")))
	}
}

func TestProgressRendererStartsUpdatesAndClears(t *testing.T) {
	var output bytes.Buffer
	renderer, err := NewProgressRenderer(&output, "test", model.CommandCampus)
	if err != nil {
		t.Fatal(err)
	}
	renderer.Update(provider.ProgressEvent{
		Provider: model.ProviderCampus,
		Phase:    provider.ProgressMeasuring,
	})
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
}
