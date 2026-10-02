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
	"xyzshell/pkg/sshclient"
	"xyzshell/pkg/xyzshell"
)

const version = "0.1.0"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println("xyzshell", version)
		return
	}
	host := flag.String("host", "", "SSH server hostname or IP")
	user := flag.String("user", "", "SSH username")
	port := flag.Uint("port", 22, "SSH server port")
	flag.Parse()
	if *host == "" || *user == "" || *port == 0 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "Usage: xyzshell -host server.example -user username [-port 22]")
		os.Exit(2)
	}

	fmt.Fprint(os.Stderr, "SSH password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fatal("read password", err)
	}
	cfg := xyzshell.Config{Host: *host, User: *user, Port: uint16(*port)}
	client, err := sshclient.Connect(context.Background(), cfg, string(password), func(host, fingerprint string) bool {
		fmt.Fprintf(os.Stderr, "Untrusted SSH host key for %s\nSHA256 fingerprint: %s\nTrust this key? [y/N] ", host, fingerprint)
		var answer string
		_, _ = fmt.Scanln(&answer)
		return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
	})
	for i := range password {
		password[i] = 0
	}
	if err != nil {
		fatal("connect", err)
	}
	defer client.Close()

	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(os.Stdout, client.Output())
		close(readDone)
	}()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if err := client.SendLine(scanner.Text()); err != nil {
			fatal("send input", err)
		}
	}
	if err := scanner.Err(); err != nil {
		fatal("read input", err)
	}
	_ = client.Close()
	<-readDone
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
