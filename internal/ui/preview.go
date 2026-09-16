package ui

import (
	"regexp"
	"strings"
	"time"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/preferences"
	"github.com/soundadam/soundprobe/internal/provider"
	"github.com/soundadam/soundprobe/internal/target"
)

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(value string) string {
	return ansiSequence.ReplaceAllString(value, "")
}

func compactPreview(view string) string {
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

func PreviewSetupLanguage() string {
	setup := newSetupModel("test", preferences.DefaultConfig())
	return compactPreview(setup.View().Content)
}

func PreviewSelector() string {
	latency := 12.0
	selector := newSelectorModelConfigured("test", []target.ProbeResult{
		{StationID: "nju-campus", Family: "ipv4", Status: target.ProbeReachable, LatencyMS: &latency},
		{StationID: "tongji", Family: "ipv4", Status: target.ProbeReachable},
	}, preferences.Config{
		SchemaVersion: preferences.SchemaVersion,
		Language:      preferences.LanguageEnglish,
		DailyStations: []string{"nju-campus", "mlab", "tongji"},
	})
	return compactPreview(selector.View().Content)
}

func PreviewProgress() string {
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
	return compactPreview(progress.View().Content)
}
