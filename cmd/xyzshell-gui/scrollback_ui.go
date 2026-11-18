package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
)

const (
	maxScrollWheelDelta = 48
	maxScrollbackRows   = 8000
)

// scrollbackTerminal keeps the live screen at the bottom of the same scroll
// area as the rows that have moved off its screen.
type scrollbackTerminal struct {
	*terminal.Terminal
	onScroll       func(*fyne.ScrollEvent)
	viewportHeight float32
}

func (t *scrollbackTerminal) MinSize() fyne.Size {
	size := t.Terminal.MinSize()
	if t.viewportHeight > size.Height {
		size.Height = t.viewportHeight
	}
	return size
}

func (t *scrollbackTerminal) Resize(size fyne.Size) {
	t.viewportHeight = size.Height
	t.Terminal.Resize(size)
}

// MouseDown focuses the wrapper that is embedded in the scroll content.
func (t *scrollbackTerminal) MouseDown(ev *desktop.MouseEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
		c.Focus(t)
	}
	t.Terminal.MouseDown(ev)
}

func (t *scrollbackTerminal) Scrolled(ev *fyne.ScrollEvent) {
	if t.onScroll != nil {
		t.onScroll(ev)
	}
}

// scrollbackScroll provides one continuously scrollable surface containing
// both historical rows and the current terminal screen.
type scrollbackScroll struct {
	*container.Scroll
	terminal   *scrollbackTerminal
	history    *widget.TextGrid
	showNotice bool
}

func (s *scrollbackScroll) Resize(size fyne.Size) {
	if s.terminal != nil {
		s.terminal.viewportHeight = size.Height
	}
	s.Scroll.Resize(size)
}

func (s *scrollbackScroll) AtBottom() bool {
	maxOffset := s.Content.MinSize().Height - s.Size().Height
	if maxOffset <= 1 {
		return true
	}
	return s.Offset.Y >= maxOffset-2
}

func (s *scrollbackScroll) ResetHistory() {
	s.showNotice = false
	s.history.Rows = nil
	s.history.Refresh()
	s.Scroll.Refresh()
}

func (s *scrollbackScroll) AppendRows(rows []widget.TextGridRow, dropped int) {
	if len(rows) == 0 && dropped == 0 {
		return
	}
	wasAtBottom := s.AtBottom()
	oldCount := len(s.history.Rows)
	if s.showNotice && oldCount > 0 {
		oldCount--
	}
	rowHeight := float32(0)
	if oldCount > 0 {
		rowHeight = s.history.MinSize().Height / float32(len(s.history.Rows))
	}

	historyRows := s.history.Rows
	if s.showNotice && len(historyRows) > 0 {
		historyRows = historyRows[1:]
	}
	historyRows = append(historyRows, rows...)
	if excess := len(historyRows) - maxScrollbackRows; excess > 0 {
		historyRows = append([]widget.TextGridRow(nil), historyRows[excess:]...)
		dropped += excess
	}
	if dropped > 0 {
		s.showNotice = true
	}
	s.history.Rows = historyRows
	if s.showNotice {
		notice := scrollbackNoticeRow("[Earlier terminal history omitted]")
		s.history.Rows = append([]widget.TextGridRow{notice}, historyRows...)
	}
	s.history.Refresh()
	s.Scroll.Refresh()

	if wasAtBottom {
		s.Scroll.ScrollToBottom()
	} else if dropped > 0 && rowHeight > 0 {
		y := fyne.Max(0, s.Offset.Y-float32(dropped)*rowHeight)
		s.Scroll.ScrollToOffset(fyne.NewPos(s.Offset.X, y))
	}
}

func scrollbackNoticeRow(text string) widget.TextGridRow {
	style := &widget.CustomTextGridStyle{
		TextStyle: fyne.TextStyle{Monospace: true},
		FGColor:   theme.Color(theme.ColorNameDisabled),
	}
	runes := []rune(text)
	row := widget.TextGridRow{Cells: make([]widget.TextGridCell, len(runes))}
	for i, r := range runes {
		row.Cells[i] = widget.TextGridCell{Rune: r, Style: style}
	}
	return row
}

func (s *scrollbackScroll) Scrolled(ev *fyne.ScrollEvent) {
	limited := *ev
	deltaY := limited.Scrolled.DY
	if deltaY > maxScrollWheelDelta {
		deltaY = maxScrollWheelDelta
	} else if deltaY < -maxScrollWheelDelta {
		deltaY = -maxScrollWheelDelta
	}
	limited.Scrolled = fyne.NewDelta(limited.Scrolled.DX, deltaY)
	s.Scroll.Scrolled(&limited)
}
