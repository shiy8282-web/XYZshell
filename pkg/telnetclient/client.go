package telnetclient

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	iac byte = 255
	dont byte = 254
	do byte = 253
	wont byte = 252
	will byte = 251
	sb byte = 250
	se byte = 240
	ttype byte = 24
	naws byte = 31
	sgA byte = 3
	echo byte = 1
	ttypeIs byte = 0
)
type optionCommand struct { option, command byte }

type Client struct {
	conn net.Conn
	out *io.PipeReader
	in *input
	mu sync.Mutex
	sent map[optionCommand]bool
	rows, columns uint
	nawsEnabled bool
	closed sync.Once
}
type input struct { conn net.Conn; mu sync.Mutex }

func Connect(host string, port uint16) (*Client,error) {
	if host=="" || port==0 { return nil,errors.New("Telnet host and port are required") }
	address:=net.JoinHostPort(host,strconv.Itoa(int(port)))
	conn,err:=(&net.Dialer{Timeout:15*time.Second}).Dial("tcp",address)
	if err!=nil { return nil,fmt.Errorf("connect to Telnet server %s: %w",address,err) }
	reader,writer:=io.Pipe()
	c:=&Client{conn:conn,out:reader,in:&input{conn:conn},sent:make(map[optionCommand]bool)}
	go c.readLoop(writer)
	return c,nil
}
func (c *Client) Input() io.WriteCloser { return c.in }
func (c *Client) Output() io.Reader { return c.out }
func (c *Client) Resize(rows,columns uint) error {
	if rows==0 || columns==0 { return nil }
	c.mu.Lock()
	c.rows,c.columns=rows,columns
	enabled:=c.nawsEnabled
	c.mu.Unlock()
	if enabled { return c.sendWindowSize(rows,columns) }
	return nil
}
func (c *Client) sendWindowSize(rows,columns uint)error {
	return c.sendSubnegotiation(naws,[]byte{byte(columns>>8),byte(columns),byte(rows>>8),byte(rows)})
}
func (c *Client) Close() error {
	var err error
	c.closed.Do(func(){err=c.conn.Close();_=c.out.Close()})
	return err
}
func (in *input) Write(p []byte)(int,error) {
	in.mu.Lock();defer in.mu.Unlock()
	escaped:=make([]byte,0,len(p))
	for _,b:=range p {escaped=append(escaped,b);if b==iac{escaped=append(escaped,iac)}}
	if _,err:=in.conn.Write(escaped);err!=nil{return 0,err};return len(p),nil
}
func (in *input) Close()error{return in.conn.Close()}
func (c *Client) sendCommand(command,option byte)error {
	c.mu.Lock();defer c.mu.Unlock()
	key:=optionCommand{option:option,command:command}
	if c.sent[key] {return nil}
	c.sent[key]=true
	_,err:=c.conn.Write([]byte{iac,command,option});return err
}
func (c *Client) sendSubnegotiation(option byte,payload []byte)error {
	c.mu.Lock();defer c.mu.Unlock()
	buf:=[]byte{iac,sb,option}
	for _,b:=range payload {buf=append(buf,b);if b==iac{buf=append(buf,iac)}}
	buf=append(buf,iac,se);_,err:=c.conn.Write(buf);return err
}
func (c *Client) negotiate(command,option byte)error {
	switch command {
	case will:
		if option==echo || option==sgA {return c.sendCommand(do,option)}
		return c.sendCommand(dont,option)
	case do:
		if option==naws || option==ttype || option==sgA {
			if err:=c.sendCommand(will,option);err!=nil{return err}
			if option==naws {
				c.mu.Lock()
				c.nawsEnabled=true
				rows,columns:=c.rows,c.columns
				c.mu.Unlock()
				if rows>0 && columns>0 {return c.sendWindowSize(rows,columns)}
			}
			if option==ttype{return c.sendSubnegotiation(ttype,[]byte{ttypeIs,'x','t','e','r','m','-','2','5','6','c','o','l','o','r'})}
			return nil
		}
		return c.sendCommand(wont,option)
	default:return nil
	}
}
func (c *Client) readLoop(writer *io.PipeWriter) {
	defer writer.Close()
	r:=bufio.NewReader(c.conn)
	var data bytes.Buffer
	flush:=func()error{if data.Len()==0{return nil};_,err:=writer.Write(data.Bytes());data.Reset();return err}
	var readErr error
	for {
		b,err:=r.ReadByte();if err!=nil{readErr=err;break}
		if b!=iac {data.WriteByte(b);if data.Len()>=4096{if err:=flush();err!=nil{readErr=err;break}};continue}
		if err:=flush();err!=nil{readErr=err;break}
		command,err:=r.ReadByte();if err!=nil{readErr=err;break}
		switch command {
		case iac:data.WriteByte(iac)
		case will,wont,do,dont:
			option,err:=r.ReadByte();if err!=nil{readErr=err;break}
			if err=c.negotiate(command,option);err!=nil{readErr=err;break}
		case sb:if err:=discardSubnegotiation(r);err!=nil{readErr=err;break}
		}
		if readErr!=nil{break}
	}
	_=flush()
	if readErr!=nil{_=writer.CloseWithError(readErr)}
}
func discardSubnegotiation(r *bufio.Reader)error {
	for {b,err:=r.ReadByte();if err!=nil{return err};if b==iac{next,err:=r.ReadByte();if err!=nil{return err};if next==se{return nil}}}
}
