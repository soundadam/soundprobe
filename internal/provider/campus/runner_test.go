package campus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njuprobe/internal/helper"
	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
)

func TestRunnerMeasuresIPv4WithExactArguments(t *testing.T) {
	runner, argsPath := newFakeRunner(t, "testdata/librespeed-success-ipv4.json", HelperVersion, 0, "")
	measurement, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Status != model.ProviderStatusSuccess {
		t.Fatalf("status = %q", measurement.Status)
	}
	if measurement.IPFamily == nil || *measurement.IPFamily != "ipv4" {
		t.Fatalf("family = %v", measurement.IPFamily)
	}
	gotArgs := readArgs(t, argsPath)
	wantArgs := []string{
		"--server-json", ServerListURL,
		"--server", IPv4ServerID,
		"--duration", "10",
		"--concurrent", "3",
		"--no-icmp",
		"--json",
		"--ipv4",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestRunnerMeasuresIPv6WithoutFallback(t *testing.T) {
	runner, argsPath := newFakeRunner(t, "testdata/librespeed-success-ipv6.json", HelperVersion, 0, "")
	measurement, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus, IPFamily: "ipv6"})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.ServerFQDN == nil || *measurement.ServerFQDN != "speed6.nju.edu.cn" {
		t.Fatalf("server = %v", measurement.ServerFQDN)
	}
	gotArgs := readArgs(t, argsPath)
	if !containsPair(gotArgs, "--server", IPv6ServerID) || !contains(gotArgs, "--ipv6") || contains(gotArgs, "--ipv4") {
		t.Fatalf("IPv6 arguments = %#v", gotArgs)
	}
}

func TestRunnerRejectsWrongHelperVersion(t *testing.T) {
	runner, _ := newFakeRunner(t, "testdata/librespeed-success-ipv4.json", "v1.0.12", 0, "")
	_, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestRunnerMapsEmptyResultToConnectFailure(t *testing.T) {
	runner, _ := newFakeRunner(t, "", HelperVersion, 0, "[]")
	measurement, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Failure == nil || measurement.Failure.Stage != model.FailureStageConnect || measurement.Failure.Code != "server_unreachable" {
		t.Fatalf("failure = %#v", measurement.Failure)
	}
}

func TestRunnerRejectsMalformedHelperOutput(t *testing.T) {
	runner, _ := newFakeRunner(t, "", HelperVersion, 0, "not-json")
	_, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if err == nil || !strings.Contains(err.Error(), "parse LibreSpeed helper output") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunnerClassifiesDownloadFailure(t *testing.T) {
	runner, _ := newFakeRunner(t, "", HelperVersion, 1, "Failed to get download speed: connection reset by peer")
	measurement, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Failure == nil || measurement.Failure.Stage != model.FailureStageDownload || measurement.Failure.Code != "download_failure" {
		t.Fatalf("failure = %#v", measurement.Failure)
	}
	if measurement.DownloadMbps == nil || *measurement.DownloadMbps != 0 || measurement.UploadMbps == nil || *measurement.UploadMbps != 0 {
		t.Fatalf("failed speeds = %v/%v", measurement.DownloadMbps, measurement.UploadMbps)
	}
}

func TestRunnerTimesOutMeasurement(t *testing.T) {
	runner, _ := newFakeRunner(t, "", HelperVersion, 0, "")
	t.Setenv("NJUPROBE_FAKE_SLEEP", "1")
	runner.Timeout = 25 * time.Millisecond
	measurement, err := runner.Measure(context.Background(), provider.Request{Command: model.CommandCampus})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Failure == nil || measurement.Failure.Stage != model.FailureStageTimeout {
		t.Fatalf("failure = %#v", measurement.Failure)
	}
}

func TestRunnerPreservesCancellation(t *testing.T) {
	runner, _ := newFakeRunner(t, "", HelperVersion, 0, "")
	t.Setenv("NJUPROBE_FAKE_SLEEP", "5")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	measurement, err := runner.Measure(ctx, provider.Request{Command: model.CommandCampus})
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Status != model.ProviderStatusCancelled || measurement.Failure == nil || measurement.Failure.Stage != model.FailureStageCancelled {
		t.Fatalf("measurement = %#v", measurement)
	}
}

func newFakeRunner(t *testing.T, fixture, version string, exitCode int, literalOutput string) (*Runner, string) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "prefix", "bin", "njuprobe")
	helperPath := filepath.Join(root, "prefix", "libexec", "njuprobe", HelperName)
	argsPath := filepath.Join(root, "args.txt")
	writeTestExecutable(t, executable, "#!/bin/sh\nexit 0\n")

	fixturePath := ""
	if fixture != "" {
		absolute, err := filepath.Abs(fixture)
		if err != nil {
			t.Fatal(err)
		}
		fixturePath = absolute
	}
	script := `#!/bin/sh
if [ "${1:-}" = "--version" ]; then
  printf 'librespeed-cli %s (built on test)\n'
  exit 0
fi
printf '%%s\n' "$@" > "$NJUPROBE_FAKE_ARGS"
if [ -n "${NJUPROBE_FAKE_SLEEP:-}" ]; then
  exec sleep "$NJUPROBE_FAKE_SLEEP"
fi
if [ -n "%s" ]; then
  cat "%s"
else
  printf '%%s' '%s'
fi
if [ %d -ne 0 ]; then
  printf '%%s\n' '%s' >&2
fi
exit %d
`
	script = fmt.Sprintf(script, version, fixturePath, fixturePath, literalOutput, exitCode, literalOutput, exitCode)
	writeTestExecutable(t, helperPath, script)
	t.Setenv("NJUPROBE_FAKE_ARGS", argsPath)

	clock := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	return &Runner{
		Resolver: helper.Resolver{
			ExecutablePath:   executable,
			WorkingDirectory: root,
			LookupPath: func(string) (string, error) {
				return "", errors.New("not found")
			},
		},
		Now: func() time.Time {
			clock = clock.Add(250 * time.Millisecond)
			return clock
		},
	}, argsPath
}

func writeTestExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsPair(values []string, first, second string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == first && values[index+1] == second {
			return true
		}
	}
	return false
}
