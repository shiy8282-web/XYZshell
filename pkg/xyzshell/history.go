package xyzshell

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxSavedConnections = 30

// ConnectionProfile contains connection settings only. It intentionally has no
// password field so saved connection history never stores credentials.
type ConnectionProfile struct {
	Protocol      string
	Host          string
	Port          uint16
	User          string
	SerialPort    string
	BaudRate      int
	DataBits      int
	Parity        string
	StopBits      int
	LastConnected time.Time
}

func (p ConnectionProfile) DisplayName() string {
	switch p.Protocol {
	case "SSH":
		return fmt.Sprintf("SSH %s@%s:%d", p.User, p.Host, p.Port)
	case "Telnet":
		return fmt.Sprintf("Telnet %s:%d", p.Host, p.Port)
	case "Serial":
		return fmt.Sprintf("Serial %s (%d baud, %d data, %s parity, %d stop)", p.SerialPort, p.BaudRate, p.DataBits, p.Parity, p.StopBits)
	default:
		return p.Protocol
	}
}

func (p ConnectionProfile) key() string {
	return strings.Join([]string{
		p.Protocol, p.Host, strconv.Itoa(int(p.Port)), p.User,
		p.SerialPort, strconv.Itoa(p.BaudRate), strconv.Itoa(p.DataBits),
		p.Parity, strconv.Itoa(p.StopBits),
	}, "\x00")
}

func LoadConnectionHistory() ([]ConnectionProfile, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read connection history: %w", err)
	}
	var profiles []ConnectionProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parse connection history: %w", err)
	}
	return profiles, nil
}

// RememberConnection saves a successful endpoint, most recent first. It never
// persists passwords or other authentication secrets.
func RememberConnection(profile ConnectionProfile) ([]ConnectionProfile, error) {
	if profile.Protocol == "" {
		return nil, errors.New("connection protocol is required")
	}
	profile.LastConnected = time.Now()
	profiles, err := LoadConnectionHistory()
	if err != nil {
		return nil, err
	}
	key := profile.key()
	updated := make([]ConnectionProfile, 0, maxSavedConnections)
	updated = append(updated, profile)
	for _, old := range profiles {
		if old.key() == key {
			continue
		}
		if len(updated) == maxSavedConnections {
			break
		}
		updated = append(updated, old)
	}
	path, err := historyPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create connection history directory: %w", err)
	}
	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode connection history: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary connection history: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("protect connection history file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write connection history: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close connection history file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return nil, fmt.Errorf("replace connection history: %w", err)
	}
	return updated, nil
}

func historyPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user configuration directory: %w", err)
	}
	return filepath.Join(configDir, "XYZshell", "connections.json"), nil
}
