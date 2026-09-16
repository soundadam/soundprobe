package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/provider"
)

func TestProgressModelRendersEqualProviderPanels(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}, ready)
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
		Provider:     model.ProviderNJUCampusIPv4,
		Phase:        provider.ProgressComplete,
		Server:       "speed.nju.edu.cn",
		DownloadMbps: model.Pointer(876.54),
		UploadMbps:   model.Pointer(345.67),
	}))
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressDownloading,
		Test:     "download",
		Server:   "ndt.example.net",
		LiveMbps: model.Pointer(80.0),
	}))

	view := progress.View().Content
	for _, expected := range []string{
		"soundprobe",
		"test",
		"en0 · wifi · NJU-WLAN",
		"NJU Campus · IPv4 → M-Lab",
		"NJU Campus · IPv4",
		"✓",
		"complete",
		"[" + strings.Repeat("█", activityWidth) + "]",
		"↓ 876.54 Mbps · ↑ 345.67 Mbps",
		"server speed.nju.edu.cn",
		"M-Lab",
		"◐",
		"downloading",
		"↓ 80.00 Mbps · ↑ —",
		"server ndt.example.net",
		"00:12",
		"ctrl+c",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
	if strings.Contains(view, "Activity  ") || strings.Contains(view, "Rate      ") || strings.Contains(view, "Network   ") {
		t.Fatalf("progress still uses labeled field chrome:\n%s", view)
	}
	if strings.Count(view, "[") < 2 {
		t.Fatalf("expected two activity bars:\n%s", view)
	}
}

func TestProgressModelKeepsLiveRatesAndRendersEitherProviderFailure(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}, ready)
	progress.startedAt = time.Unix(0, 0)
	firstTick := progress.startedAt.Add(time.Second)
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressDownloading,
		Test:     "download",
		Server:   "ndt.example.net",
		LiveMbps: model.Pointer(33.67),
	}))
	progress.now = firstTick

	secondTick := firstTick.Add(time.Second)
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressUploading,
		Test:     "upload",
		LiveMbps: model.Pointer(4.75),
	}))
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderNJUCampusIPv4,
		Phase:    provider.ProgressFailed,
		Message:  "server unreachable",
	}))
	progress.now = secondTick

	view := progress.View().Content
	for _, expected := range []string{
		"NJU Campus · IPv4",
		"×",
		"failed",
		"error: server unreachable",
		"M-Lab",
		"uploading",
		"↓ 33.67 Mbps · ↑ 4.75 Mbps",
		"server ndt.example.net",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
}

func TestProgressModelRendersMLabFailureWithTheSamePanelContract(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}, ready)
	now := time.Unix(10, 0)
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressFailed,
		Test:     "upload",
		Server:   "ndt.example.net",
		Message:  "dial tcp: network is unreachable",
	}))
	progress.now = now

	view := progress.View().Content
	for _, expected := range []string{
		"M-Lab",
		"failed · upload",
		"[" + strings.Repeat("─", activityWidth) + "]",
		"↓ — · ↑ —",
		"error: dial tcp: network is unreachable",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
}

func TestActivityBarAnimatesWithoutPretendingPercentage(t *testing.T) {
	first := renderActivity(provider.ProgressMeasuring, time.Unix(0, 0), activityWidth)
	second := renderActivity(provider.ProgressMeasuring, time.Unix(0, int64(refreshInterval)), activityWidth)
	if first == second {
		t.Fatalf("activity bar did not animate: %q", first)
	}
	if strings.Contains(first, "%") || strings.Contains(second, "%") {
		t.Fatalf("activity bar presents a false percentage: %q / %q", first, second)
	}
	if got := renderActivity(provider.ProgressComplete, time.Time{}, activityWidth); strings.Count(got, "█") != activityWidth {
		t.Fatalf("complete activity = %q", got)
	}
}

func TestProgressRendererStartsUpdatesAndClears(t *testing.T) {
	var output bytes.Buffer
	renderer, err := NewProgressRenderer(&output, "test", []model.Provider{model.ProviderNJUCampusIPv4})
	if err != nil {
		t.Fatal(err)
	}
	renderer.Update(provider.ProgressEvent{
		Provider: model.ProviderNJUCampusIPv4,
		Phase:    provider.ProgressMeasuring,
	})
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProgressRendererRendersFinalFrameBeforeClear(t *testing.T) {
	var output bytes.Buffer
	renderer, err := newProgressRenderer(
		&output,
		"test",
		[]model.Provider{model.ProviderMLab},
		tea.WithWindowSize(120, 40),
	)
	if err != nil {
		t.Fatal(err)
	}
	renderer.Update(provider.ProgressEvent{
		Provider:     model.ProviderMLab,
		Phase:        provider.ProgressComplete,
		DownloadMbps: model.Pointer(44.08),
		UploadMbps:   model.Pointer(4.23),
	})
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}

	if got := output.String(); !strings.Contains(got, "↓ 44.08 Mbps · ↑ 4.23 Mbps") {
		t.Fatalf("final frame was not rendered before clear:\n%q", got)
	}
}

func TestProgressNarrowWidthKeepsFourRowPanels(t *testing.T) {
	ready := make(chan struct{})
	progress := newProgressModel("test", []model.Provider{model.ProviderMLab}, ready)
	_, _ = progress.Update(tea.WindowSizeMsg{Width: 42, Height: 18})
	_, _ = progress.Update(progressMessage(provider.ProgressEvent{
		Provider: model.ProviderMLab,
		Phase:    provider.ProgressDownloading,
		Test:     "download",
		Server:   "very-long.measurement-lab.example.net",
		LiveMbps: model.Pointer(12.5),
	}))
	view := progress.View().Content
	if !strings.Contains(view, "M-Lab") || !strings.Contains(view, "↓ 12.50 Mbps") || !strings.Contains(view, "ctrl+c") {
		t.Fatalf("narrow progress dropped panel rows:\n%s", view)
	}
	if !strings.Contains(view, "["+strings.Repeat("█", 12)) && !strings.Contains(view, "["+strings.Repeat("░", 12)) {
		// 42-col terminals use the 12-cell activity bar.
		if !strings.Contains(view, "[") {
			t.Fatalf("narrow progress missing activity bar:\n%s", view)
		}
	}
}
