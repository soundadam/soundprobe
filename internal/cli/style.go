package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// styleSet holds the lipgloss styles used for human-readable output. When
// disabled (stdout is not a TTY) every helper returns its input unchanged so
// redirected output stays plain text.
type styleSet struct {
	enabled bool
	title   lipgloss.Style
	header  lipgloss.Style
	dim     lipgloss.Style
	ok      lipgloss.Style
	warn    lipgloss.Style
	bad     lipgloss.Style
	accent  lipgloss.Style
}

func newStyleSet(enabled bool) styleSet {
	return styleSet{
		enabled: enabled,
		title:   lipgloss.NewStyle().Bold(true),
		header:  lipgloss.NewStyle().Faint(true),
		dim:     lipgloss.NewStyle().Faint(true),
		ok:      lipgloss.NewStyle().Foreground(lipgloss.Green),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Yellow),
		bad:     lipgloss.NewStyle().Foreground(lipgloss.Red),
		accent:  lipgloss.NewStyle().Foreground(lipgloss.Cyan),
	}
}

func (styles styleSet) render(style lipgloss.Style, text string) string {
	if !styles.enabled || text == "" {
		return text
	}
	return style.Render(text)
}

func (styles styleSet) Title(text string) string  { return styles.render(styles.title, text) }
func (styles styleSet) Header(text string) string { return styles.render(styles.header, text) }
func (styles styleSet) Dim(text string) string    { return styles.render(styles.dim, text) }
func (styles styleSet) OK(text string) string     { return styles.render(styles.ok, text) }
func (styles styleSet) Warn(text string) string   { return styles.render(styles.warn, text) }
func (styles styleSet) Bad(text string) string    { return styles.render(styles.bad, text) }
func (styles styleSet) Accent(text string) string { return styles.render(styles.accent, text) }

// Status colors a well-known status word (run, measurement, or probe state);
// unrecognized words pass through untouched.
func (styles styleSet) Status(text string) string {
	switch text {
	case "success", "reachable", "ready":
		return styles.OK(text)
	case "partial", "skipped", "automatic":
		return styles.Warn(text)
	case "failed", "unreachable":
		return styles.Bad(text)
	case "cancelled", "unsupported":
		return styles.Dim(text)
	default:
		return text
	}
}

// humanOutput returns the writer and styles for human-readable rendering.
// On a TTY the writer downsamples colors to the terminal's capabilities and
// honors NO_COLOR; otherwise styling is disabled entirely.
func (app *App) humanOutput() (io.Writer, styleSet) {
	if !app.StdoutTTY {
		return app.Out, newStyleSet(false)
	}
	return colorprofile.NewWriter(app.Out, os.Environ()), newStyleSet(true)
}

// writeAligned renders rows as space-padded columns separated by two spaces.
// Cell widths are measured with ANSI escapes ignored, so styled cells stay
// aligned with plain ones.
func writeAligned(out io.Writer, rows [][]string) {
	var widths []int
	for _, row := range rows {
		for column, cell := range row {
			width := lipgloss.Width(cell)
			if column == len(widths) {
				widths = append(widths, width)
			} else if width > widths[column] {
				widths[column] = width
			}
		}
	}
	for _, row := range rows {
		var line strings.Builder
		for column, cell := range row {
			if column > 0 {
				line.WriteString("  ")
			}
			line.WriteString(cell)
			if column < len(row)-1 {
				line.WriteString(strings.Repeat(" ", widths[column]-lipgloss.Width(cell)))
			}
		}
		fmt.Fprintln(out, strings.TrimRight(line.String(), " "))
	}
}
