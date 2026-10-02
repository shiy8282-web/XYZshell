# XYZshell

XYZshell is a cross-platform SSH client project with a native Fyne desktop GUI, a CLI, and an embeddable Go API.

## Current MVP

- Password-authenticated SSH connection and remote shell.
- First-use host-key fingerprint confirmation, with trusted keys stored in the user's `XYZshell/known_hosts` file. A changed key is rejected.
- Native desktop window with an embedded VT-style terminal: direct keyboard input, ANSI control sequences, alternate screen, and PTY resize notifications.
- CLI and reusable connection/configuration packages.
- GUI view of secure SSH algorithms supported by the selected SSH library, plus its implemented legacy algorithms. Legacy algorithms are not enabled automatically.
- Passwords are not included in saved profiles or written to disk.

This is a first working foundation, not feature parity with PuTTY or SecureCRT. The GUI connects the terminal widget directly to an SSH PTY, so it supports character-by-character input and terminal resize. The CLI still accepts one line at a time. SFTP, port forwarding, key/agent authentication, profile persistence, session logging, and multi-tab sessions remain to be added.

## Requirements

- Go 1.27 or newer.
- Windows: GCC from MSYS2 MinGW x64 available on `PATH` for Fyne's CGO graphics backend.
- Linux: GCC and the platform graphics development headers for Fyne.
- Network access to a Go module proxy on the first build.

## Run

Start the native GUI:

```powershell
go run ./cmd/xyzshell-gui
```

Start the line-input CLI:

```powershell
go run ./cmd/xyzshell -host example.com -user myuser -port 22
```

## Run on Linux

`xyzshell-gui.exe` is a Windows executable and cannot run directly on Linux. Copy the project source to the Linux machine and install Fyne's Linux build prerequisites. On Debian or Ubuntu:

```sh
sudo apt-get update
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
```

Then, from the project directory, build and run the Linux binary:

```sh
go build -o xyzshell-gui ./cmd/xyzshell-gui
./xyzshell-gui
```

To run the CLI on Linux instead:

```sh
go build -o xyzshell-cli ./cmd/xyzshell
./xyzshell-cli -host example.com -user myuser -port 22
```

Fyne uses CGO and requires a C compiler and graphics headers on the build machine. The package list above is for Debian/Ubuntu; use the equivalent packages for other Linux distributions. See Fyne's [Linux prerequisites](https://docs.fyne.io/started/quick/) and [cross-compilation notes](https://docs.fyne.io/started/cross-compiling/).

Build both applications:

```powershell
go build -o xyzshell-cli.exe ./cmd/xyzshell
go build -ldflags="-H=windowsgui" -o xyzshell-gui.exe ./cmd/xyzshell-gui
```

## Module proxy

The default Go proxy is `https://proxy.golang.org`. If it is unreachable on your network, configure an approved module proxy, for example:

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
```

## API

`pkg/xyzshell` exposes endpoint configuration and algorithm capability queries. `pkg/sshclient` exposes `Connect`, `Client.Output`, `Client.SendLine`, and `Client.Close` for integration by other Go applications. A future stable API revision will add key/agent authentication, terminal resize, and raw terminal I/O.
