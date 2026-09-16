package ui

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/soundadam/soundprobe/internal/preferences"
	"github.com/soundadam/soundprobe/internal/target"
)

var ErrSetupCancelled = errors.New("setup cancelled")

type setupModel struct {
	version   string
	language  preferences.Language
	screen    int
	stations  []target.Station
	cursor    int
	selected  map[string]bool
	done      bool
	cancelled bool
	errorText string
	chrome    chrome
}

func Configure(ctx context.Context, input io.Reader, output io.Writer, version string, current preferences.Config) (preferences.Config, error) {
	model := newSetupModel(version, current)
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return preferences.Config{}, err
	}
	setup, ok := finalModel.(*setupModel)
	if !ok {
		return preferences.Config{}, errors.New("setup returned an unexpected model")
	}
	if setup.cancelled {
		return preferences.Config{}, ErrSetupCancelled
	}
	if !setup.done {
		return preferences.Config{}, errors.New("setup exited without preferences")
	}
	return setup.config(), nil
}

func newSetupModel(version string, current preferences.Config) *setupModel {
	if current.Validate() != nil {
		current = preferences.DefaultConfig()
	}
	stations := make([]target.Station, 0)
	for _, station := range target.Stations() {
		if station.TerminalSupported && station.DailyEligible && target.PlatformAvailable(station) {
			stations = append(stations, station)
		}
	}
	selected := map[string]bool{}
	for _, id := range current.DailyStations {
		selected[id] = true
	}
	return &setupModel{
		version:  version,
		language: current.Language,
		stations: stations,
		selected: selected,
		chrome:   newChrome(),
	}
}

func (setup *setupModel) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (setup *setupModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg, tea.BackgroundColorMsg:
		setup.chrome = setup.chrome.update(message)
		return setup, nil
	case tea.KeyPressMsg:
		return setup.handleKey(message)
	default:
		return setup, nil
	}
}

func (setup *setupModel) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	setup.errorText = ""
	if key.String() == "ctrl+c" || key.String() == "q" || key.String() == "esc" {
		setup.cancelled = true
		return setup, tea.Quit
	}
	if setup.screen == 0 {
		switch key.String() {
		case "left", "right", "h", "l", "tab", "space":
			if setup.language == preferences.LanguageChinese {
				setup.language = preferences.LanguageEnglish
			} else {
				setup.language = preferences.LanguageChinese
			}
		case "enter":
			setup.screen = 1
		}
		return setup, nil
	}
	switch key.String() {
	case "up", "k":
		if setup.cursor > 0 {
			setup.cursor--
		}
	case "down", "j":
		if setup.cursor+1 < len(setup.stations) {
			setup.cursor++
		}
	case "home":
		setup.cursor = 0
	case "end":
		if len(setup.stations) > 0 {
			setup.cursor = len(setup.stations) - 1
		}
	case "space":
		id := setup.stations[setup.cursor].ID
		setup.selected[id] = !setup.selected[id]
	case "b":
		setup.screen = 0
	case "enter":
		if len(setup.selectedIDs()) == 0 {
			setup.errorText = setup.text("请至少选择一个日常测速站", "Select at least one daily station")
			return setup, nil
		}
		setup.done = true
		return setup, tea.Quit
	}
	return setup, nil
}

func (setup *setupModel) View() tea.View {
	if setup.done || setup.cancelled {
		return tea.NewView("")
	}
	if setup.screen == 0 {
		chinese := setup.language == preferences.LanguageChinese
		return tea.NewView(setup.chrome.render(screen{
			title:       "soundprobe",
			version:     setup.version,
			description: "Choose interface language / 选择界面语言",
			body:        []string{setup.chrome.indent(setup.chrome.languageChoice(chinese, !chinese))},
			help: []string{
				setup.chrome.helpHorizontal(true, true),
				setup.chrome.helpKeys("enter", "q"),
			},
		}))
	}
	body := make([]string, 0, len(setup.stations)*2)
	if len(setup.stations) == 0 {
		body = append(body, setup.chrome.indentFaint(setup.text("没有可加入日常测速的站点", "no daily stations available")))
	}
	for index, station := range setup.stations {
		active := index == setup.cursor
		label := setup.chrome.cursor(active) + setup.chrome.check(setup.selected[station.ID], false) +
			setup.chrome.optionLabel(station.Label, active, false)
		description, useCase := station.Description, station.UseCase
		if setup.language == preferences.LanguageChinese {
			description, useCase = station.DescriptionZH, station.UseCaseZH
		}
		body = append(body,
			setup.chrome.indent(label+"  "+setup.chrome.palette.Faint(truncateRunes(description, setup.chrome.descriptionLimit()))),
			setup.chrome.indentFaint(useCase),
		)
	}
	upActive := setup.cursor > 0
	downActive := setup.cursor+1 < len(setup.stations)
	return tea.NewView(setup.chrome.render(screen{
		title:       "soundprobe",
		version:     setup.version,
		description: setup.text("选择日常测速站", "choose daily stations"),
		hint:        setup.text("以后直接运行 soundprobe 时只显示这些站点。", "Only these stations appear in daily use."),
		body:        body,
		help: []string{
			setup.chrome.helpArrows(upActive, downActive),
			setup.chrome.helpKeys("space", "enter", "b", "q"),
		},
		err:      setup.errorText,
		footnote: setup.text("网页测速（不加入日常 CLI）：南大 http://test.nju.edu.cn · 中科大 https://test.ustc.edu.cn", "Web tests (not daily CLI): NJU http://test.nju.edu.cn · USTC https://test.ustc.edu.cn"),
	}))
}

func (setup *setupModel) selectedIDs() []string {
	ids := make([]string, 0, len(setup.selected))
	for _, station := range setup.stations {
		if setup.selected[station.ID] {
			ids = append(ids, station.ID)
		}
	}
	return ids
}

func (setup *setupModel) config() preferences.Config {
	return preferences.Config{SchemaVersion: preferences.SchemaVersion, Language: setup.language, DailyStations: setup.selectedIDs()}
}

func (setup *setupModel) text(chinese, english string) string {
	if setup.language == preferences.LanguageChinese {
		return chinese
	}
	return english
}
