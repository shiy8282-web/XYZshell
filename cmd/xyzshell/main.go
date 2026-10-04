package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"xyzshell/pkg/serialclient"
	"xyzshell/pkg/sshclient"
	"xyzshell/pkg/telnetclient"
	"xyzshell/pkg/xyzshell"
)

const version = "0.1.0"

type session interface {
	Input() io.WriteCloser
	Output() io.Reader
	Resize(rows, columns uint) error
	Close() error
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println("xyzshell", version)
		return
	}
	protocol := flag.String("protocol", "ssh", "connection type: ssh, telnet, or serial")
	host := flag.String("host", "", "SSH/Telnet server hostname or IP")
	user := flag.String("user", "", "SSH username")
	port := flag.Uint("port", 22, "SSH/Telnet server port")
	serialPort := flag.String("serial-port", "", "serial port name, such as COM3 or /dev/ttyUSB0")
	baud := flag.Int("baud", 9600, "serial baud rate")
	dataBits := flag.Int("data-bits", 8, "serial data bits (5-8)")
	parity := flag.String("parity", "None", "serial parity: None, Even, or Odd")
	stopBits := flag.Int("stop-bits", 1, "serial stop bits: 1 or 2")
	flag.Parse()

	var s session
	var err error
	switch strings.ToLower(*protocol) {
	case "ssh":
		if *host == "" || *user == "" || *port == 0 || *port > 65535 {
			fmt.Fprintln(os.Stderr, "Usage: xyzshell -protocol ssh -host server.example -user username [-port 22]")
			os.Exit(2)
		}
		fmt.Fprint(os.Stderr, "SSH password: ")
		password, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if readErr != nil {
			fatal("read password", readErr)
		}
		cfg := xyzshell.Config{Host: *host, User: *user, Port: uint16(*port)}
		var client *sshclient.Client
		client, err = sshclient.Connect(context.Background(), cfg, string(password), func(host, fingerprint string) bool {
			fmt.Fprintf(os.Stderr, "Untrusted SSH host key for %s\nSHA256 fingerprint: %s\nTrust this key? [y/N] ", host, fingerprint)
			var answer string
			_, _ = fmt.Scanln(&answer)
			return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
		})
		for i := range password {
			password[i] = 0
		}
		s = client
	case "telnet":
		if *host == "" || *port == 0 || *port > 65535 {
			fmt.Fprintln(os.Stderr, "Usage: xyzshell -protocol telnet -host server.example [-port 23]")
			os.Exit(2)
		}
		var client *telnetclient.Client
		client, err = telnetclient.Connect(*host, uint16(*port))
		s = client
	case "serial":
		var client *serialclient.Client
		client, err = serialclient.Connect(serialclient.Config{
			Port: *serialPort, BaudRate: *baud, DataBits: *dataBits,
			Parity: *parity, StopBits: *stopBits,
		})
		s = client
	default:
		fmt.Fprintln(os.Stderr, "protocol must be ssh, telnet, or serial")
		os.Exit(2)
	}
	if err != nil {
		fatal("connect", err)
	}
	defer s.Close()

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fatal("set terminal raw mode", err)
	}
	defer term.Restore(fd, oldState)

	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(os.Stdout, s.Output())
		readDone <- copyErr
	}()
	inputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(s.Input(), os.Stdin)
		inputDone <- copyErr
	}()
	select {
	case err = <-readDone:
	case err = <-inputDone:
	}
	_ = s.Close()
	if err != nil && err != io.EOF {
		fatal("terminal session", err)
	}
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
