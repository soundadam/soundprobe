package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/soundadam/soundprobe/internal/model"
	"github.com/soundadam/soundprobe/internal/provider"
	"github.com/soundadam/soundprobe/internal/target"
)

const (
	refreshInterval = 250 * time.Millisecond
	// Bubble Tea flushes views on a renderer loop independent of Update.
	// Keep the terminal state visible for several default renderer frames
	// before replacing it with the blank frame used to clear the inline block.
	finalFrameDelay = 50 * time.Millisecond
)

const (
	activityWidth   = 24
	activityPulse   = 5
	detailRuneLimit = 58
)

type ProgressRenderer struct {
	program *tea.Program
	ready   chan struct{}
	done    chan struct{}
	stop    sync.Once
	errMu   sync.Mutex
	runErr  error
}

func NewProgressRenderer(output io.Writer, version string, targets []model.Provider) (*ProgressRenderer, error) {
	return newProgressRenderer(output, version, targets)
}

func newProgressRenderer(output io.Writer, version string, targets []model.Provider, options ...tea.ProgramOption) (*ProgressRenderer, error) {
	ready := make(chan struct{})
	progressModel := newProgressModel(version, targets, ready)
	programOptions := []tea.ProgramOption{
		tea.WithInput(nil),
		tea.WithOutput(output),
	}
	programOptions = append(programOptions, options...)
	program := tea.NewProgram(progressModel, programOptions...)
	renderer := &ProgressRenderer{
		program: program,
		ready:   ready,
		done:    make(chan struct{}),
	}
	go func() {
		_, err := program.Run()
		renderer.errMu.Lock()
		renderer.runErr = err
		renderer.errMu.Unlock()
		close(renderer.done)
	}()
	select {
	case <-ready:
		return renderer, nil
	case <-renderer.done:
		err := renderer.err()
		if err == nil {
			err = fmt.Errorf("progress renderer exited before initialization")
		}
		return nil, err
	}
}

func (renderer *ProgressRenderer) Update(event provider.ProgressEvent) {
	if renderer != nil && renderer.program != nil {
		renderer.program.Send(progressMessage(event))
	}
}

func (renderer *ProgressRenderer) Close() error {
	if renderer == nil || renderer.program == nil {
		return nil
	}
	renderer.stop.Do(func() {
		renderer.program.Send(stopMessage{})
	})
	<-renderer.done
	return renderer.err()
}

func (renderer *ProgressRenderer) err() error {
	renderer.errMu.Lock()
	defer renderer.errMu.Unlock()
	return renderer.runErr
}

type providerState struct {
	phase        provider.ProgressPhase
	test         string
	server       string
	downloadMbps *float64
	uploadMbps   *float64
	message      string
	startedAt    time.Time
	endedAt      time.Time
}

type progressModel struct {
	version   string
	startedAt time.Time
	now       time.Time
	network   model.NetworkContext
	providers map[model.Provider]providerState
	order     []model.Provider
	ready     chan struct{}
	blank     bool
	chrome    chrome
}

type progressMessage provider.ProgressEvent
type tickMessage time.Time
type stopMessage struct{}
type clearMessage struct{}

func newProgressModel(version string, targets []model.Provider, ready chan struct{}) *progressModel {
	order := append([]model.Provider(nil), targets...)
	if len(order) == 0 {
		order = []model.Provider{model.ProviderNJUCampusIPv4, model.ProviderMLab, model.ProviderApple}
	}
	states := make(map[model.Provider]providerState, len(order))
	for _, name := range order {
		states[name] = providerState{phase: provider.ProgressWaiting}
	}
	now := time.Now()
	return &progressModel{
		version:   version,
		startedAt: now,
		now:       now,
		providers: states,
		order:     order,
		ready:     ready,
		chrome:    newChrome(),
	}
}

func (progress *progressModel) Init() tea.Cmd {
	close(progress.ready)
	return tea.Batch(tea.RequestBackgroundColor, tick())
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(now time.Time) tea.Msg {
		return tickMessage(now)
	})
}

