package ookla

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/provider"
)

const (
	HelperName     = "speedtest"
	DefaultTimeout = 90 * time.Second
)

var versionPattern = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9]+)?\b`)

type Runner struct {
	Path       string
	LookupPath func(string) (string, error)
	Timeout    time.Duration
	Now        func() time.Time
}

func New() *Runner { return &Runner{} }

func (runner *Runner) Preflight(ctx context.Context, _ provider.Request) error {
	runner.setDefaults()
	_, _, err := runner.resolve(ctx)
	return err
}

func (runner *Runner) Measure(ctx context.Context, request provider.Request) (model.Measurement, error) {
	runner.setDefaults()
	path, version, err := runner.resolve(ctx)
	if err != nil {
		return model.Measurement{}, err
	}

	measurementCtx, cancel := context.WithTimeout(ctx, runner.Timeout)
	defer cancel()
	startedAt := runner.Now()
	args := []string{"--format=json"}
	if request.Network != nil && request.Network.ActiveInterface != nil && strings.TrimSpace(*request.Network.ActiveInterface) != "" {
		args = append(args, "--interface="+strings.TrimSpace(*request.Network.ActiveInterface))
	}
	request.Report(provider.ProgressEvent{Provider: model.ProviderOokla, Phase: provider.ProgressMeasuring})

	command := exec.CommandContext(measurementCtx, path, args...)
	var stdout, stderr cappedBuffer
	stdout.Limit = 4 * 1024 * 1024
	stderr.Limit = 32 * 1024
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		durationMS := elapsedMS(startedAt, runner.Now())
		if errors.Is(measurementCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return cancelledMeasurement(version, durationMS), nil
		}
		if errors.Is(measurementCtx.Err(), context.DeadlineExceeded) {
			return failedMeasurement(version, durationMS, model.FailureStageTimeout, "timeout", "Ookla Speedtest timed out"), nil
		}
		if stdout.Len() > 0 {
			if measurement, parseErr := parseResult(stdout.Bytes(), version, durationMS); parseErr == nil {
				return measurement, nil
			}
		}
		message := compact(stderr.String())
		if message == "" {
			message = compact(stdout.String())
		}
		if message == "" {
			message = "Ookla Speedtest exited without a result"
		}
		code := "helper_exit"
		if strings.Contains(strings.ToLower(message), "license") || strings.Contains(strings.ToLower(message), "gdpr") {
			code = "license_required"
			message += "; run `speedtest --accept-license --accept-gdpr` once, then retry"
		}
		return failedMeasurement(version, durationMS, model.FailureStageHelper, code, message), nil
	}

	durationMS := elapsedMS(startedAt, runner.Now())
	measurement, parseErr := parseResult(stdout.Bytes(), version, durationMS)
	if parseErr != nil {
		message := parseErr.Error()
		if stderr.String() != "" {
			message = compact(stderr.String())
		}
		return failedMeasurement(version, durationMS, model.FailureStageHelper, "invalid_output", message), nil
	}
	return measurement, nil
}

func (runner *Runner) resolve(ctx context.Context) (string, string, error) {
	path := strings.TrimSpace(runner.Path)
	if path == "" {
		lookup := runner.LookupPath
		if lookup == nil {
			lookup = exec.LookPath
		}
		resolved, err := lookup(HelperName)
		if err != nil {
			return "", "", fmt.Errorf("%w: official Ookla CLI %q was not found; install it from https://www.speedtest.net/apps/cli", provider.ErrUnavailable, HelperName)
		}
		path = resolved
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("%w: Ookla CLI was not found at %s", provider.ErrUnavailable, path)
	}
	if err != nil {
		return "", "", fmt.Errorf("%w: inspect Ookla CLI: %v", provider.ErrUnavailable, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", "", fmt.Errorf("%w: Ookla CLI %s is not executable", provider.ErrUnavailable, path)
	}

	command := exec.CommandContext(ctx, path, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("%w: run Ookla CLI --version: %v", provider.ErrUnavailable, err)
	}
	text := strings.TrimSpace(string(output))
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "ookla") || !strings.Contains(lower, "speedtest") || strings.Contains(lower, "speedtest-cli") {
		return "", "", fmt.Errorf("%w: %s is not the official Ookla Speedtest CLI", provider.ErrUnavailable, path)
	}
	version := "unknown"
	if match := versionPattern.FindString(text); match != "" {
		version = match
	}
	return path, version, nil
}

func (runner *Runner) setDefaults() {
	if runner.Timeout <= 0 {
		runner.Timeout = DefaultTimeout
	}
	if runner.Now == nil {
		runner.Now = time.Now
	}
}

func elapsedMS(start, end time.Time) int64 {
	duration := end.Sub(start).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}

type cappedBuffer struct {
	bytes.Buffer
	Limit int
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	if buffer.Limit > 0 {
		remaining := buffer.Limit - buffer.Len()
		if remaining > 0 {
			if len(data) > remaining {
				_, _ = buffer.Buffer.Write(data[:remaining])
			} else {
				_, _ = buffer.Buffer.Write(data)
			}
		}
	}
	return len(data), nil
}
