package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/soundadam/soundprobe/internal/consent"
	"github.com/soundadam/soundprobe/internal/exporter"
	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/preferences"
	"github.com/soundadam/soundprobe/internal/provider"
	"github.com/soundadam/soundprobe/internal/storage"
	"github.com/soundadam/soundprobe/internal/target"
	"github.com/soundadam/soundprobe/internal/ui"
)

const usage = `soundprobe measures education-network paths first, with public references.

Usage:
  soundprobe
  soundprobe run [--targets LIST] [--family ipv4|ipv6|dual] [--label TEXT] [--note TEXT] [--no-save]
  soundprobe campus [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
  soundprobe edge [--ipv4|--ipv6] [--label TEXT] [--note TEXT] [--no-save]
  soundprobe domestic [--targets LIST] [--family ipv4|dual] [--label TEXT] [--note TEXT] [--no-save]
  soundprobe mlab [--label TEXT] [--note TEXT] [--no-save]
  soundprobe apple [--label TEXT] [--note TEXT] [--no-save]
  soundprobe ookla [--label TEXT] [--note TEXT] [--no-save]
  soundprobe stations [--json]
  soundprobe history [--limit N]
  soundprobe last [--json]
  soundprobe show RUN_ID [--json]
  soundprobe export --format jsonl|csv --output PATH
  soundprobe consent status|accept|revoke
  soundprobe setup
  soundprobe doctor [--json]
  soundprobe version
`

type progressRenderer interface {
	Update(provider.ProgressEvent)
	Close() error
}

type App struct {
	In                        io.Reader
	Out                       io.Writer
	Err                       io.Writer
	StdinTTY                  bool
	StdoutTTY                 bool
	Version                   string
	Runner                    provider.Runner
	History                   *storage.Store
	Consent                   *consent.Store
	Now                       func() time.Time
	ProgressFactory           func(io.Writer, string, []model.Provider) (progressRenderer, error)
	SelectorFactory           func(context.Context, io.Reader, io.Writer, string) (target.Plan, error)
	ConfiguredSelectorFactory func(context.Context, io.Reader, io.Writer, string, preferences.Config) (target.Plan, error)
	SetupFactory              func(context.Context, io.Reader, io.Writer, string, preferences.Config) (preferences.Config, error)
	Preferences               *preferences.Store
}

type commandOptions struct {
	label   string
	note    string
	targets string
	family  string
	noSave  bool
	ipv4    bool
	ipv6    bool
}

func (app *App) Execute(ctx context.Context, args []string) int {
	app.setDefaults()
	args, jsonMode := extractGlobalJSON(args)

	if len(args) == 0 {
		if app.StdinTTY && app.StdoutTTY && !jsonMode {
			config, err := app.loadOrConfigurePreferences(ctx)
			if err != nil {
				if errors.Is(err, ui.ErrSetupCancelled) {
					return 130
				}
				return app.fail(false, "preferences_error", err.Error(), 1)
			}
			var plan target.Plan
			if app.Preferences != nil {
				plan, err = app.ConfiguredSelectorFactory(ctx, app.In, app.Out, app.Version, config)
			} else {
				plan, err = app.SelectorFactory(ctx, app.In, app.Out, app.Version)
			}
			if err != nil {
				if errors.Is(err, ui.ErrSelectionCancelled) {
					return 130
				}
				return app.fail(false, "selector_error", err.Error(), 1)
			}
			return app.executeMeasurementPlan(ctx, model.CommandRun, commandOptions{}, plan, false)
		}
		return app.executeMeasurement(ctx, model.CommandRun, nil, jsonMode)
	}

	command := args[0]
	rest := args[1:]
	switch command {
	case "help", "--help", "-h":
		fmt.Fprint(app.Out, usage)
		return 0
	case "version":
		if len(rest) != 0 {
			return app.fail(jsonMode, "invalid_arguments", "version does not accept arguments", 1)
		}
		return app.writeValue(jsonMode, map[string]string{"version": app.Version}, "soundprobe "+app.Version)
	case "run":
		return app.executeMeasurement(ctx, model.CommandRun, rest, jsonMode)
	case "campus":
		return app.executeMeasurement(ctx, model.CommandCampus, rest, jsonMode)
	case "edge":
		return app.executeMeasurement(ctx, model.CommandEdge, rest, jsonMode)
	case "domestic":
		return app.executeMeasurement(ctx, model.CommandDomestic, rest, jsonMode)
	case "mlab":
		return app.executeMeasurement(ctx, model.CommandMLab, rest, jsonMode)
	case "apple":
		return app.executeMeasurement(ctx, model.CommandApple, rest, jsonMode)
	case "ookla":
		return app.executeMeasurement(ctx, model.CommandOokla, rest, jsonMode)
	case "stations":
		return app.executeStations(ctx, rest, jsonMode)
	case "history":
		return app.executeHistory(rest, jsonMode)
	case "last":
		return app.executeLast(rest, jsonMode)
	case "show":
		return app.executeShow(rest, jsonMode)
	case "export":
		return app.executeExport(rest, jsonMode)
	case "consent":
		return app.executeConsent(rest, jsonMode)
	case "setup":
		return app.executeSetup(ctx, rest, jsonMode)
	case "doctor":
		return app.executeDoctor(ctx, rest, jsonMode)
	default:
		return app.fail(jsonMode, "unknown_command", fmt.Sprintf("unknown command %q", command), 1)
	}
}

