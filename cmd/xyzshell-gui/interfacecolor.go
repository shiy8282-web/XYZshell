package main

import (
	"bytes"
	"io"
	"regexp"
	"strings"
)

// interfaceColorReader delays only interface configuration blocks so it can
// color the full block consistently before the terminal renders it.
type interfaceColorReader struct {
	source  io.Reader
	output  []byte
	partial []byte
	block   [][]byte
	eof     bool
}

func newInterfaceColorReader(source io.Reader) *interfaceColorReader {
	return &interfaceColorReader{source: source}
}

func (r *interfaceColorReader) Read(p []byte) (int, error) {
	for len(r.output) == 0 && !r.eof {
		buf := make([]byte, 4096)
		n, err := r.source.Read(buf)
		for _, b := range buf[:n] {
			r.partial = append(r.partial, b)
			if b == '\n' {
				r.processLine(r.partial)
				r.partial = nil
			}
		}

		// Do not hold ordinary terminal output or prompts for a newline. Keep
		// only a possible "interface ..." header buffered for classification.
		if r.block == nil && len(r.partial) > 0 && !couldStartInterface(r.partial) {
			r.output = append(r.output, r.partial...)
			r.partial = nil
		} else if len(r.block) > 0 && len(r.partial) > 0 && isOutputBoundary(strings.TrimSpace(stripANSI(string(r.partial)))) {
			if !isMoreMarker(strings.TrimSpace(stripANSI(string(r.partial)))) {
				r.flushBlock()
			}
			r.output = append(r.output, r.partial...)
			r.partial = nil
		}

		if err != nil {
			r.eof = true
			if len(r.partial) > 0 {
				r.processLine(r.partial)
				r.partial = nil
			}
			r.flushBlock()
			if len(r.output) > 0 {
				break
			}
			return 0, err
		}
		if n == 0 && len(r.output) == 0 {
			return 0, nil
		}
	}

	if len(r.output) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.output)
	r.output = r.output[n:]
	if len(r.output) == 0 {
		r.output = nil
	}
	return n, nil
}

func (r *interfaceColorReader) processLine(line []byte) {
	plain := strings.TrimSpace(stripANSI(string(line)))
	if isInterfaceHeader(plain) {
		r.flushBlock()
		r.block = [][]byte{append([]byte(nil), line...)}
		return
	}
	if len(r.block) == 0 {
		r.output = append(r.output, line...)
		return
	}
	if plain == "!" || isOutputBoundary(plain) {
		if !isMoreMarker(plain) {
			r.flushBlock()
		}
		r.output = append(r.output, line...)
		return
	}
	r.block = append(r.block, append([]byte(nil), line...))
}

func (r *interfaceColorReader) flushBlock() {
	if len(r.block) == 0 {
		return
	}
	color := classifyInterfaceBlock(r.block)
	for _, line := range r.block {
		if color == "" {
			r.output = append(r.output, line...)
			continue
		}
		r.output = append(r.output, color...)
		r.output = append(r.output, line...)
		r.output = append(r.output, "\x1b[0m"...)
	}
	r.block = nil
}

func classifyInterfaceBlock(lines [][]byte) string {
	if len(lines) == 0 {
		return ""
	}
	header := strings.ToLower(strings.TrimSpace(stripANSI(string(lines[0]))))
	body := make([]string, 0, len(lines))
	for _, line := range lines[1:] {
		body = append(body, strings.ToLower(strings.TrimSpace(stripANSI(string(line)))))
	}
	text := strings.Join(body, "\n")

	// Explicit routed interfaces and virtual interfaces are outside the
	// Layer-2 port rules. Physical switch ports are Layer 2 by default unless
	// their configuration says otherwise.
	if strings.Contains(text, "no switchport") || containsAny(text, "ip address ", "ipv6 address ", "port link-mode route") {
		return ""
	}
	if !looksLikeLayer2Port(header, text) {
		return ""
	}

	switch {
	case hasShutdown(body) || containsAny(text, "port state disable", "administratively down"):
		return "\x1b[38;2;142;147;155m" // soft gray
	case containsAny(text, "switchport mode trunk", "port link-type trunk", "port trunk", "interface trunk", "port-mode trunk"):
		return "\x1b[38;2;105;177;135m" // soft green
	case containsAny(text, "mac-address", "mac binding", "mac-bind"):
		return "\x1b[38;2;174;135;204m" // soft purple
	case containsAny(text, "dot1x", "802.1x", "8021x"):
		return "\x1b[38;2;112;157;205m" // soft blue
	case containsAny(text, "switchport mode access", "port link-type access", "port access vlan", "port default vlan", "switchport"):
		return "\x1b[38;2;211;119;119m" // soft red: access without binding or dot1x
	default:
		return "\x1b[38;2;211;119;119m" // default Layer-2 access port without binding or dot1x
	}
}

func looksLikeLayer2Port(header, text string) bool {
	if containsAny(text, "switchport", "port link-type", "port access", "port trunk", "dot1x", "802.1x", "mac-address", "mac binding", "mac-bind") {
		return true
	}
	name := strings.TrimSpace(strings.TrimPrefix(header, "interface"))
	for _, prefix := range []string{"ethernet", "fastethernet", "gigabitethernet", "tengigabitethernet", "xgigabitethernet", "bridge-aggregation", "port-channel", "eth-trunk", "ge", "xe", "eth", "fa", "gi", "te"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return numberedPortName.MatchString(name)
}

func isInterfaceHeader(line string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "interface ")
}

func isPrompt(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	for _, marker := range []byte{'#', '>'} {
		if i := bytes.IndexByte([]byte(line), marker); i > 0 && !strings.ContainsAny(line[:i], " \t") {
			return true
		}
	}
	return false
}

func isOutputBoundary(line string) bool {
	lower := strings.ToLower(line)
	return isPrompt(line) || isMoreMarker(lower)
}

func isMoreMarker(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(lower, "--more--") || strings.Contains(lower, "<--- more --->")
}

func couldStartInterface(partial []byte) bool {
	plain := strings.ToLower(strings.TrimSpace(stripANSI(string(partial))))
	return plain == "" || strings.HasPrefix("interface ", plain) || strings.HasPrefix(plain, "interface ")
}

func containsAny(text string, fragments ...string) bool {
	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

var shutdownCommand = regexp.MustCompile(`^shutdown(?:\s|$)`)

func hasShutdown(lines []string) bool {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "no shutdown") || strings.HasPrefix(line, "undo shutdown") {
			continue
		}
		if shutdownCommand.MatchString(line) {
			return true
		}
	}
	return false
}

var numberedPortName = regexp.MustCompile(`^\d+(?:/\d+){1,4}(?:\.\d+)?$`)
var ansiSequences = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)

func stripANSI(text string) string {
	return ansiSequences.ReplaceAllString(text, "")
}

