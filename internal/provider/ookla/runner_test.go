package ookla

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/provider"
)

func TestRunnerAcceptsOfficialCLIAndDoesNotAutoAcceptTerms(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "speedtest")
	argsPath := filepath.Join(root, "args")
	fixturePath := filepath.Join(root, "success.json")
	fixture, err := os.ReadFile(filepath.Join("testdata", "success.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixturePath, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"if [ \"${1:-}\" = \"--version\" ]; then echo 'Speedtest by Ookla 1.2.0.84'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" > \"$SOUNDPROBE_ARGS\"\n" +
		"cat \"$SOUNDPROBE_FIXTURE\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOUNDPROBE_ARGS", argsPath)
	t.Setenv("SOUNDPROBE_FIXTURE", fixturePath)
	runner := &Runner{Path: path}
	if err := runner.Preflight(context.Background(), provider.Request{}); err != nil {
		t.Fatal(err)
	}
	interfaceName := "utun6"
	measurement, err := runner.Measure(context.Background(), provider.Request{Network: &model.NetworkContext{ActiveInterface: &interfaceName}})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Status != model.ProviderStatusSuccess {
		t.Fatalf("status = %q", measurement.Status)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(args)
	if !strings.Contains(text, "--format=json") || !strings.Contains(text, "--interface=utun6") {
		t.Fatalf("args = %q", text)
	}
	if strings.Contains(text, "accept-license") || strings.Contains(text, "accept-gdpr") {
		t.Fatalf("runner unexpectedly accepted terms: %q", text)
	}
}

func TestRunnerRejectsPythonSpeedtestCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "speedtest")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nif [ \"${1:-}\" = \"--version\" ]; then echo 'speedtest-cli 2.1.3'; fi\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Path: path}
	if err := runner.Preflight(context.Background(), provider.Request{}); err == nil || !strings.Contains(err.Error(), "official Ookla") {
		t.Fatalf("preflight error = %v", err)
	}
}

func TestRunnerReportsMissingOptionalHelper(t *testing.T) {
	runner := &Runner{LookupPath: func(string) (string, error) {
		return "", os.ErrNotExist
	}}
	if err := runner.Preflight(context.Background(), provider.Request{}); err == nil || !strings.Contains(err.Error(), "official Ookla CLI") {
		t.Fatalf("preflight error = %v", err)
	}
}