func (progress *progressModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg, tea.BackgroundColorMsg:
		progress.chrome = progress.chrome.update(message)
		return progress, nil
	case progressMessage:
		progress.applyEvent(provider.ProgressEvent(message))
		return progress, nil
	case tickMessage:
		progress.now = time.Time(message)
		return progress, tick()
	case stopMessage:
		return progress, tea.Tick(finalFrameDelay, func(time.Time) tea.Msg {
			return clearMessage{}
		})
	case clearMessage:
		progress.blank = true
		return progress, tea.Quit
	}
	return progress, nil
}

func (progress *progressModel) applyEvent(event provider.ProgressEvent) {
	if event.Network != nil {
		progress.network = *event.Network
		return
	}
	state := progress.providers[event.Provider]
	if state.startedAt.IsZero() && event.Phase != provider.ProgressWaiting {
		state.startedAt = progress.now
	}
	state.phase = event.Phase
	if event.Test != "" {
		state.test = event.Test
	}
	if event.Server != "" {
		state.server = event.Server
	}
	if event.LiveMbps != nil {
		switch event.Test {
		case "download":
			state.downloadMbps = cloneFloat(event.LiveMbps)
		case "upload":
			state.uploadMbps = cloneFloat(event.LiveMbps)
		}
	}
	if event.DownloadMbps != nil {
		state.downloadMbps = cloneFloat(event.DownloadMbps)
	}
	if event.UploadMbps != nil {
		state.uploadMbps = cloneFloat(event.UploadMbps)
	}
	if event.Message != "" {
		state.message = event.Message
	} else if event.Phase == provider.ProgressComplete {
		state.message = ""
	}
	if isTerminalPhase(event.Phase) && state.endedAt.IsZero() {
		state.endedAt = progress.now
	}
	progress.providers[event.Provider] = state
}

func (progress *progressModel) View() tea.View {
	if progress.blank {
		return tea.NewView("")
	}
	p := progress.chrome.palette
	body := make([]string, 0, 2+4*len(progress.order))
	for _, name := range progress.order {
		body = append(body, progress.renderProvider(name, progress.providers[name])...)
	}
	body = append(body, progress.chrome.indent(p.Faint(formatElapsed(progress.now.Sub(progress.startedAt)))))
	return tea.NewView(progress.chrome.render(screen{
		title:       "soundprobe",
		version:     progress.version,
		description: renderNetwork(progress.network),
		hint:        renderOrder(progress.order),
		body:        body,
		help:        []string{progress.chrome.helpKeys("ctrl+c")},
	}))
}

func (progress *progressModel) providerLabel(text string, phase provider.ProgressPhase) string {
	switch {
	case isActivePhase(phase):
		return progress.chrome.palette.Accent(text)
	case phase == provider.ProgressWaiting:
		return progress.chrome.palette.Faint(text)
	default:
		return text
	}
}

func (progress *progressModel) renderProvider(name model.Provider, state providerState) []string {
	p := progress.chrome.palette
	label := progress.providerLabel(target.Label(name), state.phase)
	phase := phaseLabel(state.phase, state.test)
	marker := colorPhase(p, state.phase, phaseMarker(state.phase))
	status := label + "  " + marker + " " + colorPhase(p, state.phase, phase)
	if elapsed, ok := providerElapsed(state, progress.now); ok {
		status += p.Faint(" · " + formatElapsed(elapsed))
	}
	bar := renderActivity(state.phase, progress.now, progress.chrome.activityWidth())
	rates := fmt.Sprintf("↓ %s · ↑ %s", formatMbps(state.downloadMbps), formatMbps(state.uploadMbps))
	detail := truncateRunes(providerDetail(state), progress.chrome.detailLimit())
	return []string{
		progress.chrome.indent(status),
		progress.chrome.indent(colorActivity(p, state.phase, bar)),
		progress.chrome.indent(p.Faint(rates)),
		progress.chrome.indent(colorDetail(p, state, detail)),
	}
}

func renderOrder(providers []model.Provider) string {
	labels := make([]string, 0, len(providers))
	for _, provider := range providers {
		labels = append(labels, target.Label(provider))
	}
	return strings.Join(labels, " → ")
}

func renderNetwork(network model.NetworkContext) string {
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
	if len(parts) == 0 {
		return "detecting"
	}
	return strings.Join(parts, " · ")
}

func phaseMarker(phase provider.ProgressPhase) string {
	switch phase {
	case provider.ProgressComplete:
		return "✓"
	case provider.ProgressFailed:
		return "×"
	case provider.ProgressCancelled:
		return "■"
	case provider.ProgressWaiting:
		return "○"
	default:
		return "◐"
	}
}

