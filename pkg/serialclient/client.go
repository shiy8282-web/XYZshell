package serialclient

import (
	"errors"
	"fmt"
	"io"

	"go.bug.st/serial"
)

type Config struct {
	Port string
	BaudRate int
	DataBits int
	Parity string
	StopBits int
}
type Client struct { port serial.Port }

func ListPorts() ([]string,error) { return serial.GetPortsList() }
func Connect(cfg Config) (*Client,error) {
	if cfg.Port=="" { return nil,errors.New("serial port is required") }
	if cfg.BaudRate<=0 { return nil,errors.New("baud rate must be greater than zero") }
	if cfg.DataBits<5 || cfg.DataBits>8 { return nil,errors.New("data bits must be between 5 and 8") }
	mode:=&serial.Mode{BaudRate:cfg.BaudRate,DataBits:cfg.DataBits}
	switch cfg.Parity {
	case "", "None": mode.Parity=serial.NoParity
	case "Odd": mode.Parity=serial.OddParity
	case "Even": mode.Parity=serial.EvenParity
	default: return nil,fmt.Errorf("unsupported parity %q",cfg.Parity)
	}
	switch cfg.StopBits {
	case 0,1: mode.StopBits=serial.OneStopBit
	case 2: mode.StopBits=serial.TwoStopBits
	default: return nil,errors.New("stop bits must be 1 or 2")
	}
	port,err:=serial.Open(cfg.Port,mode)
	if err!=nil{return nil,fmt.Errorf("open serial port %s: %w",cfg.Port,err)}
	return &Client{port:port},nil
}
func (c *Client) Input() io.WriteCloser { return c.port }
func (c *Client) Output() io.Reader { return c.port }
func (c *Client) Resize(rows,columns uint) error { return nil }
func (c *Client) Close() error { return c.port.Close() }
