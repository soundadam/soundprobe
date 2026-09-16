package ui

import (
	tea "charm.land/bubbletea/v2"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	minWidth      = 40
	bodyIndent    = "  "
	helpGap       = "   "
)

// chrome is the shared layout context for inline Bubble Tea views: terminal
// size, Charm palette, and the 2-space body indent used by teaway.
type chrome struct {
	width   int
	height  int
	palette Palette
}

func newChrome() chrome {
	return chrome{
		width:   defaultWidth,
		height:  defaultHeight,
		palette: DefaultPalette(),
	}
}

func (c chrome) update(message tea.Msg) chrome {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		c.width = message.Width
		c.height = message.Height
	case tea.BackgroundColorMsg:
		c.palette.Dark = message.IsDark()
	}
	return c
}

func (c chrome) clampedWidth() int {
	width := c.width
	if width <= 0 {
		return defaultWidth
	}
	if width < minWidth {
		return minWidth
	}
	return width
}

func (c chrome) contentWidth() int {
	width := c.clampedWidth() - len(bodyIndent)
	if width < 16 {
		return 16
	}
	return width
}

func (c chrome) activityWidth() int {
	switch {
	case c.width > 0 && c.width < 50:
		return 12
	case c.width > 0 && c.width < 70:
		return 16
	case c.width > 120:
		return 32
	default:
		return activityWidth
	}
}

func (c chrome) detailLimit() int {
	limit := c.contentWidth()
	if limit > detailRuneLimit {
		limit = detailRuneLimit
	}
	if limit < 20 {
		return 20
	}
	return limit
}

func (c chrome) descriptionLimit() int {
	limit := c.contentWidth() - 18
	if limit < 12 {
		return 12
	}
	if limit > 44 {
		return 44
	}
	return limit
}

func (c chrome) statusLimit() int {
	limit := c.contentWidth()
	if limit > 68 {
		return 68
	}
	if limit < 16 {
		return 16
	}
	return limit
}
