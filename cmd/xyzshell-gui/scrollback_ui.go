package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/fyne-io/terminal"
)

// scrollbackTerminal makes the normal mouse wheel open the current connection's
// buffered output, as it does in desktop terminal clients.
type scrollbackTerminal struct {
	*terminal.Terminal
	onScrollback func()
}

// MouseDown focuses the wrapper that is actually embedded in the Fyne canvas.
// The embedded Terminal's own MouseDown method looks itself up in the canvas,
// but the wrapper is the object present there, so that lookup can fail and
// keyboard input is then never delivered to the terminal.
func (t *scrollbackTerminal) MouseDown(ev *desktop.MouseEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
		c.Focus(t)
	}
	t.Terminal.MouseDown(ev)
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
