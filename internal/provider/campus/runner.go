package campus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/soundadam/njuprobe/internal/helper"
	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
)

const (
	HelperName          = "librespeed-cli"
	HelperVersion       = "v1.0.13"
	IPv4ServerID        = "1"
	IPv6ServerID        = "2"
	ConcurrentRequests  = 3
	MeasurementDuration = 10
)

const pinnedServerListJSON = `[
  {
    "id": 1,
    "name": "NJU Speed Test v4",
    "server": "http://speed.nju.edu.cn",
    "dlURL": "/backend/garbage.php",
    "ulURL": "/backend/empty.php",
    "pingURL": "/backend/empty.php",
    "getIpURL": "/backend/getIP.php"
  },
  {
    "id": 2,
    "name": "NJU Speed Test v6",
    "server": "http://speed6.nju.edu.cn",
    "dlURL": "/backend/garbage.php",
    "ulURL": "/backend/empty.php",
    "pingURL": "/backend/empty.php",
    "getIpURL": "/backend/getIP.php"
  }
]`

var versionPattern = regexp.MustCompile(`(?m)^librespeed-cli\s+(v?[0-9]+\.[0-9]+\.[0-9]+)\b`)

type HelperResolver interface {
	Resolve(string) (helper.Resolved, error)
}

type Runner struct {
	Resolver       HelperResolver
	Timeout        time.Duration
	VersionTimeout time.Duration
	Now            func() time.Time

	prepareMu sync.Mutex
	prepared  *preparedHelper
}

type preparedHelper struct {
	resolved helper.Resolved
	version  string
}

func New(resolver HelperResolver) *Runner {
	return &Runner{Resolver: resolver}
}

func (runner *Runner) Preflight(ctx context.Context, request provider.Request) error {
	runner.setDefaults()
	if _, _, _, err := selectedServer(request.IPFamily); err != nil {
		return err
	}
	_, _, err := runner.prepareHelper(ctx)
	return err
}

func (runner *Runner) Measure(ctx context.Context, request provider.Request) (model.Measurement, error) {
	runner.setDefaults()
	family, serverID, expectedHost, err := selectedServer(request.IPFamily)
	if err != nil {
		return model.Measurement{}, err
	}
	resolved, helperVersion, err := runner.prepareHelper(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return cancelledMeasurement(family, "", 0), nil
		}
		return model.Measurement{}, err
	}
	request.Report(provider.ProgressEvent{
		Provider: model.ProviderCampus,
		Phase:    provider.ProgressMeasuring,
		Server:   expectedHost,
	})

	measurementCtx, cancel := context.WithTimeout(ctx, runner.Timeout)
	defer cancel()
	startedAt := runner.Now()
	serverList := []byte(pinnedServerListJSON)
	if err := validateServerList(serverList, serverID, expectedHost); err != nil {
		return model.Measurement{}, fmt.Errorf("validate pinned NJU server list: %w", err)
	}
	args := []string{
		"--local-json", "-",
		"--server", serverID,
		"--duration", fmt.Sprintf("%d", MeasurementDuration),
		"--concurrent", fmt.Sprintf("%d", ConcurrentRequests),
		"--no-icmp",
		"--json",
		"--" + family,
	}
	command := exec.CommandContext(measurementCtx, resolved.Path, args...)
	stdout := newCappedBuffer(128 * 1024)
	stderr := newCappedBuffer(16 * 1024)
	command.Stdin = bytes.NewReader(serverList)
	command.Stdout = stdout
	command.Stderr = stderr
	err = command.Run()
	durationMS := runner.Now().Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	if err != nil {
		if errors.Is(measurementCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return cancelledMeasurement(family, helperVersion, durationMS), nil
		}
		if errors.Is(measurementCtx.Err(), context.DeadlineExceeded) {
			return failedMeasurement(family, helperVersion, durationMS, model.FailureStageTimeout, "timeout", "campus measurement timed out"), nil
		}
		stage, code := classifyFailure(stderr.String())
		message := sanitizeMessage(stderr.String())
		if message == "" {
			message = "LibreSpeed helper exited before producing a result"
		}
		if stage == model.FailureStageHelper {
			return model.Measurement{}, fmt.Errorf("LibreSpeed helper failed: %s", message)
		}
		return failedMeasurement(family, helperVersion, durationMS, stage, code, message), nil
	}

	measurement, err := parseResult(stdout.Bytes(), family, helperVersion, durationMS)
	if errors.Is(err, errNoResult) {
		return failedMeasurement(family, helperVersion, durationMS, model.FailureStageConnect, "server_unreachable", "NJU campus server did not produce a measurement"), nil
	}
	if err != nil {
		return model.Measurement{}, fmt.Errorf("parse LibreSpeed helper output: %w", err)
	}
	if measurement.ServerFQDN == nil || !strings.EqualFold(*measurement.ServerFQDN, expectedHost) {
		return model.Measurement{}, errors.New("LibreSpeed helper returned an unexpected campus server")
	}
	return measurement, nil
}

