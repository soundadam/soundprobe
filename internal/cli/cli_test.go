package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njuprobe/internal/consent"
	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
	"github.com/soundadam/njuprobe/internal/storage"
)

type fakeRunner struct {
	request provider.Request
	summary model.RunSummary
	err     error
}

func (runner *fakeRunner) Run(_ context.Context, request provider.Request) (model.RunSummary, error) {
	runner.request = request
	return runner.summary, runner.err
}

func TestVersionJSON(t *testing.T) {
	app, stdout, _ := newTestApp(t, &fakeRunner{})
	exitCode := app.Execute(context.Background(), []string{"version", "--json"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	var payload map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload["version"] != "test" {
		t.Fatalf("version = %q, want test", payload["version"])
	}
}

func TestBareCommandRunsBothProvidersAfterConsent(t *testing.T) {
	runner := &fakeRunner{summary: successfulSummary(model.CommandRun)}
	app, stdout, stderr := newTestApp(t, runner)
	if _, err := app.Consent.Accept(app.Version, app.Now()); err != nil {
		t.Fatal(err)
	}

	exitCode := app.Execute(context.Background(), nil)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if runner.request.Command != model.CommandRun {
		t.Fatalf("command = %q, want run", runner.request.Command)
	}
	if !strings.Contains(stdout.String(), "campus") || !strings.Contains(stdout.String(), "mlab") {
		t.Fatalf("summary output missing providers: %q", stdout.String())
	}
}

func TestCampusIPFlagsAreMutuallyExclusive(t *testing.T) {
	app, _, stderr := newTestApp(t, &fakeRunner{})
	exitCode := app.Execute(context.Background(), []string{"campus", "--ipv4", "--ipv6"})
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "mutually exclusive") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestMLabFailsClosedWithoutNoninteractiveConsent(t *testing.T) {
	app, stdout, _ := newTestApp(t, &fakeRunner{})
	app.StdinTTY = false
	exitCode := app.Execute(context.Background(), []string{"mlab", "--json"})
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout.String(), "consent_required") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestSuccessfulRunIsSaved(t *testing.T) {
	runner := &fakeRunner{summary: successfulSummary(model.CommandCampus)}
	app, _, stderr := newTestApp(t, runner)
	exitCode := app.Execute(context.Background(), []string{"campus"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	loaded, err := app.History.Load(runner.summary.RunID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.RunID != runner.summary.RunID {
		t.Fatalf("saved run ID = %q, want %q", loaded.RunID, runner.summary.RunID)
	}
}

func TestNoSaveLeavesHistoryEmpty(t *testing.T) {
	runner := &fakeRunner{summary: successfulSummary(model.CommandCampus)}
	app, _, stderr := newTestApp(t, runner)
	exitCode := app.Execute(context.Background(), []string{"campus", "--no-save"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	items, err := app.History.List(0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("saved runs = %d, want 0", len(items))
	}
}

func TestPartialRunReturnsTwoAndJSONHasNoANSI(t *testing.T) {
	summary := successfulSummary(model.CommandRun)
	summary.Status = model.RunStatusPartial
	summary.Measurements[1].Status = model.ProviderStatusFailed
	summary.Measurements[1].DownloadMbps = model.Pointer(0.0)
	summary.Measurements[1].UploadMbps = model.Pointer(0.0)
	summary.Measurements[1].Failure = &model.Failure{
		Stage:   model.FailureStageConnect,
		Code:    "unreachable",
		Message: "M-Lab server was unreachable",
	}
	runner := &fakeRunner{summary: summary}
	app, stdout, stderr := newTestApp(t, runner)
	if _, err := app.Consent.Accept(app.Version, app.Now()); err != nil {
		t.Fatal(err)
	}
	exitCode := app.Execute(context.Background(), []string{"run", "--json", "--no-save"})
	if exitCode != 2 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("JSON output contains ANSI: %q", stdout.String())
	}
	var decoded model.RunSummary
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Status != model.RunStatusPartial {
		t.Fatalf("status = %q, want partial", decoded.Status)
	}
}

func TestInteractiveConsentAccept(t *testing.T) {
	app, stdout, stderr := newTestApp(t, &fakeRunner{})
	app.In = strings.NewReader("accept\n")
	exitCode := app.Execute(context.Background(), []string{"consent", "accept"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	_, accepted, err := app.Consent.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !accepted {
		t.Fatal("consent was not recorded")
	}
	if !strings.Contains(stdout.String(), consent.PolicyVersion) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func newTestApp(t *testing.T, runner provider.Runner) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	return &App{
		In:        strings.NewReader(""),
		Out:       stdout,
		Err:       stderr,
		StdinTTY:  true,
		StdoutTTY: true,
		Version:   "test",
		Runner:    runner,
		History:   storage.New(filepath.Join(root, "history")),
		Consent:   consent.New(filepath.Join(root, "consent.json")),
		Now:       func() time.Time { return now },
	}, stdout, stderr
}

func successfulSummary(command model.Command) model.RunSummary {
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	measurements := []model.Measurement{{
		Provider:     model.ProviderCampus,
		Method:       "librespeed-three-stream",
		Status:       model.ProviderStatusSuccess,
		DownloadMbps: model.Pointer(100.0),
		UploadMbps:   model.Pointer(50.0),
	}}
	if command == model.CommandRun {
		measurements = append(measurements, model.Measurement{
			Provider:     model.ProviderMLab,
			Method:       "ndt7-single-stream",
			Status:       model.ProviderStatusSuccess,
			DownloadMbps: model.Pointer(80.0),
			UploadMbps:   model.Pointer(40.0),
		})
	}
	return model.RunSummary{
		SchemaVersion: model.SchemaVersion,
		RunID:         "00000000-0000-4000-8000-000000000001",
		ToolVersion:   "test",
		StartedAt:     now,
		EndedAt:       now.Add(time.Second),
		Command:       command,
		Status:        model.RunStatusSuccess,
		Measurements:  measurements,
	}
}
