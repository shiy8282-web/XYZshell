package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"xyzshell/pkg/serialclient"
	"xyzshell/pkg/sessionlog"
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
	logSession := flag.Bool("log", false, "save terminal input and output to a session log")
	flag.Parse()

	var s session
	var err error
	var profile xyzshell.ConnectionProfile
	switch strings.ToLower(*protocol) {
	case "ssh":
		if *host == "" || *user == "" || *port == 0 || *port > 65535 {
			fmt.Fprintln(os.Stderr, "Usage: xyzshell -protocol ssh -host server.example -user username [-port 22] [-log]")
			os.Exit(2)
		}
		fmt.Fprint(os.Stderr, "SSH password: ")
		password, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if readErr != nil {
			fatal("read password", readErr)
		}
		cfg := xyzshell.Config{Host: *host, User: *user, Port: uint16(*port)}
		profile = xyzshell.ConnectionProfile{Protocol: "SSH", Host: cfg.Host, Port: cfg.Port, User: cfg.User}
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
			fmt.Fprintln(os.Stderr, "Usage: xyzshell -protocol telnet -host server.example [-port 23] [-log]")
			os.Exit(2)
		}
		profile = xyzshell.ConnectionProfile{Protocol: "Telnet", Host: *host, Port: uint16(*port)}
		var client *telnetclient.Client
		client, err = telnetclient.Connect(*host, uint16(*port))
		s = client
	case "serial":
		profile = xyzshell.ConnectionProfile{
			Protocol: "Serial", SerialPort: *serialPort, BaudRate: *baud,
			DataBits: *dataBits, Parity: *parity, StopBits: *stopBits,
		}
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

	if _, err := xyzshell.RememberConnection(profile); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save connection history: %v\n", err)
	}
	var recorder *sessionlog.Recorder
	if *logSession {
		recorder, err = sessionlog.New(profile)
		if err != nil {
			fatal("create session log", err)
		}
		defer recorder.Close()
		fmt.Fprintf(os.Stderr, "Session log: %s\nWarning: terminal input, including Telnet/serial credentials, may be recorded.\n", recorder.Path())
	}

	input := s.Input()
	output := s.Output()
	if recorder != nil {
		input = recorder.WrapInput(input)
		output = recorder.WrapOutput(output)
	}
	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(os.Stdout, output)
		close(readDone)
	}()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if _, err := io.WriteString(input, scanner.Text()+"\r"); err != nil {
			fatal("send input", err)
		}
	}
	if err := scanner.Err(); err != nil {
		fatal("read input", err)
	}
	_ = s.Close()
	<-readDone
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
