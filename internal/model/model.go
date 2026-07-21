package model

import (
	"errors"
	"fmt"
	"time"
)

const SchemaVersion = 1

type Command string

const (
	CommandRun    Command = "run"
	CommandCampus Command = "campus"
	CommandMLab   Command = "mlab"
)

type Provider string

const (
	ProviderCampus Provider = "campus"
	ProviderMLab   Provider = "mlab"
)

type RunStatus string

const (
	RunStatusSuccess   RunStatus = "success"
	RunStatusPartial   RunStatus = "partial"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

type ProviderStatus string

const (
	ProviderStatusSuccess   ProviderStatus = "success"
	ProviderStatusFailed    ProviderStatus = "failed"
	ProviderStatusSkipped   ProviderStatus = "skipped"
	ProviderStatusCancelled ProviderStatus = "cancelled"
)

type FailureStage string

const (
	FailureStageHelper    FailureStage = "helper"
	FailureStageDNS       FailureStage = "dns"
	FailureStageConnect   FailureStage = "connect"
	FailureStageDownload  FailureStage = "download"
	FailureStageUpload    FailureStage = "upload"
	FailureStageTimeout   FailureStage = "timeout"
	FailureStageCancelled FailureStage = "cancelled"
)

type Failure struct {
	Stage   FailureStage `json:"stage"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
}

type NetworkContext struct {
	OS              string   `json:"os,omitempty"`
	Architecture    string   `json:"architecture,omitempty"`
	ActiveInterface *string  `json:"activeInterface"`
	InterfaceKind   *string  `json:"interfaceKind"`
	SSID            *string  `json:"ssid"`
	BSSID           *string  `json:"bssid"`
	LocalIPv4       []string `json:"localIPv4,omitempty"`
	LocalIPv6       []string `json:"localIPv6,omitempty"`
	DefaultGateway  *string  `json:"defaultGateway"`
	DNSServers      []string `json:"dnsServers,omitempty"`
}

type Measurement struct {
	Provider       Provider       `json:"provider"`
	Method         string         `json:"method"`
	Status         ProviderStatus `json:"status"`
	IPFamily       *string        `json:"ipFamily"`
	ServerName     *string        `json:"serverName"`
	ServerAddress  *string        `json:"serverAddress"`
	ClientPublicIP *string        `json:"clientPublicIp"`
	PingMS         *float64       `json:"pingMs"`
	JitterMS       *float64       `json:"jitterMs"`
	DownloadMbps   *float64       `json:"downloadMbps"`
	UploadMbps     *float64       `json:"uploadMbps"`
	DownloadBytes  *int64         `json:"downloadBytes"`
	UploadBytes    *int64         `json:"uploadBytes"`
	DurationMS     *int64         `json:"durationMs"`
	Concurrency    *int           `json:"concurrency"`
	HelperVersion  *string        `json:"helperVersion"`
	Failure        *Failure       `json:"failure,omitempty"`
}

type RunSummary struct {
	SchemaVersion int            `json:"schemaVersion"`
	RunID         string         `json:"runId"`
	ToolVersion   string         `json:"toolVersion"`
	StartedAt     time.Time      `json:"startedAt"`
	EndedAt       time.Time      `json:"endedAt"`
	Command       Command        `json:"command"`
	Status        RunStatus      `json:"status"`
	Label         *string        `json:"label"`
	Note          *string        `json:"note"`
	Network       NetworkContext `json:"network"`
	Measurements  []Measurement  `json:"measurements"`
}

func (summary RunSummary) Validate() error {
	if summary.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", summary.SchemaVersion)
	}
	if summary.RunID == "" {
		return errors.New("run ID is required")
	}
	if summary.ToolVersion == "" {
		return errors.New("tool version is required")
	}
	if summary.StartedAt.IsZero() || summary.EndedAt.IsZero() {
		return errors.New("start and end timestamps are required")
	}
	if summary.EndedAt.Before(summary.StartedAt) {
		return errors.New("end timestamp precedes start timestamp")
	}
	if !summary.Command.valid() {
		return fmt.Errorf("invalid command %q", summary.Command)
	}
	if !summary.Status.valid() {
		return fmt.Errorf("invalid run status %q", summary.Status)
	}

	expectedProviders := summary.Command.expectedProviders()
	if len(summary.Measurements) != len(expectedProviders) {
		return fmt.Errorf("command %q requires %d measurements, got %d", summary.Command, len(expectedProviders), len(summary.Measurements))
	}
	seenProviders := make(map[Provider]struct{}, len(summary.Measurements))
	for index, measurement := range summary.Measurements {
		if !measurement.Provider.valid() {
			return fmt.Errorf("measurement %d: invalid provider %q", index, measurement.Provider)
		}
		if _, exists := seenProviders[measurement.Provider]; exists {
			return fmt.Errorf("measurement %d: duplicate provider %q", index, measurement.Provider)
		}
		seenProviders[measurement.Provider] = struct{}{}
		if _, expected := expectedProviders[measurement.Provider]; !expected {
			return fmt.Errorf("measurement %d: provider %q is not valid for command %q", index, measurement.Provider, summary.Command)
		}
		if measurement.Method != measurement.Provider.method() {
			return fmt.Errorf("measurement %d: method %q does not match provider %q", index, measurement.Method, measurement.Provider)
		}
		if !measurement.Status.valid() {
			return fmt.Errorf("measurement %d: invalid status %q", index, measurement.Status)
		}
		if err := measurement.validateValues(); err != nil {
			return fmt.Errorf("measurement %d: %w", index, err)
		}
	}
	if derived := DeriveRunStatus(summary.Measurements); summary.Status != derived {
		return fmt.Errorf("run status %q does not match measurement status %q", summary.Status, derived)
	}
	return nil
}

func (command Command) valid() bool {
	switch command {
	case CommandRun, CommandCampus, CommandMLab:
		return true
	default:
		return false
	}
}

func (command Command) expectedProviders() map[Provider]struct{} {
	switch command {
	case CommandCampus:
		return map[Provider]struct{}{ProviderCampus: {}}
	case CommandMLab:
		return map[Provider]struct{}{ProviderMLab: {}}
	case CommandRun:
		return map[Provider]struct{}{ProviderCampus: {}, ProviderMLab: {}}
	default:
		return nil
	}
}

func (provider Provider) valid() bool {
	return provider == ProviderCampus || provider == ProviderMLab
}

func (provider Provider) method() string {
	switch provider {
	case ProviderCampus:
		return "librespeed-three-stream"
	case ProviderMLab:
		return "ndt7-single-stream"
	default:
		return ""
	}
}

func (status RunStatus) valid() bool {
	switch status {
	case RunStatusSuccess, RunStatusPartial, RunStatusFailed, RunStatusCancelled:
		return true
	default:
		return false
	}
}

func (status ProviderStatus) valid() bool {
	switch status {
	case ProviderStatusSuccess, ProviderStatusFailed, ProviderStatusSkipped, ProviderStatusCancelled:
		return true
	default:
		return false
	}
}

func (measurement Measurement) validateValues() error {
	if err := validateNonnegative("download Mbps", measurement.DownloadMbps); err != nil {
		return err
	}
	if err := validateNonnegative("upload Mbps", measurement.UploadMbps); err != nil {
		return err
	}
	if err := validateNonnegative("ping milliseconds", measurement.PingMS); err != nil {
		return err
	}
	if err := validateNonnegative("jitter milliseconds", measurement.JitterMS); err != nil {
		return err
	}

	switch measurement.Status {
	case ProviderStatusSuccess:
		if measurement.DownloadMbps == nil || measurement.UploadMbps == nil {
			return errors.New("successful measurement speeds must be present")
		}
		if measurement.Failure != nil {
			return errors.New("successful measurement must not contain a failure")
		}
	case ProviderStatusFailed:
		if measurement.DownloadMbps == nil || measurement.UploadMbps == nil {
			return errors.New("failed attempted measurement speeds must be zero or measured, not null")
		}
		if measurement.Failure == nil {
			return errors.New("failed measurement requires a failure object")
		}
	case ProviderStatusSkipped:
		if measurement.DownloadMbps != nil || measurement.UploadMbps != nil {
			return errors.New("skipped measurement speeds must be null")
		}
		if measurement.Failure != nil {
			return errors.New("skipped measurement must not contain a failure")
		}
	case ProviderStatusCancelled:
		if measurement.Failure == nil || measurement.Failure.Stage != FailureStageCancelled {
			return errors.New("cancelled measurement requires a cancelled failure")
		}
	}

	if measurement.Failure != nil {
		if !measurement.Failure.Stage.valid() {
			return fmt.Errorf("invalid failure stage %q", measurement.Failure.Stage)
		}
		if measurement.Failure.Code == "" || measurement.Failure.Message == "" {
			return errors.New("failure code and message are required")
		}
	}
	return nil
}

func (stage FailureStage) valid() bool {
	switch stage {
	case FailureStageHelper, FailureStageDNS, FailureStageConnect, FailureStageDownload,
		FailureStageUpload, FailureStageTimeout, FailureStageCancelled:
		return true
	default:
		return false
	}
}

func validateNonnegative(name string, value *float64) error {
	if value != nil && *value < 0 {
		return fmt.Errorf("%s must be non-negative", name)
	}
	return nil
}

func DeriveRunStatus(measurements []Measurement) RunStatus {
	if len(measurements) == 0 {
		return RunStatusFailed
	}

	successes := 0
	for _, measurement := range measurements {
		switch measurement.Status {
		case ProviderStatusCancelled:
			return RunStatusCancelled
		case ProviderStatusSuccess:
			successes++
		}
	}

	if successes == len(measurements) {
		return RunStatusSuccess
	}
	if successes > 0 {
		return RunStatusPartial
	}
	return RunStatusFailed
}

func (summary RunSummary) ExitCode() int {
	switch summary.Status {
	case RunStatusSuccess:
		return 0
	case RunStatusPartial, RunStatusFailed:
		return 2
	case RunStatusCancelled:
		return 130
	default:
		return 1
	}
}

func Pointer[T any](value T) *T {
	return &value
}