func (runner *Runner) prepareHelper(ctx context.Context) (helper.Resolved, string, error) {
	runner.prepareMu.Lock()
	defer runner.prepareMu.Unlock()
	if runner.prepared != nil {
		return runner.prepared.resolved, runner.prepared.version, nil
	}
	if runner.Resolver == nil {
		return helper.Resolved{}, "", fmt.Errorf("%w: LibreSpeed helper resolver is not configured", provider.ErrUnavailable)
	}
	resolved, err := runner.Resolver.Resolve(HelperName)
	if err != nil {
		return helper.Resolved{}, "", fmt.Errorf("%w: resolve LibreSpeed helper: %v", provider.ErrUnavailable, err)
	}
	version, err := runner.probeVersion(ctx, resolved.Path)
	if err != nil {
		return helper.Resolved{}, "", err
	}
	runner.prepared = &preparedHelper{resolved: resolved, version: version}
	return resolved, version, nil
}

func (runner *Runner) setDefaults() {
	if runner.Timeout <= 0 {
		runner.Timeout = 60 * time.Second
	}
	if runner.VersionTimeout <= 0 {
		runner.VersionTimeout = 5 * time.Second
	}
	if runner.Now == nil {
		runner.Now = time.Now
	}
}

type serverListEntry struct {
	ID     int    `json:"id"`
	Server string `json:"server"`
}

func validateServerList(data []byte, selectedID, expectedHost string) error {
	var entries []serverListEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	wantedID, err := strconv.Atoi(selectedID)
	if err != nil {
		return fmt.Errorf("invalid selected server ID %q", selectedID)
	}
	found := false
	for _, entry := range entries {
		if entry.ID != wantedID {
			continue
		}
		if found {
			return fmt.Errorf("server ID %d is duplicated", wantedID)
		}
		serverURL, err := url.Parse(entry.Server)
		if err != nil || serverURL.Hostname() == "" {
			return fmt.Errorf("server ID %d has invalid URL %q", wantedID, entry.Server)
		}
		if serverURL.Scheme != "http" && serverURL.Scheme != "https" {
			return fmt.Errorf("server ID %d has unsupported URL scheme %q", wantedID, serverURL.Scheme)
		}
		if serverURL.User != nil {
			return fmt.Errorf("server ID %d URL must not contain user information", wantedID)
		}
		if !strings.EqualFold(serverURL.Hostname(), expectedHost) {
			return fmt.Errorf("server ID %d resolves to unexpected host %q", wantedID, serverURL.Hostname())
		}
		found = true
	}
	if !found {
		return fmt.Errorf("server ID %d is missing", wantedID)
	}
	return nil
}

