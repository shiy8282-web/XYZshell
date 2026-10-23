package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/fyne-io/terminal"
)

// scrollbackTerminal makes the normal mouse wheel open the current connection's
// buffered output, as it does in desktop terminal clients.
type scrollbackTerminal struct {
	*terminal.Terminal
	onScrollback func()
}

func (t *scrollbackTerminal) Scrolled(ev *fyne.ScrollEvent) {
	if ev.Scrolled.DY > 0 && t.onScrollback != nil {
		t.onScrollback()
	}
}

// scrollbackScroll returns to the live terminal when the user scrolls down
// after reaching the newest output.
type scrollbackScroll struct {
	*container.Scroll
	onReturn func()
}

func (s *scrollbackScroll) Scrolled(ev *fyne.ScrollEvent) {
	maxOffset := s.Content.MinSize().Height - s.Size().Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if ev.Scrolled.DY < 0 && s.Offset.Y >= maxOffset-1 && s.onReturn != nil {
		s.onReturn()
		return
	}
	s.Scroll.Scrolled(ev)
}
