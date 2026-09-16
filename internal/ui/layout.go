package ui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// screen is the teaway-shaped frame: indigo title, faint description, fuchsia
// current value, indented body, two-line key help, and a short error sentence.
type screen struct {
	title       string
	version     string
	description string
	value       string
	hint        string
	body        []string
	help        []string
	err         string
	footnote    string
}

func (c chrome) render(view screen) string {
	p := c.palette
	lines := make([]string, 0, 8+len(view.body)+len(view.help))
	title := p.Title(view.title)
	if view.version != "" {
		title += " " + p.Faint(view.version)
	}
	lines = append(lines, title, "")
	if view.description != "" {
		lines = append(lines, wrapIndented(p.Faint, view.description, c.contentWidth())...)
	}
	if view.value != "" {
		lines = append(lines, bodyIndent+p.Accent(view.value))
	}
	if view.hint != "" {
		lines = append(lines, wrapIndented(p.Faint, view.hint, c.contentWidth())...)
	}
	if view.description != "" || view.value != "" || view.hint != "" {
		lines = append(lines, "")
	}
	if len(view.body) == 0 {
		lines = append(lines, bodyIndent+p.Faint("nothing here"))
	} else {
		lines = append(lines, view.body...)
	}
	lines = append(lines, "")
	for _, help := range view.help {
		lines = append(lines, bodyIndent+help)
	}
	if view.footnote != "" {
		lines = append(lines, wrapIndented(p.Faint, view.footnote, c.contentWidth())...)
	}
	if view.err != "" {
		lines = append(lines, wrapIndented(p.Bad, view.err, c.contentWidth())...)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), " ")
}

func (c chrome) helpKeys(keys ...string) string {
	return c.palette.Faint(strings.Join(keys, helpGap))
}

func (c chrome) helpArrows(upActive, downActive bool) string {
	up, down := c.palette.Knob("↑"), c.palette.Knob("↓")
	if !upActive {
		up = c.palette.Faint("↑")
	}
	if !downActive {
		down = c.palette.Faint("↓")
	}
	return up + "  " + down
}

func (c chrome) helpHorizontal(leftActive, rightActive bool) string {
	left, right := c.palette.Knob("←"), c.palette.Knob("→")
	if !leftActive {
		left = c.palette.Faint("←")
	}
	if !rightActive {
		right = c.palette.Faint("→")
	}
	return left + "  " + right
}

func (c chrome) cursor(active bool) string {
	if active {
		return c.palette.Knob(">") + " "
	}
	return "  "
}

func (c chrome) check(selected, disabled bool) string {
	switch {
	case disabled:
		return c.palette.Faint("– ")
	case selected:
		return c.palette.OK("✓ ")
	default:
		return c.palette.Faint("• ")
	}
}

func (c chrome) optionLabel(text string, active, disabled bool) string {
	switch {
	case disabled:
		return c.palette.Faint(text)
	case active:
		return c.palette.Accent(text)
	default:
		return text
	}
}

func (c chrome) indent(text string) string {
	return bodyIndent + text
}

func (c chrome) indentFaint(text string) string {
	return bodyIndent + c.palette.Faint(truncateRunes(text, c.contentWidth()))
}

func (c chrome) languageChoice(chinese, english bool) string {
	zh, en := "中文", "English"
	if chinese {
		zh = c.palette.Accent("中文")
	} else {
		zh = c.palette.Faint("中文")
	}
	if english {
		en = c.palette.Accent("English")
	} else {
		en = c.palette.Faint("English")
	}
	cursor := c.palette.Knob(">")
	if english && !chinese {
		return zh + "     " + cursor + " " + en
	}
	return cursor + " " + zh + "     " + en
}

func wrapIndented(style func(string) string, value string, width int) []string {
	if width <= 0 {
		width = 16
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(value)
	raw := strings.Split(wrapped, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		lines = append(lines, bodyIndent+style(line))
	}
	if len(lines) == 0 {
		return []string{bodyIndent + style(value)}
	}
	return lines
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= limit && lipgloss.Width(value) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	var builder strings.Builder
	width := 0
	for _, r := range value {
		rw := lipgloss.Width(string(r))
		if width+rw > limit-1 {
			builder.WriteRune('…')
			return builder.String()
		}
		builder.WriteRune(r)
		width += rw
	}
	return builder.String()
}
