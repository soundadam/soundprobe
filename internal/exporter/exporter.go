package exporter

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/soundadam/njuprobe/internal/model"
)

func Write(path, format string, summaries []model.RunSummary) error {
	if path == "" {
		return errors.New("output path is required")
	}
	if format != "jsonl" && format != "csv" {
		return fmt.Errorf("unsupported export format %q", format)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open export output: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("set export output mode: %w", err)
	}

	switch format {
	case "jsonl":
		encoder := json.NewEncoder(file)
		for _, summary := range summaries {
			if err := encoder.Encode(summary); err != nil {
				return fmt.Errorf("write JSONL export: %w", err)
			}
		}
	case "csv":
		writer := csv.NewWriter(file)
		header := []string{
			"run_id", "started_at", "ended_at", "command", "status", "label", "note",
			"campus_download_mbps", "campus_upload_mbps", "mlab_download_mbps", "mlab_upload_mbps",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("write CSV header: %w", err)
		}
		for _, summary := range summaries {
			campusDown, campusUp := measurementSpeeds(summary, model.ProviderCampus)
			mlabDown, mlabUp := measurementSpeeds(summary, model.ProviderMLab)
			row := []string{
				summary.RunID,
				summary.StartedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
				summary.EndedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
				string(summary.Command),
				string(summary.Status),
				stringValue(summary.Label),
				stringValue(summary.Note),
				floatValue(campusDown),
				floatValue(campusUp),
				floatValue(mlabDown),
				floatValue(mlabUp),
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("write CSV row: %w", err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("flush CSV export: %w", err)
		}
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync export output: %w", err)
	}
	return nil
}

func measurementSpeeds(summary model.RunSummary, provider model.Provider) (*float64, *float64) {
	for _, measurement := range summary.Measurements {
		if measurement.Provider == provider {
			return measurement.DownloadMbps, measurement.UploadMbps
		}
	}
	return nil, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatValue(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
