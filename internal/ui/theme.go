package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Charm-aligned palette shared with teaway's TUI (huh ThemeCharm + custom
// picker). Indigo titles, fuchsia selection/values, gray help, and Charm
// green/red for outcome states.
var (
	indigoLight = lipgloss.Color("#5A56E0")
	indigoDark  = lipgloss.Color("#7571F9")
	fuchsia     = lipgloss.Color("#F780E2")
	greenLight  = lipgloss.Color("#02BA7C")
	greenDark   = lipgloss.Color("#02BF87")
	redLight    = lipgloss.Color("#FF4672")
	redDark     = lipgloss.Color("#ED567A")
	warnLight   = lipgloss.Color("#AD7F00")
	warnDark    = lipgloss.Color("#FFC14A")
	faintColor  = lipgloss.Color("243")
)

const (
	brandName   = "soundprobe"
	frameIndent = "  "
)

// Theme holds the lipgloss styles for one background. Interactive views
// default to a dark Charm palette so snapshots stay deterministic; they can
// follow tea.BackgroundColorMsg when a real terminal reports it.
type Theme struct {
	Title  lipgloss.Style
	Value  lipgloss.Style
	Accent lipgloss.Style
	Faint  lipgloss.Style
	Cursor lipgloss.Style
	OK     lipgloss.Style
	Warn   lipgloss.Style
	Bad    lipgloss.Style
	Help   lipgloss.Style
}

func NewTheme(dark bool) Theme {
	lightDark := lipgloss.LightDark(dark)
	indigo := lightDark(indigoLight, indigoDark)
	green := lightDark(greenLight, greenDark)
	red := lightDark(redLight, redDark)
	warn := lightDark(warnLight, warnDark)
	return Theme{
		Title:  lipgloss.NewStyle().Foreground(indigo).Bold(true),
		Value:  lipgloss.NewStyle().Foreground(fuchsia).Bold(true),
		Accent: lipgloss.NewStyle().Foreground(fuchsia),
		Faint:  lipgloss.NewStyle().Foreground(faintColor),
		Cursor: lipgloss.NewStyle().Foreground(fuchsia),
		OK:     lipgloss.NewStyle().Foreground(green),
		Warn:   lipgloss.NewStyle().Foreground(warn),
		Bad:    lipgloss.NewStyle().Foreground(red),
		Help:   lipgloss.NewStyle().Foreground(faintColor),
	}
}

func (theme Theme) brand() string {
	return theme.Title.Render(brandName)
}

func (theme Theme) help(keys ...string) string {
	return theme.Help.Render(strings.Join(keys, "   "))
}

// renderFrame paints the teaway-style hierarchy: indigo brand title, a blank
// line, 2-space-indented body, then a faint help cluster.
func renderFrame(theme Theme, body []string, help []string) string {
	lines := []string{theme.brand()}
	if len(body) > 0 {
		lines = append(lines, "")
		for _, line := range body {
			if line == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, frameIndent+line)
		}
	}
	if len(help) > 0 {
		lines = append(lines, "")
		for _, line := range help {
			if line == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, frameIndent+line)
		}
	}
	return strings.Join(lines, "\n")
}

func padRight(text string, width int) string {
	extra := width - lipgloss.Width(text)
	if extra <= 0 {
		return text
	}
	return text + strings.Repeat(" ", extra)
}

func choiceRow(theme Theme, focused, selected, supported bool, label, description string) []string {
	cursor := "  "
	if focused {
		cursor = theme.Cursor.Render("› ")
	}
	check := "[ ]"
	switch {
	case !supported:
		check = theme.Faint.Render("[-]")
	case selected:
		check = theme.Accent.Render("[x]")
	}
	renderedLabel := label
	if focused {
		renderedLabel = theme.Cursor.Render(label)
	}
	return []string{cursor + check + "  " + padRight(renderedLabel, 12) + " " + description}
}
