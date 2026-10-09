package xyzshell

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Config describes an SSH endpoint. Passwords are deliberately kept outside
// this structure so a saved profile never contains secret material.
type Config struct {
	Name string `json:"name,omitempty"`
	Host string `json:"host"`
	Port uint16 `json:"port"`
	User string `json:"user"`
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))) }

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" || strings.ContainsAny(c.Host, " \t\r\n") {
		return errors.New("host is required")
	}
	if c.Port == 0 {
		return errors.New("port must be between 1 and 65535")
	}
	if net.ParseIP(strings.Trim(c.Host, "[]")) == nil && strings.ContainsAny(c.Host, "/\\") {
		return fmt.Errorf("invalid host %q", c.Host)
	}
	return nil
}
