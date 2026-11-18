# Local terminal fork

This directory contains the runtime Go sources from `github.com/fyne-io/terminal` at the version pinned in the root `go.mod`.

XYZshell keeps this small local fork so the terminal can expose rows as they leave the live screen. The GUI uses those rows to show scrollback above the live screen in one continuous terminal viewport. The upstream package's license is included in `LICENSE`.
