package model

import "testing"

func TestDeriveRunStatus(t *testing.T) {
	tests := []struct {
		name         string
		measurements []Measurement
		want         RunStatus
	}{
		{name: "no measurements", want: RunStatusFailed},
		{name: "all success", measurements: []Measurement{{Status: ProviderStatusSuccess}, {Status: ProviderStatusSuccess}}, want: RunStatusSuccess},
		{name: "partial", measurements: []Measurement{{Status: ProviderStatusSuccess}, {Status: ProviderStatusFailed}}, want: RunStatusPartial},
		{name: "all failed", measurements: []Measurement{{Status: ProviderStatusFailed}, {Status: ProviderStatusFailed}}, want: RunStatusFailed},
		{name: "cancelled wins", measurements: []Measurement{{Status: ProviderStatusSuccess}, {Status: ProviderStatusCancelled}}, want: RunStatusCancelled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeriveRunStatus(test.measurements); got != test.want {
				t.Fatalf("DeriveRunStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSkippedMeasurementRequiresNullSpeeds(t *testing.T) {
	now := testTime()
	summary := RunSummary{
		SchemaVersion: SchemaVersion,
		RunID:         "run-1",
		ToolVersion:   "dev",
		StartedAt:     now,
		EndedAt:       now,
		Command:       CommandMLab,
		Status:        RunStatusFailed,
		Measurements: []Measurement{{
			Provider:     ProviderMLab,
			Method:       "ndt7-single-stream",
			Status:       ProviderStatusSkipped,
			DownloadMbps: Pointer(0.0),
		}},
	}
	if err := summary.Validate(); err == nil {
		t.Fatal("Validate() succeeded for skipped measurement with a speed")
	}
}

func TestRunStatusMustMatchMeasurements(t *testing.T) {
	now := testTime()
	summary := RunSummary{
		SchemaVersion: SchemaVersion,
		RunID:         "run-1",
		ToolVersion:   "dev",
		StartedAt:     now,
		EndedAt:       now,
		Command:       CommandCampus,
		Status:        RunStatusPartial,
		Measurements: []Measurement{{
			Provider:     ProviderCampus,
			Method:       "librespeed-three-stream",
			Status:       ProviderStatusSuccess,
			DownloadMbps: Pointer(100.0),
			UploadMbps:   Pointer(50.0),
		}},
	}
	if err := summary.Validate(); err == nil {
		t.Fatal("Validate() succeeded for inconsistent run status")
	}
}

func TestMeasurementMetadataValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Measurement)
	}{
		{name: "negative bytes", mutate: func(measurement *Measurement) { measurement.DownloadBytes = Pointer(int64(-1)) }},
		{name: "negative duration", mutate: func(measurement *Measurement) { measurement.DurationMS = Pointer(int64(-1)) }},
		{name: "zero concurrency", mutate: func(measurement *Measurement) { measurement.Concurrency = Pointer(0) }},
		{name: "invalid family", mutate: func(measurement *Measurement) { measurement.IPFamily = Pointer("ipv5") }},
		{name: "empty helper version", mutate: func(measurement *Measurement) { measurement.HelperVersion = Pointer("") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := testTime()
			measurement := Measurement{
				Provider:     ProviderCampus,
				Method:       MethodLibreSpeedThreeStream,
				Status:       ProviderStatusSuccess,
				DownloadMbps: Pointer(100.0),
				UploadMbps:   Pointer(50.0),
			}
			test.mutate(&measurement)
			summary := RunSummary{
				SchemaVersion: SchemaVersion,
				RunID:         "run-1",
				ToolVersion:   "dev",
				StartedAt:     now,
				EndedAt:       now,
				Command:       CommandCampus,
				Status:        RunStatusSuccess,
				Measurements:  []Measurement{measurement},
			}
			if err := summary.Validate(); err == nil {
				t.Fatal("Validate() succeeded for invalid metadata")
			}
		})
	}
}

func TestFailedMeasurementRequiresZeroOrMeasuredSpeeds(t *testing.T) {
	now := testTime()
	summary := RunSummary{
		SchemaVersion: SchemaVersion,
		RunID:         "run-1",
		ToolVersion:   "dev",
		StartedAt:     now,
		EndedAt:       now,
		Command:       CommandCampus,
		Status:        RunStatusFailed,
		Measurements: []Measurement{{
			Provider: ProviderCampus,
			Method:   "librespeed-three-stream",
			Status:   ProviderStatusFailed,
			Failure: &Failure{
				Stage:   FailureStageConnect,
				Code:    "unreachable",
				Message: "campus endpoint was unreachable",
			},
		}},
	}
	if err := summary.Validate(); err == nil {
		t.Fatal("Validate() succeeded for failed measurement with null speeds")
	}
}
