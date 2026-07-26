package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
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
	_, _ = progress.Update(tickMessage(progress.now))

	view := progress.View().Content
	for _, expected := range []string{
		"NJUProbe test",
		"Network   en0 · wifi · NJU-WLAN",
		"Order     NJU Campus · IPv4 → M-Lab · sequential",
		"NJU Campus · IPv4    ✓ complete · 00:00",
		"Activity  [████████████████████████]",
		"Rate      ↓ 876.54 Mbps · ↑ 345.67 Mbps",
		"Detail    server speed.nju.edu.cn",
		"M-Lab                ◐ downloading · 00:00",
		"Rate      ↓ 80.00 Mbps · ↑ —",
		"Detail    server ndt.example.net",
		"Elapsed   00:12",
		"Ctrl-C    cancel",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
	if len(strings.Split(view, "\n")) != 13 {
		t.Fatalf("view lines = %d, want 13", len(strings.Split(view, "\n")))
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
	_, _ = progress.Update(tickMessage(firstTick))

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
	_, _ = progress.Update(tickMessage(secondTick))

	view := progress.View().Content
	for _, expected := range []string{
		"NJU Campus · IPv4    × failed",
		"error: server unreachable",
		"M-Lab                ◐ uploading",
		"Rate      ↓ 33.67 Mbps · ↑ 4.75 Mbps",
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
	_, _ = progress.Update(tickMessage(now))

	view := progress.View().Content
	for _, expected := range []string{
		"M-Lab                × failed · upload",
		"Activity  [────────────────────────]",
		"Rate      ↓ — · ↑ —",
		"Detail    error: dial tcp: network is unreachable",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("view missing %q:\n%s", expected, view)
		}
	}
}

func TestActivityBarAnimatesWithoutPretendingPercentage(t *testing.T) {
	first := renderActivity(provider.ProgressMeasuring, time.Unix(0, 0))
	second := renderActivity(provider.ProgressMeasuring, time.Unix(0, int64(refreshInterval)))
	if first == second {
		t.Fatalf("activity bar did not animate: %q", first)
	}
	if strings.Contains(first, "%") || strings.Contains(second, "%") {
		t.Fatalf("activity bar presents a false percentage: %q / %q", first, second)
	}
	if got := renderActivity(provider.ProgressComplete, time.Time{}); strings.Count(got, "█") != activityWidth {
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
