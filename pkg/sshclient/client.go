package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"xyzshell/pkg/xyzshell"
)

const connectTimeout = 15 * time.Second

var trustStoreMu sync.Mutex

// Client represents one authenticated remote terminal session.
type Client struct {
	client     *ssh.Client
	session    *ssh.Session
	stdin      io.WriteCloser
	stdout     io.Reader
	outPipe    *io.PipeReader
	algorithms ssh.NegotiatedAlgorithms
	close      sync.Once
}

// TrustPrompt is called only when the server key is not yet in known_hosts.
// Returning false rejects the connection. Existing changed keys are always
// rejected and cannot be overridden through this prompt.
type TrustPrompt func(host, fingerprint string) bool

func Connect(ctx context.Context, cfg xyzshell.Config, password string, ask TrustPrompt) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	knownHosts, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	if err := ensureKnownHosts(knownHosts); err != nil {
		return nil, err
	}
	verify, err := knownhosts.New(knownHosts)
	if err != nil {
		return nil, fmt.Errorf("load trusted host keys: %w", err)
	}

	var rawConn net.Conn
	callback := func(host string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(host, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) != 0 {
			return fmt.Errorf("server host key verification failed: %w", err)
		}
		if ask == nil || !ask(host, ssh.FingerprintSHA256(key)) {
			return errors.New("server host key was not trusted")
		}
		if rawConn != nil {
			_ = rawConn.SetDeadline(time.Now().Add(connectTimeout))
		}
		return appendKnownHost(knownHosts, host, key)
	}

	conn, err := (&net.Dialer{Timeout: connectTimeout}).DialContext(ctx, "tcp", cfg.Address())
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.Address(), err)
	}
	rawConn = conn
	_ = conn.SetDeadline(time.Now().Add(connectTimeout))
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	if signers := defaultKeySigners(); len(signers) > 0 {
		authMethods = append(authMethods, ssh.PublicKeys(signers...))
	}
	clientConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: callback,
		Timeout:         connectTimeout,
	}
	clientConn, channels, requests, err := ssh.NewClientConn(conn, cfg.Address(), clientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SSH handshake or authentication failed: %w", err)
	}
	var negotiated ssh.NegotiatedAlgorithms
	if metadata, ok := clientConn.(ssh.AlgorithmsConnMetadata); ok {
		negotiated = metadata.Algorithms()
	}
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(clientConn, channels, requests)
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("open SSH session: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open session input: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open session output: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("open session error output: %w", err)
	}
	if err := session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{
		ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("request remote terminal: %w", err)
	}
	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, fmt.Errorf("start remote shell: %w", err)
	}
	merged, mergedWriter := io.Pipe()
	var readers atomic.Int32
	readers.Store(2)
	copyStream := func(src io.Reader) {
		_, _ = io.Copy(mergedWriter, src)
		if readers.Add(-1) == 0 {
			_ = mergedWriter.Close()
		}
	}
	go copyStream(stdout)
	go copyStream(stderr)
	return &Client{client: client, session: session, stdin: stdin, stdout: merged, outPipe: merged, algorithms: negotiated}, nil
}

func (c *Client) Output() io.Reader { return c.stdout }

// EncryptionSummary reports the algorithms actually selected for this SSH connection.
func (c *Client) EncryptionSummary() string {
	return fmt.Sprintf("KEX %s | 主机密钥 %s | 加密 %s | MAC %s",
		c.algorithms.KeyExchange, c.algorithms.HostKey, c.algorithms.Read.Cipher, c.algorithms.Read.MAC)
}

// Input returns the raw terminal input stream for a terminal emulator widget.
func (c *Client) Input() io.WriteCloser { return c.stdin }

// Resize updates the remote PTY dimensions. rows and columns come from the
// terminal emulator's current cell grid.
func (c *Client) Resize(rows, columns uint) error {
	if rows == 0 || columns == 0 {
		return nil
	}
	return c.session.WindowChange(int(rows), int(columns))
}

func (c *Client) SendLine(line string) error {
	if strings.ContainsAny(line, "\r\n") {
		return errors.New("input must contain one line")
	}
	_, err := io.WriteString(c.stdin, line+"\r")
	return err
}

func (c *Client) Close() error {
	var err error
	c.close.Do(func() {
		if c.outPipe != nil {
			_ = c.outPipe.Close()
		}
		if c.session != nil {
			err = c.session.Close()
		}
		if c.client != nil {
			if closeErr := c.client.Close(); err == nil {
				err = closeErr
			}
		}
	})
	return err
}

func knownHostsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "XYZshell", "known_hosts"), nil
}

func ensureKnownHosts(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

func appendKnownHost(path, host string, key ssh.PublicKey) error {
	trustStoreMu.Lock()
	defer trustStoreMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("save trusted host key: %w", err)
	}
	_, writeErr := fmt.Fprintln(f, knownhosts.Line([]string{host}, key))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func defaultKeySigners() []ssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var signers []ssh.Signer
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err == nil {
			signers = append(signers, signer)
		}
	}
	return signers
}