func (app *App) setDefaults() {
	if app.In == nil {
		app.In = strings.NewReader("")
	}
	if app.Out == nil {
		app.Out = io.Discard
	}
	if app.Err == nil {
		app.Err = io.Discard
	}
	if app.Version == "" {
		app.Version = "dev"
	}
	if app.Now == nil {
		app.Now = time.Now
	}
	if app.ProgressFactory == nil {
		app.ProgressFactory = func(output io.Writer, version string, providers []model.Provider) (progressRenderer, error) {
			return ui.NewProgressRenderer(output, version, providers)
		}
	}
	if app.SelectorFactory == nil {
		app.SelectorFactory = ui.SelectPlan
	}
	if app.ConfiguredSelectorFactory == nil {
		app.ConfiguredSelectorFactory = ui.SelectPlanConfigured
	}
	if app.SetupFactory == nil {
		app.SetupFactory = ui.Configure
	}
}

func (app *App) loadOrConfigurePreferences(ctx context.Context) (preferences.Config, error) {
	if app.Preferences == nil {
		return preferences.DefaultConfig(), nil
	}
	config, exists, err := app.Preferences.Load()
	if err != nil {
		return preferences.Config{}, err
	}
	if exists {
		return config, nil
	}
	config, err = app.SetupFactory(ctx, app.In, app.Out, app.Version, preferences.DefaultConfig())
	if err != nil {
		return preferences.Config{}, err
	}
	if err := app.Preferences.Save(config); err != nil {
		return preferences.Config{}, err
	}
	return config, nil
}

func (app *App) executeSetup(ctx context.Context, args []string, jsonMode bool) int {
	if len(args) != 0 {
		return app.fail(jsonMode, "invalid_arguments", "setup does not accept arguments", 1)
	}
	if jsonMode || !app.StdinTTY || !app.StdoutTTY {
		return app.fail(jsonMode, "setup_requires_interaction", "setup requires an interactive terminal", 1)
	}
	if app.Preferences == nil {
		return app.fail(false, "preferences_error", "preferences store is not configured", 1)
	}
	current, exists, err := app.Preferences.Load()
	if err != nil {
		return app.fail(false, "preferences_error", err.Error(), 1)
	}
	if !exists {
		current = preferences.DefaultConfig()
	}
	config, err := app.SetupFactory(ctx, app.In, app.Out, app.Version, current)
	if err != nil {
		if errors.Is(err, ui.ErrSetupCancelled) {
			return 130
		}
		return app.fail(false, "preferences_error", err.Error(), 1)
	}
	if err := app.Preferences.Save(config); err != nil {
		return app.fail(false, "preferences_error", err.Error(), 1)
	}
	return app.writeValue(false, config, app.preferenceSavedMessage(config))
}

