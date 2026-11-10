package main

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	text := cleanScrollbackText(data)
	if truncated {
		text = "[较早的输出已因缓冲区限制被移除]\n" + text
	}
	return text
}

// Segments rebuilds interface colors for the history view and drops paging
// prompts and terminal control residue that are meaningful only to the live
// terminal emulator.
func (b *scrollbackBuffer) Segments() []widget.RichTextSegment {
	text := b.Text()
	colored, _ := io.ReadAll(newInterfaceColorReader(bytes.NewBufferString(text)))
	return interfaceColorSegments(colored)
}

var morePromptPattern = regexp.MustCompile("(?i)\\r*(?:<\\s*-{2,}\\s*more\\s*-{2,}\\s*>|-{2,}\\s*more\\s*-{2,})\\r*")
var historyANSIColorPattern = regexp.MustCompile("\\x1b\\[38;2;(\\d+);(\\d+);(\\d+)m|\\x1b\\[0m")

func cleanScrollbackText(data []byte) string {
	text := stripANSI(string(data))
	// Backspace and other C0 control bytes can leak from pagers when the
	// device redraws its --More-- prompt. Remove them before matching markers.
	var printable strings.Builder
	for _, r := range text {
		if r == '\n' || r == '\r' || r == '\t' || r >= 0x20 {
			printable.WriteRune(r)
		}
	}
	text = morePromptPattern.ReplaceAllString(printable.String(), "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func interfaceColorSegments(data []byte) []widget.RichTextSegment {
	segments := make([]widget.RichTextSegment, 0, 32)
	current := theme.ColorNameForeground
	last := 0
	for _, match := range historyANSIColorPattern.FindAllStringSubmatchIndex(string(data), -1) {
		appendRichTextSegment(&segments, string(data[last:match[0]]), current)
		if match[2] >= 0 {
			r, _ := strconv.Atoi(string(data[match[2]:match[3]]))
			g, _ := strconv.Atoi(string(data[match[4]:match[5]]))
			b, _ := strconv.Atoi(string(data[match[6]:match[7]]))
			current = interfaceColorName(r, g, b)
		} else {
			current = theme.ColorNameForeground
		}
		last = match[1]
	}
	appendRichTextSegment(&segments, string(data[last:]), current)
	return segments
}

func appendRichTextSegment(segments *[]widget.RichTextSegment, text string, colorName fyne.ThemeColorName) {
	if text == "" {
		return
	}
	*segments = append(*segments, &widget.TextSegment{
		Text:  text,
		Style: widget.RichTextStyle{Inline: true, ColorName: colorName},
	})
}

func interfaceColorName(r, g, b int) fyne.ThemeColorName {
	switch {
	case r == 142 && g == 147 && b == 155:
		return interfaceShutdownColor
	case r == 105 && g == 177 && b == 135:
		return interfaceTrunkColor
	case r == 174 && g == 135 && b == 204:
		return interfaceMACColor
	case r == 112 && g == 157 && b == 205:
		return interfaceDot1XColor
	case r == 211 && g == 119 && b == 119:
		return interfaceAccessColor
	default:
		return theme.ColorNameForeground
	}
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
