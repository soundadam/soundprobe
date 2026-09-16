package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Charm palette shared with teaway's TUI: indigo titles, fuchsia current
// values, faint chrome, and green/red for settled states. Colors downsample
// through lipgloss / colorprofile; NO_COLOR is honored at the writer.
const (
	colorIndigoLight = "#5A56E0"
	colorIndigoDark  = "#7571F9"
	colorFuchsia     = "#F780E2"
	colorGreenLight  = "#02BA84"
	colorGreenDark   = "#02BF87"
	colorRedLight    = "#FF4672"
	colorRedDark     = "#ED567A"
	colorAmberLight  = "#C97800"
	colorAmberDark   = "#E0A458"
	colorFaint       = "243"
)

// Palette is the Charm/teaway color set used by interactive views and TTY
// command output. Dark defaults to true, matching teaway's client preview.
type Palette struct {
	Dark bool
}

// DefaultPalette returns the dark Charm palette used when the terminal
// background has not been reported yet.
func DefaultPalette() Palette {
	return Palette{Dark: true}
}

func (p Palette) lightDark() lipgloss.LightDarkFunc {
	return lipgloss.LightDark(p.Dark)
}

func (p Palette) indigo() color.Color {
	return p.lightDark()(lipgloss.Color(colorIndigoLight), lipgloss.Color(colorIndigoDark))
}

func (p Palette) green() color.Color {
	return p.lightDark()(lipgloss.Color(colorGreenLight), lipgloss.Color(colorGreenDark))
}

func (p Palette) red() color.Color {
	return p.lightDark()(lipgloss.Color(colorRedLight), lipgloss.Color(colorRedDark))
}

func (p Palette) amber() color.Color {
	return p.lightDark()(lipgloss.Color(colorAmberLight), lipgloss.Color(colorAmberDark))
}

func (p Palette) fuchsia() color.Color {
	return lipgloss.Color(colorFuchsia)
}

func (p Palette) faintColor() color.Color {
	return lipgloss.Color(colorFaint)
}

// TitleStyle is the indigo product/screen title, matching teaway's titleStyle.
func (p Palette) TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.indigo()).Bold(true)
}

// AccentStyle is the fuchsia current-value style, matching teaway's valueStyle.
func (p Palette) AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.fuchsia()).Bold(true)
}

// KnobStyle is fuchsia without bold, used for cursors, arrows, and tracks.
func (p Palette) KnobStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.fuchsia())
}

// FaintStyle is muted chrome: help, secondary status, unused options.
func (p Palette) FaintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.faintColor())
}

// OKStyle is a settled success state.
func (p Palette) OKStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.green())
}

// BadStyle is a settled failure or validation error.
func (p Palette) BadStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.red())
}

// WarnStyle is a partial or skipped state. Amber stays distinct from fuchsia.
func (p Palette) WarnStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(p.amber())
}

func (p Palette) Title(text string) string  { return p.TitleStyle().Render(text) }
func (p Palette) Accent(text string) string { return p.AccentStyle().Render(text) }
func (p Palette) Knob(text string) string   { return p.KnobStyle().Render(text) }
func (p Palette) Faint(text string) string  { return p.FaintStyle().Render(text) }
func (p Palette) OK(text string) string     { return p.OKStyle().Render(text) }
func (p Palette) Bad(text string) string    { return p.BadStyle().Render(text) }
func (p Palette) Warn(text string) string   { return p.WarnStyle().Render(text) }
