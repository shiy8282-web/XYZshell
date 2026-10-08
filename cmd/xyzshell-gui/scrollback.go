package main

import (
	"io"
	"sync"
	"time"
)

const maxScrollbackBytes = 2 << 20

// scrollbackReader preserves readable output while passing the original stream
// unchanged to the terminal emulator.
type scrollbackReader struct {
	source  io.Reader
	updated func(string)
	mu      sync.Mutex
	text    []byte
	state   uint8
	lastUI  time.Time
}

func newScrollbackReader(source io.Reader, updated func(string)) *scrollbackReader {
	return &scrollbackReader{source: source, updated: updated}
}

func (r *scrollbackReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n == 0 {
		return n, err
	}

	r.mu.Lock()
	for _, b := range p[:n] {
		r.capture(b)
	}
	if len(r.text) > maxScrollbackBytes {
		cut := len(r.text) - maxScrollbackBytes
		for cut < len(r.text) && r.text[cut] != '\n' {
			cut++
		}
		r.text = append(r.text[:0], r.text[cut:]...)
	}
	if r.updated != nil && time.Since(r.lastUI) >= 150*time.Millisecond {
		r.lastUI = time.Now()
		text := string(r.text)
		go r.updated(text)
	}
	r.mu.Unlock()
	return n, err
}

// capture strips terminal control sequences from a copy for the readable
// history pane. The original bytes are still delivered to the live terminal.
func (r *scrollbackReader) capture(b byte) {
	switch r.state {
	case 1: // ESC
		switch b {
		case '[':
			r.state = 2 // CSI
		case ']':
			r.state = 3 // OSC
		default:
			r.state = 0
		}
	case 2: // CSI, ending with a byte in @..~
		if b >= 0x40 && b <= 0x7e {
			r.state = 0
		}
	case 3: // OSC, ending with BEL or ESC \\
		if b == 0x07 {
			r.state = 0
		} else if b == 0x1b {
			r.state = 4
		}
	case 4: // possible OSC string terminator
		if b == '\\' {
			r.state = 0
		} else if b != 0x1b {
			r.state = 3
		}
	default:
		switch b {
		case 0x1b:
			r.state = 1
		case '\n', '\t':
			r.text = append(r.text, b)
		case '\r':
			// CR is used by terminal prompts to return to the start of a line.
		case 0x08, 0x7f:
			for len(r.text) > 0 && r.text[len(r.text)-1]&0xc0 == 0x80 {
				r.text = r.text[:len(r.text)-1]
			}
			if len(r.text) > 0 && r.text[len(r.text)-1] != '\n' {
				r.text = r.text[:len(r.text)-1]
			}
		default:
			if b >= 0x20 {
				r.text = append(r.text, b)
			}
		}
	}
}

