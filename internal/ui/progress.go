package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/soundadam/njuprobe/internal/model"
	"github.com/soundadam/njuprobe/internal/provider"
)

const refreshInterval = 250 * time.Millisecond

type ProgressRenderer struct {
	program *tea.Program
	ready   chan struct{}
	done    chan struct{}
	stop    sync.Once
	errMu   sync.Mutex
	runErr  error
}

func NewProgressRenderer(output io.Writer, version string, command model.Command) (*ProgressRenderer, error) {
	ready := make(chan struct{})
	progressModel := newProgressModel(version, command, ready)
	program := tea.NewProgram(
		progressModel,
		tea.WithInput(nil),
		tea.WithOutput(output),
	)
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
	liveMbps     *float64
	downloadMbps *float64
	uploadMbps   *float64
	message      string
}

type progressModel struct {
	version   string
	command   model.Command
	startedAt time.Time
	now       time.Time
	network   model.NetworkContext
	providers map[model.Provider]providerState
	order     []model.Provider
	pending   []provider.ProgressEvent
	ready     chan struct{}
	blank     bool
}

type progressMessage provider.ProgressEvent
type tickMessage time.Time
type stopMessage struct{}

func newProgressModel(version string, command model.Command, ready chan struct{}) *progressModel {
	order := []model.Provider{model.ProviderCampus, model.ProviderMLab}
	if command == model.CommandCampus {
		order = []model.Provider{model.ProviderCampus}
	}
	if command == model.CommandMLab {
		order = []model.Provider{model.ProviderMLab}
	}
	states := make(map[model.Provider]providerState, len(order))
	for _, name := range order {
		states[name] = providerState{phase: provider.ProgressWaiting}
	}
	now := time.Now()
	return &progressModel{
		version:   version,
		command:   command,
		startedAt: now,
		now:       now,
		providers: states,
		order:     order,
		ready:     ready,
	}
}

func (progress *progressModel) Init() tea.Cmd {
	close(progress.ready)
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(now time.Time) tea.Msg {
		return tickMessage(now)
	})
}

func (progress *progressModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case progressMessage:
		progress.pending = append(progress.pending, provider.ProgressEvent(message))
		return progress, nil
	case tickMessage:
		for _, event := range progress.pending {
			progress.applyEvent(event)
		}
		progress.pending = progress.pending[:0]
		progress.now = time.Time(message)
		return progress, tick()
	case stopMessage:
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
	state.phase = event.Phase
	state.test = event.Test
	state.server = event.Server
	state.liveMbps = cloneFloat(event.LiveMbps)
	if event.DownloadMbps != nil {
		state.downloadMbps = cloneFloat(event.DownloadMbps)
	}
	if event.UploadMbps != nil {
		state.uploadMbps = cloneFloat(event.UploadMbps)
	}
	state.message = event.Message
	progress.providers[event.Provider] = state
}

func (progress *progressModel) View() tea.View {
	if progress.blank {
		return tea.NewView("")
	}
	lines := []string{
		fmt.Sprintf("NJUProbe %s", progress.version),
		fmt.Sprintf("Network   %s", renderNetwork(progress.network)),
		fmt.Sprintf("Mode      %s · sequential", progress.command),
	}
	for _, name := range progress.order {
		lines = append(lines, renderProvider(name, progress.providers[name]))
	}
	lines = append(lines,
		fmt.Sprintf("Elapsed   %s", formatElapsed(progress.now.Sub(progress.startedAt))),
		"Ctrl-C    cancel",
	)
	return tea.NewView(strings.Join(lines, "\n"))
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

func renderProvider(name model.Provider, state providerState) string {
	label := "Campus"
	if name == model.ProviderMLab {
		label = "M-Lab"
	}
	status := string(state.phase)
	if state.phase == provider.ProgressComplete {
		status = fmt.Sprintf("complete · ↓ %s · ↑ %s", formatMbps(state.downloadMbps), formatMbps(state.uploadMbps))
	} else if state.liveMbps != nil {
		direction := state.test
		if direction == "" {
			direction = "live"
		}
		status = fmt.Sprintf("%s · %s %.2f Mbps", state.phase, direction, *state.liveMbps)
	} else if state.message != "" {
		status = fmt.Sprintf("%s · %s", state.phase, state.message)
	}
	return fmt.Sprintf("%-9s %s", label, status)
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
