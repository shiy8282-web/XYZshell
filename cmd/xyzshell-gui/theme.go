package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// persistentScrollbarTheme keeps the scrollbar at its normal width when idle.
type persistentScrollbarTheme struct {
	fyne.Theme
}

func (t persistentScrollbarTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameScrollBarSmall {
		return t.Theme.Size(theme.SizeNameScrollBar)
	}
	return t.Theme.Size(name)
}
