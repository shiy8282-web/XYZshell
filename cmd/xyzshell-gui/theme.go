package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

const (
	interfaceShutdownColor fyne.ThemeColorName = "xyzshellInterfaceShutdown"
	interfaceTrunkColor    fyne.ThemeColorName = "xyzshellInterfaceTrunk"
	interfaceMACColor      fyne.ThemeColorName = "xyzshellInterfaceMAC"
	interfaceDot1XColor    fyne.ThemeColorName = "xyzshellInterfaceDot1X"
	interfaceAccessColor   fyne.ThemeColorName = "xyzshellInterfaceAccess"
)

// persistentScrollbarTheme keeps the scrollbar at its normal width when idle.
type persistentScrollbarTheme struct {
	fyne.Theme
}

func (t persistentScrollbarTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case interfaceShutdownColor:
		return color.NRGBA{R: 142, G: 147, B: 155, A: 255}
	case interfaceTrunkColor:
		return color.NRGBA{R: 105, G: 177, B: 135, A: 255}
	case interfaceMACColor:
		return color.NRGBA{R: 174, G: 135, B: 204, A: 255}
	case interfaceDot1XColor:
		return color.NRGBA{R: 112, G: 157, B: 205, A: 255}
	case interfaceAccessColor:
		return color.NRGBA{R: 211, G: 119, B: 119, A: 255}
	default:
		return t.Theme.Color(name, variant)
	}
}

func (t persistentScrollbarTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameScrollBarSmall {
		return t.Theme.Size(theme.SizeNameScrollBar)
	}
	return t.Theme.Size(name)
}
