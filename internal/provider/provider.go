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
	Now         func() time.Time
	NewRunID    func() (string, error)
	Snapshot    func() model.NetworkContext
}

func (runner SummaryRunner) Preflight(ctx context.Context, request Request) error {
	providers, err := runner.providersFor(request.Command)
	if err != nil {
		return err
	}
	for _, entry := range providers {
		if preflight, ok := entry.provider.(ProviderPreflight); ok {
			if err := preflight.Preflight(ctx, request); err != nil {
				return err
			}
		}
	}
	return nil
}

func (runner SummaryRunner) Run(ctx context.Context, request Request) (model.RunSummary, error) {
	runner.setDefaults()
	providers, err := runner.providersFor(request.Command)
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
		measurement, err := entry.provider.Measure(ctx, request)
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
	return ProgressEvent{
		Provider:     measurement.Provider,
		Phase:        phase,
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

func (runner SummaryRunner) providersFor(command model.Command) ([]providerEntry, error) {
	entries := make([]providerEntry, 0, 2)
	appendProvider := func(kind model.Provider, implementation MeasurementProvider) error {
		if implementation == nil {
			return fmt.Errorf("%w: %s", ErrUnavailable, kind)
		}
		entries = append(entries, providerEntry{kind: kind, provider: implementation})
		return nil
	}

	switch command {
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
		return nil, fmt.Errorf("unsupported measurement command %q", command)
	}
	return entries, nil
}

func skippedMeasurement(provider model.Provider) model.Measurement {
	method := ""
	switch provider {
	case model.ProviderCampus:
		method = model.MethodLibreSpeedThreeStream
	case model.ProviderMLab:
		method = model.MethodNDT7SingleStream
	}
	return model.Measurement{
		Provider: provider,
		Method:   method,
		Status:   model.ProviderStatusSkipped,
	}
}
