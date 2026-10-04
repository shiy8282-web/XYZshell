# XYZshell

XYZshell is a cross-platform terminal client with a native Fyne desktop GUI, a CLI, and reusable Go connection packages. It supports SSH, Telnet, and serial connections.

## Connection types

- **SSH**: password authentication, first-use host-key fingerprint confirmation, and trusted key storage in the user's configuration directory under XYZshell/known_hosts. Changed host keys are rejected.
- **Telnet**: TCP terminal connection with Telnet option negotiation, terminal type, and window-size reporting. Telnet sends data without encryption; use it only on trusted networks.
- **Serial**: choose or enter a port such as COM3, /dev/ttyUSB0, or /dev/ttyACM0; configure baud rate, data bits, parity, and stop bits.

The GUI connects the embedded terminal directly to the selected session and supports character-by-character input. SSH PTY and Telnet window sizes follow the terminal. Serial connections use the selected port's configured line settings.

Passwords are not saved. The terminal client does not currently include SFTP, port forwarding, SSH key/agent authentication, saved profiles, session logging, or multiple tabs.

## Requirements

- Go 1.27 or newer.
- Windows: GCC from MSYS2 MinGW x64 available on PATH for Fyne's CGO graphics backend.
- Linux: GCC and the platform graphics development headers for Fyne.
- Network access to a Go module proxy on the first build.

## Run

Start the native GUI:

~~~powershell
go run ./cmd/xyzshell-gui
~~~

Start an SSH CLI session:

~~~sh
go run ./cmd/xyzshell -protocol ssh -host example.com -user myuser -port 22
~~~

Start Telnet:

~~~sh
go run ./cmd/xyzshell -protocol telnet -host example.com -port 23
~~~

Open a serial terminal:

~~~sh
go run ./cmd/xyzshell -protocol serial -serial-port COM3 -baud 115200 -data-bits 8 -parity None -stop-bits 1
~~~

## Build

Build both applications on Windows:

~~~powershell
go build -o xyzshell-cli.exe ./cmd/xyzshell
go build -ldflags="-H=windowsgui" -o xyzshell-gui.exe ./cmd/xyzshell-gui
~~~

On Linux, Fyne uses CGO and requires a C compiler and graphics headers on the build machine. On Debian or Ubuntu:

~~~sh
sudo apt-get update
sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
go build -o xyzshell-gui ./cmd/xyzshell-gui
~~~

See Fyne's [Linux prerequisites](https://docs.fyne.io/started/quick/) and [cross-compilation notes](https://docs.fyne.io/started/cross-compiling/).

## Module proxy

The default Go proxy is https://proxy.golang.org. If it is unreachable on your network, configure an approved module proxy, for example:

~~~powershell
go env -w GOPROXY=https://goproxy.cn,direct
~~~

## API

pkg/sshclient, pkg/telnetclient, and pkg/serialclient expose the connection implementations. SSH host keys are checked before accepting a new server key.