func (app *App) preferenceSavedMessage(config preferences.Config) string {
	if config.Language == preferences.LanguageChinese {
		return "日常测速站设置已保存：" + strings.Join(config.DailyStations, ", ")
	}
	return "Daily stations saved: " + strings.Join(config.DailyStations, ", ")
}

func extractGlobalJSON(args []string) ([]string, bool) {
	filtered := make([]string, 0, len(args))
	jsonMode := false
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered, jsonMode
}

func (app *App) executeMeasurement(ctx context.Context, command model.Command, args []string, jsonMode bool) int {
	options, err := app.parseMeasurementFlags(command, args, jsonMode)
	if err != nil {
		return app.fail(jsonMode, "invalid_arguments", err.Error(), 1)
	}
	plan, err := resolvePlan(command, options)
	if err != nil {
		return app.fail(jsonMode, "invalid_arguments", err.Error(), 1)
	}
	return app.executeMeasurementPlan(ctx, command, options, plan, jsonMode)
}

func (app *App) executeMeasurementPlan(ctx context.Context, command model.Command, options commandOptions, plan target.Plan, jsonMode bool) int {
	if app.Runner == nil {
		return app.fail(jsonMode, "internal_error", "measurement runner is not configured", 1)
	}
	request := provider.Request{
		Command: command,
		Targets: append([]model.Provider(nil), plan.Providers...),
		Label:   optionalString(options.label),
		Note:    optionalString(options.note),
	}
	requestedProviders := append([]model.Provider(nil), request.Targets...)
	prepared := false
	if preparer, ok := app.Runner.(provider.RequestPreparer); ok {
		var err error
		request, err = preparer.Prepare(ctx, request)
		if err != nil {
			return app.fail(jsonMode, measurementErrorCode(err), err.Error(), 1)
		}
		prepared = true
		if !jsonMode {
			for _, requested := range requestedProviders {
				if containsProvider(request.Targets, requested) {
					continue
				}
				fmt.Fprintf(app.Err, "soundprobe: optional target %s is unavailable; continuing without it (see `soundprobe doctor --json`)\n", target.Label(requested))
			}
		}
		// Optional helpers may be removed during preparation. Keep consent,
		// progress, and the persisted target order aligned with the actual run.
		plan.Providers = append([]model.Provider(nil), request.Targets...)
		plan.StationIDs = target.StationIDs(request.Targets)
	}
	if !prepared {
		if preflight, ok := app.Runner.(provider.PreflightRunner); ok {
			if err := preflight.Preflight(ctx, request); err != nil {
				return app.fail(jsonMode, measurementErrorCode(err), err.Error(), 1)
			}
		}
	}
	if target.NeedsMLab(plan.Providers) {
		if exitCode := app.ensureMLabConsent(jsonMode); exitCode != 0 {
			return exitCode
		}
	}

	var progress progressRenderer
	var err error
	if app.StdoutTTY && !jsonMode {
		progress, err = app.ProgressFactory(app.Out, app.Version, plan.Providers)
		if err != nil {
			return app.fail(false, "renderer_error", fmt.Sprintf("start interactive renderer: %v", err), 1)
		}
		request.Progress = progress.Update
	}
	closeProgress := func() error {
		if progress == nil {
			return nil
		}
		err := progress.Close()
		progress = nil
		return err
	}

	summary, runErr := app.Runner.Run(ctx, request)
	if runErr != nil {
		_ = closeProgress()
		return app.fail(jsonMode, measurementErrorCode(runErr), runErr.Error(), 1)
	}
	if summary.SchemaVersion == 0 {
		summary.SchemaVersion = model.SchemaVersion
	}
	if summary.ToolVersion == "" {
		summary.ToolVersion = app.Version
	}
	if summary.Command == "" {
		summary.Command = command
	}
	if len(summary.Targets) == 0 {
		summary.Targets = append([]model.Provider(nil), plan.Providers...)
	}
	if summary.Status == "" {
		summary.Status = model.DeriveRunStatus(summary.Measurements)
	}

	if !options.noSave {
		if app.History == nil {
			_ = closeProgress()
			return app.fail(jsonMode, "storage_error", "history store is not configured", 1)
		}
		if err := app.History.Save(summary); err != nil {
			_ = closeProgress()
			return app.fail(jsonMode, "storage_error", err.Error(), 1)
		}
	}
	if err := closeProgress(); err != nil {
		return app.fail(jsonMode, "renderer_error", err.Error(), 1)
	}
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(summary); err != nil {
			return 1
		}
	} else {
		app.renderSummary(summary)
	}
	return summary.ExitCode()
}

