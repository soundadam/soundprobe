package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
	return &setupModel{version: version, language: current.Language, stations: stations, selected: selected}
}

func (setup *setupModel) Init() tea.Cmd { return nil }

func (setup *setupModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return setup, nil
	}
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
		zh, en := "  中文  ", "  English  "
		if setup.language == preferences.LanguageChinese {
			zh = "› 中文"
		} else {
			en = "› English"
		}
		return tea.NewView(strings.Join([]string{
			fmt.Sprintf("soundprobe %s · welcome / 欢迎", setup.version),
			"",
			"Choose interface language / 选择界面语言",
			"",
			zh + "     " + en,
			"",
			"←/→ switch   Enter continue   q cancel",
		}, "\n"))
	}
	lines := []string{
		fmt.Sprintf("soundprobe %s · %s", setup.version, setup.text("选择日常测速站", "choose daily stations")),
		setup.text("以后直接运行 soundprobe 时只显示这些站点，可用 soundprobe setup 修改。", "Only these stations appear in daily use. Run soundprobe setup to change them."),
		"",
	}
	for index, station := range setup.stations {
		cursor := "  "
		if index == setup.cursor {
			cursor = "› "
		}
		check := "[ ]"
		if setup.selected[station.ID] {
			check = "[x]"
		}
		description, useCase := station.Description, station.UseCase
		if setup.language == preferences.LanguageChinese {
			description, useCase = station.DescriptionZH, station.UseCaseZH
		}
		lines = append(lines, fmt.Sprintf("%s%s %-12s %s", cursor, check, station.Label, description), "      "+useCase)
	}
	lines = append(lines, "",
		setup.text("网页测速（不加入日常 CLI）：南大 http://test.nju.edu.cn · 中科大 https://test.ustc.edu.cn", "Web tests (not daily CLI): NJU http://test.nju.edu.cn · USTC https://test.ustc.edu.cn"),
		"",
		setup.text("↑/↓ 移动   Space 选择   Enter 保存   b 返回   q 取消", "↑/↓ move   Space toggle   Enter save   b back   q cancel"),
	)
	if setup.errorText != "" {
		lines = append(lines, setup.errorText)
	}
	return tea.NewView(strings.Join(lines, "\n"))
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
