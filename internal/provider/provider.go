package provider

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/soundadam/njuprobe/internal/id"
	"github.com/soundadam/njuprobe/internal/model"
)

var ErrUnavailable = errors.New("measurement provider unavailable")

type ProgressPhase string

const (
	ProgressWaiting     ProgressPhase = "waiting"
	ProgressStarting    ProgressPhase = "starting"
	ProgressConnecting  ProgressPhase = "connecting"
	ProgressMeasuring   ProgressPhase = "measuring"
	ProgressDownloading ProgressPhase = "downloading"
	ProgressUploading   ProgressPhase = "uploading"
	ProgressComplete    ProgressPhase = "complete"
	ProgressFailed      ProgressPhase = "failed"
	ProgressCancelled   ProgressPhase = "cancelled"
)

type ProgressEvent struct {
	Provider     model.Provider
	Phase        ProgressPhase
	Test         string
	Server       string
	LiveMbps     *float64
	DownloadMbps *float64
	UploadMbps   *float64
	Message      string
	Network      *model.NetworkContext
}

type ProgressSink func(ProgressEvent)

type Request struct {
	Command  model.Command
	Targets  []model.Provider
	IPFamily string
	Label    *string
	Note     *string
	Progress ProgressSink
}

func (request Request) Report(event ProgressEvent) {
	if request.Progress != nil {
		request.Progress(event)
	}
}

type Runner interface {
	Run(context.Context, Request) (model.RunSummary, error)
}

type PreflightRunner interface {
	Preflight(context.Context, Request) error
}

type MeasurementProvider interface {
	Measure(context.Context, Request) (model.Measurement, error)
}

type ProviderPreflight interface {
	Preflight(context.Context, Request) error
}

type SummaryRunner struct {
	ToolVersion string
	Campus      MeasurementProvider
	MLab        MeasurementProvider
	Providers   map[model.Provider]MeasurementProvider
	Now         func() time.Time
	NewRunID    func() (string, error)
	Snapshot    func() model.NetworkContext
}

func (runner SummaryRunner) Preflight(ctx context.Context, request Request) error {
	providers, err := runner.providersFor(request)
	if err != nil {
		return err
	}
	for _, entry := range providers {
		if preflight, ok := entry.provider.(ProviderPreflight); ok {
			providerRequest := request
			providerRequest.Targets = []model.Provider{entry.kind}
			if err := preflight.Preflight(ctx, providerRequest); err != nil {
				return err
			}
		}
	}
	return nil
}

func (runner SummaryRunner) Run(ctx context.Context, request Request) (model.RunSummary, error) {
	runner.setDefaults()
	providers, err := runner.providersFor(request)
	if err != nil {
		return model.RunSummary{}, err
	}

	startedAt := runner.Now()
	runID, err := runner.NewRunID()
	if err != nil {
		return model.RunSummary{}, fmt.Errorf("generate run ID: %w", err)
	}
	network := runner.Snapshot()
	request.Report(ProgressEvent{Network: &network})

	measurements := make([]model.Measurement, 0, len(providers))
	for index, entry := range providers {
		request.Report(ProgressEvent{Provider: entry.kind, Phase: ProgressStarting})
		providerRequest := request
		providerRequest.Targets = []model.Provider{entry.kind}
		measurement, err := entry.provider.Measure(ctx, providerRequest)
		if err != nil {
			request.Report(ProgressEvent{Provider: entry.kind, Phase: ProgressFailed, Message: err.Error()})
			return model.RunSummary{}, err
		}
		measurements = append(measurements, measurement)
		request.Report(progressFromMeasurement(measurement))
		if measurement.Status == model.ProviderStatusCancelled {
			for _, remaining := range providers[index+1:] {
				measurements = append(measurements, skippedMeasurement(remaining.kind))
			}
			break
		}
	}

	summary := model.RunSummary{
		SchemaVersion: model.SchemaVersion,
		RunID:         runID,
		ToolVersion:   runner.ToolVersion,
		StartedAt:     startedAt,
		EndedAt:       runner.Now(),
		Command:       request.Command,
		Targets:       cloneProviders(request.Targets),
		Status:        model.DeriveRunStatus(measurements),
		Label:         request.Label,
		Note:          request.Note,
		Network:       network,
		Measurements:  measurements,
	}
	if err := summary.Validate(); err != nil {
		return model.RunSummary{}, fmt.Errorf("validate generated summary: %w", err)
	}
	return summary, nil
}