func (app *App) parseMeasurementFlags(command model.Command, args []string, jsonMode bool) (commandOptions, error) {
	options := commandOptions{family: string(target.FamilyIPv4)}
	flags := flag.NewFlagSet(string(command), flag.ContinueOnError)
	if jsonMode {
		flags.SetOutput(io.Discard)
	} else {
		flags.SetOutput(app.Err)
	}
	flags.StringVar(&options.label, "label", "", "optional run label")
	flags.StringVar(&options.note, "note", "", "optional run note")
	flags.BoolVar(&options.noSave, "no-save", false, "do not persist the result")
	if command == model.CommandRun || command == model.CommandDomestic {
		flags.StringVar(&options.targets, "targets", "", "comma-separated target IDs")
		flags.StringVar(&options.family, "family", string(target.FamilyIPv4), "ipv4, ipv6, or dual")
	}
	if command == model.CommandCampus || command == model.CommandEdge {
		flags.BoolVar(&options.ipv4, "ipv4", false, "use the IPv4 service")
		flags.BoolVar(&options.ipv6, "ipv6", false, "use the IPv6 service")
	}
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, err
	}
	if flags.NArg() != 0 {
		return commandOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if options.ipv4 && options.ipv6 {
		return commandOptions{}, errors.New("--ipv4 and --ipv6 are mutually exclusive")
	}
	if options.ipv6 {
		options.family = string(target.FamilyIPv6)
	} else if options.ipv4 {
		options.family = string(target.FamilyIPv4)
	}
	family := target.Family(options.family)
	if family != target.FamilyIPv4 && family != target.FamilyIPv6 && family != target.FamilyDual {
		return commandOptions{}, fmt.Errorf("--family must be ipv4, ipv6, or dual")
	}
	return options, nil
}