func (runner *Runner) probeVersion(ctx context.Context, path string) (string, error) {
	versionCtx, cancel := context.WithTimeout(ctx, runner.VersionTimeout)
	defer cancel()
	command := exec.CommandContext(versionCtx, path, "--version")
	stdout := newCappedBuffer(16 * 1024)
	stderr := newCappedBuffer(4 * 1024)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if errors.Is(versionCtx.Err(), context.Canceled) {
			return "", context.Canceled
		}
		if errors.Is(versionCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: LibreSpeed version probe timed out", provider.ErrUnavailable)
		}
		message := sanitizeMessage(stderr.String())
		if message == "" {
			message = "helper could not report its version"
		}
		return "", fmt.Errorf("%w: LibreSpeed version probe failed: %s", provider.ErrUnavailable, message)
	}
	matches := versionPattern.FindSubmatch(stdout.Bytes())
	if len(matches) != 2 {
		return "", fmt.Errorf("%w: LibreSpeed version output is unrecognized", provider.ErrUnavailable)
	}
	version := string(matches[1])
	if version[0] != 'v' {
		version = "v" + version
	}
	if version != HelperVersion {
		return "", fmt.Errorf("%w: LibreSpeed helper version %s does not match required %s", provider.ErrUnavailable, version, HelperVersion)
	}
	return version, nil
}

func selectedServer(requestedFamily string) (family, serverID, expectedHost string, err error) {
	switch requestedFamily {
	case "", "ipv4":
		return "ipv4", IPv4ServerID, "speed.nju.edu.cn", nil
	case "ipv6":
		return "ipv6", IPv6ServerID, "speed6.nju.edu.cn", nil
	default:
		return "", "", "", fmt.Errorf("unsupported campus IP family %q", requestedFamily)
	}
}

func failedMeasurement(family, version string, durationMS int64, stage model.FailureStage, code, message string) model.Measurement {
	zero := 0.0
	return model.Measurement{
		Provider:      model.ProviderCampus,
		Method:        model.MethodLibreSpeedThreeStream,
		Status:        model.ProviderStatusFailed,
		IPFamily:      model.Pointer(family),
		DownloadMbps:  model.Pointer(zero),
		UploadMbps:    model.Pointer(zero),
		DurationMS:    model.Pointer(durationMS),
		Concurrency:   model.Pointer(ConcurrentRequests),
		HelperVersion: optionalVersion(version),
		Failure: &model.Failure{
			Stage:   stage,
			Code:    code,
			Message: message,
		},
	}
}

func cancelledMeasurement(family, version string, durationMS int64) model.Measurement {
	return model.Measurement{
		Provider:      model.ProviderCampus,
		Method:        model.MethodLibreSpeedThreeStream,
		Status:        model.ProviderStatusCancelled,
		IPFamily:      model.Pointer(family),
		DurationMS:    model.Pointer(durationMS),
		Concurrency:   model.Pointer(ConcurrentRequests),
		HelperVersion: optionalVersion(version),
		Failure: &model.Failure{
			Stage:   model.FailureStageCancelled,
			Code:    "cancelled",
			Message: "campus measurement was cancelled",
		},
	}
}

func optionalVersion(version string) *string {
	if version == "" {
		return nil
	}
	return model.Pointer(version)
}

func classifyFailure(message string) (model.FailureStage, string) {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "no such host"), strings.Contains(lower, "temporary failure in name resolution"), strings.Contains(lower, "name or service not known"):
		return model.FailureStageDNS, "dns_failure"
	case strings.Contains(lower, "failed to get download speed"):
		return model.FailureStageDownload, "download_failure"
	case strings.Contains(lower, "failed to get upload speed"):
		return model.FailureStageUpload, "upload_failure"
	case strings.Contains(lower, "failed to get ping and jitter"),
		strings.Contains(lower, "failed to get ip info"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "connection reset by peer"),
		strings.Contains(lower, "connection timed out"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "tls handshake timeout"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "no route to host"),
		strings.Contains(lower, "host is down"),
		strings.Contains(lower, "not responding"):
		return model.FailureStageConnect, "connect_failure"
	default:
		return model.FailureStageHelper, "helper_failure"
	}
}

func sanitizeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(message))
	previousSpace := false
	for _, character := range message {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			if !previousSpace {
				builder.WriteByte(' ')
				previousSpace = true
			}
			continue
		}
		builder.WriteRune(character)
		previousSpace = false
		if builder.Len() >= 256 {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = buffer.buffer.Write(data)
	}
	return originalLength, nil
}

func (buffer *cappedBuffer) Bytes() []byte {
	return buffer.buffer.Bytes()
}

func (buffer *cappedBuffer) String() string {
	return buffer.buffer.String()
}
