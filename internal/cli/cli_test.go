package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/soundprobe/internal/consent"
	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/provider"
	"github.com/soundadam/soundprobe/internal/storage"
	"github.com/soundadam/soundprobe/internal/target"
)

type fakeRunner struct {
	request      provider.Request
	preflightErr error
	summary      model.RunSummary
	err          error
}

type fakeProgressRenderer struct {
	events []provider.ProgressEvent
	closed bool
}

func (renderer *fakeProgressRenderer) Update(event provider.ProgressEvent) {
	renderer.events = append(renderer.events, event)
}

func (renderer *fakeProgressRenderer) Close() error {
	renderer.closed = true
	return nil
}

func (runner *fakeRunner) Preflight(_ context.Context, request provider.Request) error {
	runner.request = request
	return runner.preflightErr
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
	if !strings.Contains(stdout.String(), "NJU Campus · IPv4") || !strings.Contains(stdout.String(), "M-Lab") {
		t.Fatalf("summary output missing targets: %q", stdout.String())
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

func TestUnavailableMLabFailsBeforeConsent(t *testing.T) {
	runner := &fakeRunner{preflightErr: fmt.Errorf("%w: mlab", provider.ErrUnavailable)}
	app, stdout, _ := newTestApp(t, runner)
	exitCode := app.Execute(context.Background(), []string{"mlab", "--json"})
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout.String(), "measurement_unavailable") || strings.Contains(stdout.String(), "consent_required") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	_, accepted, err := app.Consent.Status()
	if err != nil {
		t.Fatal(err)
	}
	if accepted {
		t.Fatal("consent was unexpectedly recorded")
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

func TestLastReturnsNewestSavedRun(t *testing.T) {
	app, stdout, stderr := newTestApp(t, &fakeRunner{})
	older := successfulSummary(model.CommandCampus)
	older.RunID = "00000000-0000-4000-8000-000000000020"
	newer := successfulSummary(model.CommandCampus)
	newer.RunID = "00000000-0000-4000-8000-000000000021"
	newer.StartedAt = newer.StartedAt.Add(time.Hour)
	newer.EndedAt = newer.EndedAt.Add(time.Hour)
	if err := app.History.Save(older); err != nil {
		t.Fatal(err)
	}
	if err := app.History.Save(newer); err != nil {
		t.Fatal(err)
	}
	if exitCode := app.Execute(context.Background(), []string{"last", "--json"}); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	var decoded model.RunSummary
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RunID != newer.RunID {
		t.Fatalf("run ID = %q, want %q", decoded.RunID, newer.RunID)
	}
}

func TestDoctorReportsReadyProviders(t *testing.T) {
	app, stdout, stderr := newTestApp(t, &fakeRunner{})
	if exitCode := app.Execute(context.Background(), []string{"doctor", "--json"}); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	var payload struct {
		Ready     bool              `json:"ready"`
		Providers map[string]string `json:"providers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Ready || payload.Providers["campus"] != "ready" || payload.Providers["mlab"] != "ready" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestInteractiveMeasurementUsesProgressRenderer(t *testing.T) {
	runner := &fakeRunner{summary: successfulSummary(model.CommandCampus)}
	app, _, stderr := newTestApp(t, runner)
	app.StdoutTTY = true
	progress := &fakeProgressRenderer{}
	app.ProgressFactory = func(io.Writer, string, []model.Provider) (progressRenderer, error) {
		return progress, nil
	}
	if exitCode := app.Execute(context.Background(), []string{"campus", "--no-save"}); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if runner.request.Progress == nil {
		t.Fatal("progress sink was not passed to runner")
	}
	runner.request.Report(provider.ProgressEvent{Provider: model.ProviderCampus, Phase: provider.ProgressMeasuring})
	if len(progress.events) != 1 || !progress.closed {
		t.Fatalf("renderer = %#v", progress)
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
	if !strings.Contains(stdout.String(), consent.PolicyVersion) || !strings.Contains(stdout.String(), consent.PolicyURL) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestBareTTYUsesSelectorPlan(t *testing.T) {
	providers := []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}
	runner := &fakeRunner{summary: summaryForProviders(model.CommandRun, providers)}
	app, _, stderr := newTestApp(t, runner)
	app.StdoutTTY = true
	if _, err := app.Consent.Accept(app.Version, app.Now()); err != nil {
		t.Fatal(err)
	}
	app.SelectorFactory = func(context.Context, io.Reader, io.Writer, string) (target.Plan, error) {
		return target.Plan{StationIDs: []string{"nju-campus", "mlab"}, Family: target.FamilyIPv4, Providers: providers}, nil
	}
	app.ProgressFactory = func(io.Writer, string, []model.Provider) (progressRenderer, error) {
		return &fakeProgressRenderer{}, nil
	}
	if exitCode := app.Execute(context.Background(), nil); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if len(runner.request.Targets) != 2 || runner.request.Targets[0] != model.ProviderNJUCampusIPv4 || runner.request.Targets[1] != model.ProviderMLab {
		t.Fatalf("targets = %#v", runner.request.Targets)
	}
}

func TestRunTargetAndFamilyFlagsExpandDeterministically(t *testing.T) {
	providers := []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderNJUCampusIPv6, model.ProviderQLUIPv4}
	runner := &fakeRunner{summary: summaryForProviders(model.CommandRun, providers)}
	app, _, stderr := newTestApp(t, runner)
	exitCode := app.Execute(context.Background(), []string{"run", "--targets", "nju-campus,qlu", "--family", "dual", "--no-save"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if fmt.Sprint(runner.request.Targets) != fmt.Sprint(providers) {
		t.Fatalf("targets = %#v, want %#v", runner.request.Targets, providers)
	}
}

func TestDomesticDefaultsToThreeIPv4Stations(t *testing.T) {
	providers := []model.Provider{model.ProviderCERNETIPv4, model.ProviderQLUIPv4, model.ProviderTongjiIPv4}
	runner := &fakeRunner{summary: summaryForProviders(model.CommandDomestic, providers)}
	app, _, stderr := newTestApp(t, runner)
	if exitCode := app.Execute(context.Background(), []string{"domestic", "--no-save"}); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if fmt.Sprint(runner.request.Targets) != fmt.Sprint(providers) {
		t.Fatalf("targets = %#v", runner.request.Targets)
	}
}

func TestEdgeCommandReportsTerminalUnsupported(t *testing.T) {
	app, _, stderr := newTestApp(t, &fakeRunner{})
	if exitCode := app.Execute(context.Background(), []string{"edge", "--no-save"}); exitCode != 1 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "unavailable in terminal mode") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDomesticRejectsNonDomesticTarget(t *testing.T) {
	app, _, stderr := newTestApp(t, &fakeRunner{})
	if exitCode := app.Execute(context.Background(), []string{"domestic", "--targets", "nju-edge"}); exitCode != 1 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "not a domestic station") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestIPv6RejectsIPv4OnlyStation(t *testing.T) {
	app, _, stderr := newTestApp(t, &fakeRunner{})
	if exitCode := app.Execute(context.Background(), []string{"run", "--targets", "qlu", "--family", "ipv6"}); exitCode != 1 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "does not support IPv6") {
		t.Fatalf("stderr = %q", stderr.String())
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
		StdoutTTY: false,
		Version:   "test",
		Runner:    runner,
		History:   storage.New(filepath.Join(root, "history")),
		Consent:   consent.New(filepath.Join(root, "consent.json")),
		Now:       func() time.Time { return now },
	}, stdout, stderr
}

func successfulSummary(command model.Command) model.RunSummary {
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	providers := []model.Provider{model.ProviderNJUCampusIPv4}
	if command == model.CommandMLab {
		providers = []model.Provider{model.ProviderMLab}
	}
	if command == model.CommandRun {
		providers = []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab}
	}
	measurements := make([]model.Measurement, 0, len(providers))
	for _, measurementProvider := range providers {
		measurement := model.Measurement{
			Provider:     measurementProvider,
			Method:       model.ProviderMethod(measurementProvider),
			Status:       model.ProviderStatusSuccess,
			DownloadMbps: model.Pointer(100.0),
			UploadMbps:   model.Pointer(50.0),
		}
		if measurementProvider == model.ProviderMLab {
			measurement.DownloadMbps = model.Pointer(80.0)
			measurement.UploadMbps = model.Pointer(40.0)
		}
		measurements = append(measurements, measurement)
	}
	return model.RunSummary{
		SchemaVersion: model.SchemaVersion,
		RunID:         "00000000-0000-4000-8000-000000000001",
		ToolVersion:   "test",
		StartedAt:     now,
		EndedAt:       now.Add(time.Second),
		Command:       command,
		Targets:       providers,
		Status:        model.RunStatusSuccess,
		Measurements:  measurements,
	}
}

func summaryForProviders(command model.Command, providers []model.Provider) model.RunSummary {
	now := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	measurements := make([]model.Measurement, 0, len(providers))
	for _, measurementProvider := range providers {
		measurements = append(measurements, model.Measurement{
			Provider:     measurementProvider,
			Method:       model.ProviderMethod(measurementProvider),
			Status:       model.ProviderStatusSuccess,
			DownloadMbps: model.Pointer(100.0),
			UploadMbps:   model.Pointer(50.0),
		})
	}
	return model.RunSummary{
		SchemaVersion: model.SchemaVersion,
		RunID:         "00000000-0000-4000-8000-000000000031",
		ToolVersion:   "test",
		StartedAt:     now,
		EndedAt:       now.Add(time.Second),
		Command:       command,
		Targets:       append([]model.Provider(nil), providers...),
		Status:        model.RunStatusSuccess,
		Measurements:  measurements,
	}
}