func resolvePlan(command model.Command, options commandOptions) (target.Plan, error) {
	family := target.Family(options.family)
	ids := []string{}
	switch command {
	case model.CommandRun:
		ids = []string{"nju-campus", "mlab", "apple"}
	case model.CommandCampus:
		ids = []string{"nju-campus"}
	case model.CommandEdge:
		ids = []string{"nju-edge"}
	case model.CommandDomestic:
		ids = []string{"tongji", "qlu"}
	case model.CommandMLab:
		ids = []string{"mlab"}
	case model.CommandApple:
		ids = []string{"apple"}
	case model.CommandOokla:
		ids = []string{"ookla"}
	default:
		return target.Plan{}, fmt.Errorf("unsupported measurement command %q", command)
	}
	if strings.TrimSpace(options.targets) != "" {
		ids = splitCommaList(options.targets)
	}
	if command == model.CommandDomestic {
		if family == target.FamilyIPv6 {
			return target.Plan{}, errors.New("domestic stations currently support IPv4 only")
		}
		allowed := map[string]struct{}{"cernet": {}, "qlu": {}, "tongji": {}}
		for _, id := range ids {
			if _, ok := allowed[id]; !ok {
				return target.Plan{}, fmt.Errorf("target %q is not a domestic station", id)
			}
		}
	}
	return target.NewPlan(ids, family)
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (app *App) executeStations(ctx context.Context, args []string, jsonMode bool) int {
	if len(args) != 0 {
		return app.fail(jsonMode, "invalid_arguments", "stations does not accept arguments", 1)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	results := target.ProbeAll(probeCtx, 1500*time.Millisecond)
	cancel()
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(results); err != nil {
			return 1
		}
		return 0
	}
	writer := tabwriter.NewWriter(app.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "STATION\tFAMILY\tSTATUS\tLATENCY\tDETAIL")
	for _, result := range results {
		latency := "—"
		if result.LatencyMS != nil {
			latency = fmt.Sprintf("%.0f ms", *result.LatencyMS)
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			result.StationID, result.Family, result.Status, latency, result.Message)
	}
	_ = writer.Flush()
	return 0
}

func (app *App) executeHistory(args []string, jsonMode bool) int {
	flags := flag.NewFlagSet("history", flag.ContinueOnError)
	if jsonMode {
		flags.SetOutput(io.Discard)
	} else {
		flags.SetOutput(app.Err)
	}
	limit := flags.Int("limit", 20, "maximum number of runs; zero means all")
	if err := flags.Parse(args); err != nil {
		return app.fail(jsonMode, "invalid_arguments", err.Error(), 1)
	}
	if flags.NArg() != 0 {
		return app.fail(jsonMode, "invalid_arguments", "history does not accept positional arguments", 1)
	}
	if *limit < 0 {
		return app.fail(jsonMode, "invalid_arguments", "history limit must be non-negative", 1)
	}
	if app.History == nil {
		return app.fail(jsonMode, "storage_error", "history store is not configured", 1)
	}
	summaries, err := app.History.List(*limit)
	if err != nil {
		return app.fail(jsonMode, "storage_error", err.Error(), 1)
	}
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(summaries); err != nil {
			return 1
		}
		return 0
	}
	app.renderHistory(summaries)
	return 0
}

func (app *App) executeLast(args []string, jsonMode bool) int {
	if len(args) != 0 {
		return app.fail(jsonMode, "invalid_arguments", "last does not accept arguments", 1)
	}
	if app.History == nil {
		return app.fail(jsonMode, "storage_error", "history store is not configured", 1)
	}
	summaries, err := app.History.List(1)
	if err != nil {
		return app.fail(jsonMode, "storage_error", err.Error(), 1)
	}
	if len(summaries) == 0 {
		return app.fail(jsonMode, "no_history", "no saved runs", 1)
	}
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(summaries[0]); err != nil {
			return 1
		}
	} else {
		app.renderSummary(summaries[0])
	}
	return 0
}

func (app *App) executeShow(args []string, jsonMode bool) int {
	if len(args) != 1 {
		return app.fail(jsonMode, "invalid_arguments", "show requires exactly one RUN_ID", 1)
	}
	if app.History == nil {
		return app.fail(jsonMode, "storage_error", "history store is not configured", 1)
	}
	summary, err := app.History.Load(args[0])
	if err != nil {
		return app.fail(jsonMode, "storage_error", err.Error(), 1)
	}
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(summary); err != nil {
			return 1
		}
	} else {
		app.renderSummary(summary)
	}
	return 0
}

func (app *App) executeExport(args []string, jsonMode bool) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	if jsonMode {
		flags.SetOutput(io.Discard)
	} else {
		flags.SetOutput(app.Err)
	}
	format := flags.String("format", "", "jsonl or csv")
	output := flags.String("output", "", "output path")
	if err := flags.Parse(args); err != nil {
		return app.fail(jsonMode, "invalid_arguments", err.Error(), 1)
	}
	if flags.NArg() != 0 || (*format != "jsonl" && *format != "csv") || *output == "" {
		return app.fail(jsonMode, "invalid_arguments", "export requires --format jsonl|csv and --output PATH", 1)
	}
	if app.History == nil {
		return app.fail(jsonMode, "storage_error", "history store is not configured", 1)
	}
	summaries, err := app.History.List(0)
	if err != nil {
		return app.fail(jsonMode, "storage_error", err.Error(), 1)
	}
	if err := exporter.Write(*output, *format, summaries); err != nil {
		return app.fail(jsonMode, "export_error", err.Error(), 1)
	}
	return app.writeValue(jsonMode, map[string]any{
		"format": *format,
		"output": *output,
		"runs":   len(summaries),
	}, fmt.Sprintf("Exported %d runs to %s", len(summaries), *output))
}

