package mlab

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
)

func TestAccumulatorBuildsSuccessfulMeasurement(t *testing.T) {
	file, err := os.Open("testdata/ndt7-success.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var progress []provider.ProgressEvent
	request := provider.Request{Progress: func(event provider.ProgressEvent) {
		progress = append(progress, event)
	}}
	var parsed accumulator
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := parsed.consume(scanner.Bytes(), request); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	measurement, err := parsed.measurement(HelperVersion, 25_000)
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Status != model.ProviderStatusSuccess {
		t.Fatalf("status = %q", measurement.Status)
	}
	if measurement.DownloadMbps == nil || *measurement.DownloadMbps != 80 {
		t.Fatalf("download = %v", measurement.DownloadMbps)
	}
	if measurement.UploadMbps == nil || *measurement.UploadMbps != 40 {
		t.Fatalf("upload = %v", measurement.UploadMbps)
	}
	if measurement.DownloadBytes == nil || *measurement.DownloadBytes != 100000000 {
		t.Fatalf("download bytes = %v", measurement.DownloadBytes)
	}
	if measurement.UploadBytes == nil || *measurement.UploadBytes != 50000000 {
		t.Fatalf("upload bytes = %v", measurement.UploadBytes)
	}
	if measurement.ServerFQDN == nil || *measurement.ServerFQDN != "ndt-mlab1.example.net" {
		t.Fatalf("server FQDN = %v", measurement.ServerFQDN)
	}
	if measurement.ClientPublicIP == nil || *measurement.ClientPublicIP != "198.51.100.42" {
		t.Fatalf("client IP = %v", measurement.ClientPublicIP)
	}
	if measurement.IPFamily == nil || *measurement.IPFamily != "ipv4" {
		t.Fatalf("IP family = %v", measurement.IPFamily)
	}
	if len(progress) < 4 {
		t.Fatalf("progress events = %d", len(progress))
	}
	foundLive := false
	for _, event := range progress {
		if event.LiveMbps != nil && *event.LiveMbps > 0 {
			foundLive = true
		}
	}
	if !foundLive {
		t.Fatal("no live-rate progress event was emitted")
	}
}

func TestAccumulatorPreservesPartialFailure(t *testing.T) {
	data, err := os.ReadFile("testdata/ndt7-upload-failure.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var parsed accumulator
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if err := parsed.consume([]byte(line), provider.Request{}); err != nil {
			t.Fatal(err)
		}
	}
	measurement, err := parsed.measurement(HelperVersion, 30_000)
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Status != model.ProviderStatusFailed {
		t.Fatalf("status = %q", measurement.Status)
	}
	if measurement.DownloadMbps == nil || *measurement.DownloadMbps != 80 {
		t.Fatalf("download = %v", measurement.DownloadMbps)
	}
	if measurement.UploadMbps == nil || *measurement.UploadMbps != 0 {
		t.Fatalf("upload = %v", measurement.UploadMbps)
	}
	if measurement.Failure == nil || measurement.Failure.Stage != model.FailureStageUpload {
		t.Fatalf("failure = %#v", measurement.Failure)
	}
}

func TestAccumulatorRejectsMalformedOutput(t *testing.T) {
	var parsed accumulator
	if err := parsed.consume([]byte(`not-json`), provider.Request{}); err == nil {
		t.Fatal("consume() succeeded for malformed JSON")
	}
	if _, err := parsed.measurement(HelperVersion, 1); err == nil {
		t.Fatal("measurement() succeeded without a summary or error")
	}
}

func TestLiveMeasurement(t *testing.T) {
	rate, bytes, ok := liveMeasurement(eventValue{
		Test:    "download",
		AppInfo: &appInfo{ElapsedTime: 10_000_000, NumBytes: 100_000_000},
	})
	if !ok || rate != 80 || bytes != 100_000_000 {
		t.Fatalf("rate=%v bytes=%d ok=%v", rate, bytes, ok)
	}
}