func colorPhase(p Palette, phase provider.ProgressPhase, text string) string {
	switch phase {
	case provider.ProgressComplete:
		return p.OK(text)
	case provider.ProgressFailed:
		return p.Bad(text)
	case provider.ProgressCancelled:
		return p.Faint(text)
	case provider.ProgressWaiting:
		return p.Faint(text)
	default:
		return p.Knob(text)
	}
}

func colorActivity(p Palette, phase provider.ProgressPhase, bar string) string {
	switch phase {
	case provider.ProgressComplete:
		return p.OK(bar)
	case provider.ProgressFailed:
		return p.Bad(bar)
	case provider.ProgressCancelled, provider.ProgressWaiting:
		return p.Faint(bar)
	default:
		return p.Knob(bar)
	}
}

func colorDetail(p Palette, state providerState, detail string) string {
	if state.phase == provider.ProgressFailed {
		return p.Bad(detail)
	}
	return p.Faint(detail)
}

func isActivePhase(phase provider.ProgressPhase) bool {
	return !isTerminalPhase(phase) && phase != provider.ProgressWaiting
}

func phaseLabel(phase provider.ProgressPhase, test string) string {
	switch phase {
	case provider.ProgressWaiting:
		return "waiting"
	case provider.ProgressStarting:
		return "starting"
	case provider.ProgressConnecting:
		return "connecting"
	case provider.ProgressMeasuring:
		return "measuring"
	case provider.ProgressDownloading:
		return "downloading"
	case provider.ProgressUploading:
		return "uploading"
	case provider.ProgressComplete:
		return "complete"
	case provider.ProgressFailed:
		if test != "" {
			return "failed · " + test
		}
		return "failed"
	case provider.ProgressCancelled:
		return "cancelled"
	default:
		return string(phase)
	}
}

func renderActivity(phase provider.ProgressPhase, now time.Time, width int) string {
	if width <= 0 {
		width = activityWidth
	}
	if phase == provider.ProgressComplete {
		return "[" + strings.Repeat("█", width) + "]"
	}
	if phase == provider.ProgressWaiting {
		return "[" + strings.Repeat("░", width) + "]"
	}
	if phase == provider.ProgressFailed || phase == provider.ProgressCancelled {
		return "[" + strings.Repeat("─", width) + "]"
	}
	position := int(now.UnixNano()/int64(refreshInterval)) % (width + activityPulse)
	var builder strings.Builder
	builder.WriteByte('[')
	for index := 0; index < width; index++ {
		distance := position - index
		if distance >= 0 && distance < activityPulse {
			builder.WriteRune('█')
		} else {
			builder.WriteRune('░')
		}
	}
	builder.WriteByte(']')
	return builder.String()
}

func providerDetail(state providerState) string {
	if state.message != "" && (state.phase == provider.ProgressFailed || state.phase == provider.ProgressCancelled) {
		return "error: " + state.message
	}
	if state.server != "" {
		return "server " + state.server
	}
	switch state.phase {
	case provider.ProgressWaiting:
		return "not started"
	case provider.ProgressStarting:
		return "preparing provider"
	case provider.ProgressConnecting:
		return "selecting and connecting to server"
	case provider.ProgressMeasuring:
		return "download and upload measurement active"
	case provider.ProgressDownloading:
		return "download measurement active"
	case provider.ProgressUploading:
		return "upload measurement active"
	case provider.ProgressComplete:
		return "measurement complete"
	case provider.ProgressFailed:
		return "measurement failed"
	case provider.ProgressCancelled:
		return "measurement cancelled"
	default:
		return "—"
	}
}

func providerElapsed(state providerState, now time.Time) (time.Duration, bool) {
	if state.startedAt.IsZero() {
		return 0, false
	}
	end := now
	if !state.endedAt.IsZero() {
		end = state.endedAt
	}
	return end.Sub(state.startedAt), true
}

func isTerminalPhase(phase provider.ProgressPhase) bool {
	return phase == provider.ProgressComplete || phase == provider.ProgressFailed || phase == provider.ProgressCancelled
}

func formatMbps(value *float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f Mbps", *value)
}

func formatElapsed(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	seconds := int(duration.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