func (app *App) executeDoctor(ctx context.Context, args []string, jsonMode bool) int {
	if len(args) != 0 {
		return app.fail(jsonMode, "invalid_arguments", "doctor does not accept arguments", 1)
	}
	preflight, ok := app.Runner.(provider.PreflightRunner)
	if !ok {
		return app.fail(jsonMode, "internal_error", "measurement runner does not support diagnostics", 1)
	}

	checks := map[string]string{}
	optionalChecks := map[string]string{}
	ready := true
	for _, check := range []struct {
		name     string
		command  model.Command
		provider model.Provider
	}{
		{name: "campus", command: model.CommandCampus, provider: model.ProviderCampus},
		{name: "mlab", command: model.CommandMLab, provider: model.ProviderMLab},
	} {
		err := preflight.Preflight(ctx, provider.Request{Command: check.command, Targets: []model.Provider{check.provider}})
		if err != nil {
			checks[check.name] = err.Error()
			ready = false
		} else {
			checks[check.name] = "ready"
		}
	}
	for _, check := range []struct {
		name     string
		command  model.Command
		provider model.Provider
	}{
		{name: "apple", command: model.CommandApple, provider: model.ProviderApple},
		{name: "ookla", command: model.CommandOokla, provider: model.ProviderOokla},
	} {
		err := preflight.Preflight(ctx, provider.Request{Command: check.command, Targets: []model.Provider{check.provider}})
		if err != nil {
			optionalChecks[check.name] = err.Error()
		} else {
			optionalChecks[check.name] = "ready"
		}
	}

	consentAccepted := false
	if app.Consent != nil {
		_, consentAccepted, _ = app.Consent.Status()
	}
	historyPath := ""
	if app.History != nil {
		historyPath = app.History.HistoryDir
	}
	combinedReady := ready && consentAccepted
	payload := map[string]any{
		"version":           app.Version,
		"ready":             ready,
		"combinedReady":     combinedReady,
		"providers":         checks,
		"optionalProviders": optionalChecks,
		"consentAccepted":   consentAccepted,
		"historyPath":       historyPath,
	}
	if app.Preferences != nil {
		config, exists, preferencesErr := app.Preferences.Load()
		payload["preferencesPath"] = app.Preferences.Path
		payload["setupComplete"] = exists && preferencesErr == nil
		if preferencesErr == nil && exists {
			payload["language"] = config.Language
			payload["dailyStations"] = config.DailyStations
		}
	}
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(payload); err != nil {
			return 1
		}
	} else {
		fmt.Fprintf(app.Out, "soundprobe %s diagnostics\n", app.Version)
		fmt.Fprintf(app.Out, "Campus   %s\n", checks["campus"])
		fmt.Fprintf(app.Out, "M-Lab    %s\n", checks["mlab"])
		fmt.Fprintf(app.Out, "Apple    %s\n", optionalChecks["apple"])
		fmt.Fprintf(app.Out, "Ookla    %s\n", optionalChecks["ookla"])
		fmt.Fprintf(app.Out, "Consent  %t\n", consentAccepted)
		fmt.Fprintf(app.Out, "Combined %t\n", combinedReady)
		fmt.Fprintf(app.Out, "History  %s\n", historyPath)
	}
	if !ready {
		return 1
	}
	return 0
}

