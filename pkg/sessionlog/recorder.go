package sessionlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"xyzshell/pkg/xyzshell"
)

// Recorder stores raw terminal input and output for one session.
type Recorder struct {
	file *os.File
	path string
	mu sync.Mutex
}

func New(profile xyzshell.ConnectionProfile) (*Recorder, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find log directory: %w", err)
	}
	dir := filepath.Join(configDir, "XYZshell", "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	base := safeFileName(profile.DisplayName())
	if base == "" {
		base = "session"
	}
	stamp := time.Now().Format("20060102-150405")
	for suffix := 0; ; suffix++ {
		name := fmt.Sprintf("%s-%s.log", stamp, base)
		if suffix > 0 {
			name = fmt.Sprintf("%s-%s-%d.log", stamp, base, suffix)
		}
		path := filepath.Join(dir, name)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create session log: %w", err)
		}
		recorder := &Recorder{file: file, path: path}
		if err := recorder.writeHeader(profile); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return nil, err
		}
		return recorder, nil
	}
}

func (r *Recorder) Path() string { return r.path }

func (r *Recorder) WrapInput(dst io.WriteCloser) io.WriteCloser {
	return &recordingWriter{dst: dst, recorder: r, direction: "INPUT"}
}

func (r *Recorder) WrapOutput(src io.Reader) io.Reader {
	return &recordingReader{src: src, recorder: r, direction: "OUTPUT"}
}

func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

func (r *Recorder) writeHeader(profile xyzshell.ConnectionProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := fmt.Fprintf(r.file, "XYZshell session log\nStarted: %s\nConnection: %s\n\n",
		time.Now().Format(time.RFC3339), profile.DisplayName())
	if err != nil {
		return fmt.Errorf("write session log header: %w", err)
	}
	return nil
}

func (r *Recorder) record(direction string, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return os.ErrClosed
	}
	if _, err := fmt.Fprintf(r.file, "\n[%s] %s\n", time.Now().Format(time.RFC3339), direction); err != nil {
		return err
	}
	_, err := r.file.Write(data)
	return err
}

type recordingWriter struct {
	dst io.WriteCloser
	recorder *Recorder
	direction string
}

func (w *recordingWriter) Write(data []byte) (int, error) {
	n, err := w.dst.Write(data)
	if n > 0 {
		if logErr := w.recorder.record(w.direction, data[:n]); logErr != nil && err == nil {
			err = logErr
		}
	}
	return n, err
}

func (w *recordingWriter) Close() error { return w.dst.Close() }

type recordingReader struct {
	src io.Reader
	recorder *Recorder
	direction string
}

func (r *recordingReader) Read(data []byte) (int, error) {
	n, err := r.src.Read(data)
	if n > 0 {
		if logErr := r.recorder.record(r.direction, data[:n]); logErr != nil && err == nil {
			err = logErr
		}
	}
	return n, err
}

func safeFileName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune("<>:\"/\\|?*", r) {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
		if b.Len() >= 96 {
			break
		}
	}
	return strings.Trim(b.String(), " ._")
}
