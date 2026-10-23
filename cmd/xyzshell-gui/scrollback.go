package main

import (
	"io"
	"strings"
	"sync"
)

const maxScrollbackBytes = 2 << 20

// scrollbackBuffer keeps a bounded copy of the terminal stream so the user
// can review earlier output without leaving the live terminal screen.
type scrollbackBuffer struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (b *scrollbackBuffer) Reset() {
	b.mu.Lock()
	b.data = nil
	b.truncated = false
	b.mu.Unlock()
}

func (b *scrollbackBuffer) Append(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, data...)
	if len(b.data) > maxScrollbackBytes {
		drop := len(b.data) - maxScrollbackBytes
		if newline := bytesIndexByte(b.data[drop:], '\n'); newline >= 0 {
			drop += newline + 1
		}
		b.data = append([]byte(nil), b.data[drop:]...)
		b.truncated = true
	}
}

func (b *scrollbackBuffer) Text() string {
	b.mu.Lock()
	data := append([]byte(nil), b.data...)
	truncated := b.truncated
	b.mu.Unlock()
	text := stripANSI(string(data))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if truncated {
		text = "[较早的输出已因缓冲区限制被移除]\n" + text
	}
	return text
}

type scrollbackReader struct {
	source io.Reader
	buffer *scrollbackBuffer
}

func (r *scrollbackReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n > 0 {
		r.buffer.Append(p[:n])
	}
	return n, err
}

func bytesIndexByte(data []byte, value byte) int {
	for i, b := range data {
		if b == value {
			return i
		}
	}
	return -1
}