func (app *App) executeConsent(args []string, jsonMode bool) int {
	if len(args) != 1 {
		return app.fail(jsonMode, "invalid_arguments", "consent requires status, accept, or revoke", 1)
	}
	if app.Consent == nil {
		return app.fail(jsonMode, "consent_error", "consent store is not configured", 1)
	}
	switch args[0] {
	case "status":
		record, accepted, err := app.Consent.Status()
		if err != nil {
			return app.fail(jsonMode, "consent_error", err.Error(), 1)
		}
		if jsonMode {
			payload := map[string]any{
				"accepted":      accepted,
				"policyVersion": consent.PolicyVersion,
				"policyUrl":     consent.PolicyURL,
			}
			if !record.AcceptedAt.IsZero() {
				payload["record"] = record
			}
			return app.writeValue(true, payload, "")
		}
		if accepted {
			fmt.Fprintf(app.Out, "M-Lab consent accepted (%s at %s).\n", record.PolicyVersion, record.AcceptedAt.Format(time.RFC3339))
		} else {
			fmt.Fprintf(app.Out, "M-Lab consent is not accepted for current policy %s.\n", consent.PolicyVersion)
		}
		fmt.Fprintf(app.Out, "Policy: %s\n", consent.PolicyURL)
		return 0
	case "accept":
		if jsonMode {
			return app.fail(true, "consent_requires_interaction", "consent accept is interactive and unavailable in JSON mode", 1)
		}
		return app.promptAndAcceptConsent(false)
	case "revoke":
		if err := app.Consent.Revoke(); err != nil {
			return app.fail(jsonMode, "consent_error", err.Error(), 1)
		}
		return app.writeValue(jsonMode, map[string]bool{"revoked": true}, "M-Lab consent revoked.")
	default:
		return app.fail(jsonMode, "invalid_arguments", "consent requires status, accept, or revoke", 1)
	}
}

func (app *App) ensureMLabConsent(jsonMode bool) int {
	if app.Consent == nil {
		return app.fail(jsonMode, "consent_error", "consent store is not configured", 1)
	}
	_, accepted, err := app.Consent.Status()
	if err != nil {
		return app.fail(jsonMode, "consent_error", err.Error(), 1)
	}
	if accepted {
		return 0
	}
	if jsonMode || !app.StdinTTY {
		return app.fail(jsonMode, "consent_required", "M-Lab consent is required; run `soundprobe consent accept` interactively", 1)
	}
	return app.promptAndAcceptConsent(jsonMode)
}

func (app *App) promptAndAcceptConsent(jsonMode bool) int {
	if !app.StdinTTY {
		return app.fail(jsonMode, "consent_requires_interaction", "consent acceptance requires an interactive terminal", 1)
	}
	fmt.Fprintf(app.Out, "M-Lab collects the ISP-provided public IP address and measurement results.\n")
	fmt.Fprintf(app.Out, "M-Lab publishes and retains experiment data indefinitely.\n")
	fmt.Fprintf(app.Out, "Policy %s: %s\n", consent.PolicyVersion, consent.PolicyURL)
	fmt.Fprint(app.Out, "Type accept to continue: ")
	scanner := bufio.NewScanner(app.In)
	if !scanner.Scan() {
		return app.fail(jsonMode, "consent_declined", "consent was not accepted", 1)
	}
	if strings.TrimSpace(scanner.Text()) != "accept" {
		return app.fail(jsonMode, "consent_declined", "consent was not accepted", 1)
	}
	record, err := app.Consent.Accept(app.Version, app.Now())
	if err != nil {
		return app.fail(jsonMode, "consent_error", err.Error(), 1)
	}
	fmt.Fprintf(app.Out, "M-Lab consent recorded for policy %s.\n", record.PolicyVersion)
	return 0
}