func progressFromMeasurement(measurement model.Measurement) ProgressEvent {
	phase := ProgressComplete
	message := ""
	switch measurement.Status {
	case model.ProviderStatusFailed:
		phase = ProgressFailed
		if measurement.Failure != nil {
			message = measurement.Failure.Message
		}
	case model.ProviderStatusCancelled:
		phase = ProgressCancelled
	case model.ProviderStatusSkipped:
		phase = ProgressWaiting
	}
	server := ""
	if measurement.ServerFQDN != nil {
		server = *measurement.ServerFQDN
	}
	return ProgressEvent{
		Provider:     measurement.Provider,
		Phase:        phase,
		Server:       server,
		DownloadMbps: measurement.DownloadMbps,
		UploadMbps:   measurement.UploadMbps,
		Message:      message,
	}
}

func (runner *SummaryRunner) setDefaults() {
	if runner.ToolVersion == "" {
		runner.ToolVersion = "dev"
	}
	if runner.Now == nil {
		runner.Now = time.Now
	}
	if runner.NewRunID == nil {
		runner.NewRunID = id.NewRunID
	}
	if runner.Snapshot == nil {
		runner.Snapshot = func() model.NetworkContext {
			return model.NetworkContext{
				OS:           runtime.GOOS,
				Architecture: runtime.GOARCH,
			}
		}
	}
}

type providerEntry struct {
	kind     model.Provider
	provider MeasurementProvider
}

func (runner SummaryRunner) providersFor(request Request) ([]providerEntry, error) {
	entries := make([]providerEntry, 0, 8)
	appendProvider := func(kind model.Provider, implementation MeasurementProvider) error {
		if implementation == nil {
			return fmt.Errorf("%w: %s", ErrUnavailable, kind)
		}
		entries = append(entries, providerEntry{kind: kind, provider: implementation})
		return nil
	}

	if len(request.Targets) > 0 {
		seen := make(map[model.Provider]struct{}, len(request.Targets))
		for _, kind := range request.Targets {
			if !model.ProviderValid(kind) {
				return nil, fmt.Errorf("unsupported measurement target %q", kind)
			}
			if _, exists := seen[kind]; exists {
				return nil, fmt.Errorf("duplicate measurement target %q", kind)
			}
			seen[kind] = struct{}{}
			implementation := runner.Providers[kind]
			if kind == model.ProviderMLab && implementation == nil {
				implementation = runner.MLab
			}
			if kind == model.ProviderCampus && implementation == nil {
				implementation = runner.Campus
			}
			if err := appendProvider(kind, implementation); err != nil {
				return nil, err
			}
		}
		return entries, nil
	}

	switch request.Command {
	case model.CommandCampus:
		if err := appendProvider(model.ProviderCampus, runner.Campus); err != nil {
			return nil, err
		}
	case model.CommandMLab:
		if err := appendProvider(model.ProviderMLab, runner.MLab); err != nil {
			return nil, err
		}
	case model.CommandRun:
		if err := appendProvider(model.ProviderCampus, runner.Campus); err != nil {
			return nil, err
		}
		if err := appendProvider(model.ProviderMLab, runner.MLab); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported measurement command %q", request.Command)
	}
	return entries, nil
}

func skippedMeasurement(provider model.Provider) model.Measurement {
	return model.Measurement{
		Provider: provider,
		Method:   model.ProviderMethod(provider),
		Status:   model.ProviderStatusSkipped,
	}
}

func cloneProviders(providers []model.Provider) []model.Provider {
	if len(providers) == 0 {
		return nil
	}
	result := make([]model.Provider, len(providers))
	copy(result, providers)
	return result
}