func (app *App) renderSummary(summary model.RunSummary) {
	fmt.Fprintf(app.Out, "soundprobe %s · %s · %s\n",
		summary.ToolVersion,
		summary.Status,
		formatDuration(summary.EndedAt.Sub(summary.StartedAt)),
	)
	fmt.Fprintf(app.Out, "Run %s\n", summary.RunID)
	if network := formatNetworkContext(summary.Network); network != "" {
		fmt.Fprintf(app.Out, "Network %s\n", network)
	}
	writer := tabwriter.NewWriter(app.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "TARGET\tMETHOD\tDOWNLOAD\tUPLOAD\tSERVER\tSTATUS")
	for _, measurement := range summary.Measurements {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			target.Label(measurement.Provider),
			measurement.Method,
			formatMbps(measurement.DownloadMbps),
			formatMbps(measurement.UploadMbps),
			measurementServer(measurement),
			measurement.Status,
		)
	}
	_ = writer.Flush()
	for _, measurement := range summary.Measurements {
		if measurement.Failure != nil {
			fmt.Fprintf(app.Out, "%s error [%s/%s]: %s\n",
				measurement.Provider,
				measurement.Failure.Stage,
				measurement.Failure.Code,
				measurement.Failure.Message,
			)
		}
	}
}

func (app *App) renderHistory(summaries []model.RunSummary) {
	if len(summaries) == 0 {
		fmt.Fprintln(app.Out, "No saved runs.")
		return
	}
	writer := tabwriter.NewWriter(app.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "RUN ID\tSTARTED\tCOMMAND\tSTATUS\tLABEL")
	for _, summary := range summaries {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			summary.RunID,
			summary.StartedAt.Local().Format("2006-01-02 15:04:05"),
			summary.Command,
			summary.Status,
			valueOrEmpty(summary.Label),
		)
	}
	_ = writer.Flush()
}

func (app *App) fail(jsonMode bool, code, message string, exitCode int) int {
	if jsonMode {
		_ = json.NewEncoder(app.Out).Encode(map[string]any{
			"error": map[string]string{"code": code, "message": message},
		})
	} else {
		fmt.Fprintf(app.Err, "soundprobe: %s\n", message)
	}
	return exitCode
}

func (app *App) writeValue(jsonMode bool, value any, plain string) int {
	if jsonMode {
		if err := json.NewEncoder(app.Out).Encode(value); err != nil {
			return 1
		}
		return 0
	}
	if plain != "" {
		fmt.Fprintln(app.Out, plain)
	}
	return 0
}

func measurementErrorCode(err error) string {
	if errors.Is(err, provider.ErrUnavailable) {
		return "measurement_unavailable"
	}
	return "measurement_error"
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func containsProvider(providers []model.Provider, wanted model.Provider) bool {
	for _, provider := range providers {
		if provider == wanted {
			return true
		}
	}
	return false
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatMbps(value *float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f Mbps", *value)
}

func formatNetworkContext(network model.NetworkContext) string {
	parts := make([]string, 0, 3)
	if network.ActiveInterface != nil && *network.ActiveInterface != "" {
		parts = append(parts, *network.ActiveInterface)
	}
	if network.InterfaceKind != nil && *network.InterfaceKind != "" {
		parts = append(parts, *network.InterfaceKind)
	}
	if network.SSID != nil && *network.SSID != "" {
		parts = append(parts, *network.SSID)
	}
	return strings.Join(parts, " · ")
}

func measurementServer(measurement model.Measurement) string {
	sponsor := valueOrEmpty(measurement.ServerSponsor)
	if measurement.ServerFQDN != nil && *measurement.ServerFQDN != "" {
		if sponsor != "" {
			return *measurement.ServerFQDN + " · " + sponsor
		}
		return *measurement.ServerFQDN
	}
	if measurement.ServerName != nil && *measurement.ServerName != "" {
		if sponsor != "" && *measurement.ServerName != sponsor {
			return *measurement.ServerName + " · " + sponsor
		}
		return *measurement.ServerName
	}
	if measurement.ServerAddress != nil && *measurement.ServerAddress != "" {
		return *measurement.ServerAddress
	}
	return "—"
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration < time.Second {
		return fmt.Sprintf("%d ms", duration.Milliseconds())
	}
	return duration.Round(100 * time.Millisecond).String()
}
