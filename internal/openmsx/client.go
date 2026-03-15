// Package openmsx implements the openMSX external control protocol.
//
// On Windows, openMSX listens on a TCP port between 9938-9958.
// Communication uses XML envelopes:
//
//	Send:    <openmsx-control>\n<command>set power on</command>\n
//	Receive: <openmsx-output>\n<reply result="ok">...</reply>\n
package openmsx

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// ── Protocol types ───────────────────────────────────────────────────────────

type reply struct {
	XMLName xml.Name `xml:"reply"`
	Result  string   `xml:"result,attr"`
	Text    string   `xml:",chardata"`
}

type update struct {
	XMLName xml.Name `xml:"update"`
	Type    string   `xml:"type,attr"`
	Machine string   `xml:"machine,attr"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

type logMsg struct {
	XMLName xml.Name `xml:"log"`
	Level   string   `xml:"level,attr"`
	Text    string   `xml:",chardata"`
}

// Reply is the result of a command sent to openMSX.
type Reply struct {
	OK   bool
	Text string
}

// Update is an asynchronous notification from openMSX.
type Update struct {
	Type    string
	Machine string
	Name    string
	Value   string
}

// ── Client ───────────────────────────────────────────────────────────────────

// Client manages a connection to a running openMSX instance.
type Client struct {
	mu       sync.Mutex
	conn     net.Conn
	reader   *bufio.Reader
	pending  chan Reply
	Updates  chan Update
	Logs     chan string
	done     chan struct{}
	doneOnce sync.Once
	lastErr  error
}

// Connect dials openMSX over TCP. openMSX uses ports 9938-9958.
func Connect(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial openMSX at %s: %w", addr, err)
	}

	c := &Client{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		pending: make(chan Reply, 4), // buffer>1: avoids drops if caller is slow
		Updates: make(chan Update, 64),
		Logs:    make(chan string, 64),
		done:    make(chan struct{}),
	}

	// Read opening tag <openmsx-output>
	if err := c.readUntilTag("openmsx-output"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	// Send our opening tag
	if _, err := fmt.Fprint(conn, "<openmsx-control>\n"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send control tag: %w", err)
	}

	go c.readLoop()

	// Request LED + hardware updates
	c.enableUpdates()

	return c, nil
}

// Disconnect closes the connection gracefully.
func (c *Client) Disconnect() {
	c.mu.Lock()
	if c.conn != nil {
		fmt.Fprint(c.conn, "</openmsx-control>\n")
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	c.doneOnce.Do(func() { close(c.done) })
}

// Done returns a channel closed when the connection drops or Disconnect is called.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// IsConnected returns true if the TCP connection is active.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && c.lastErr == nil
}

// Send transmits a Tcl command and waits for the reply.
func (c *Client) Send(cmd string) Reply {
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return Reply{OK: false, Text: "not connected"}
	}
	xmlCmd := fmt.Sprintf("<command>%s</command>\n", xmlEscape(cmd))
	_, err := fmt.Fprint(c.conn, xmlCmd)
	c.mu.Unlock()

	if err != nil {
		return Reply{OK: false, Text: err.Error()}
	}

	select {
	case r := <-c.pending:
		return r
	case <-time.After(10 * time.Second):
		return Reply{OK: false, Text: "timeout waiting for reply"}
	case <-c.done:
		return Reply{OK: false, Text: "disconnected"}
	}
}

// ── Power & Control helpers ──────────────────────────────────────────────────

func (c *Client) PowerOn()   { c.Send("set power on") }
func (c *Client) PowerOff()  { c.Send("set power off") }
func (c *Client) Reset()     { c.Send("reset") }
func (c *Client) Pause()     { c.Send("set pause on") }
func (c *Client) Unpause()   { c.Send("set pause off") }
func (c *Client) RewindOff() { c.Send("reverse disable") }
func (c *Client) RewindOn()  { c.Send("reverse enable") }
func (c *Client) Throttle(on bool) {
	v := "on"
	if !on {
		v = "off"
	}
	c.Send("set throttle " + v)
}

// LoadROM inserts a ROM into slot A.
func (c *Client) LoadROM(path string) Reply {
	return c.Send(fmt.Sprintf("carta {%s}", path))
}

// LoadDisk inserts a disk image into drive A.
func (c *Client) LoadDisk(path string) Reply {
	return c.Send(fmt.Sprintf("diska {%s}", path))
}

// LoadCassette inserts a cassette image.
func (c *Client) LoadCassette(path string) Reply {
	return c.Send(fmt.Sprintf("cassetteplayer insert {%s}", path))
}

// SaveState saves the current machine state.
func (c *Client) SaveState(name string) Reply {
	return c.Send(fmt.Sprintf("savestate {%s}", name))
}

// LoadState restores a previously saved state.
func (c *Client) LoadState(name string) Reply {
	return c.Send(fmt.Sprintf("loadstate {%s}", name))
}

// ListSaveStates returns the list of saved states.
func (c *Client) ListSaveStates() ([]string, error) {
	r := c.Send("savestate list")
	if !r.OK {
		return nil, fmt.Errorf("savestate list: %s", r.Text)
	}
	var states []string
	for _, s := range strings.Fields(r.Text) {
		if s != "" {
			states = append(states, s)
		}
	}
	return states, nil
}

// GetMachine returns the current machine name.
func (c *Client) GetMachine() string {
	r := c.Send("machine")
	if r.OK {
		return strings.TrimSpace(r.Text)
	}
	return "unknown"
}

// Screenshot captures the screen to a file.
func (c *Client) Screenshot(path string) Reply {
	if path == "" {
		return c.Send("screenshot")
	}
	return c.Send(fmt.Sprintf("screenshot {%s}", path))
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (c *Client) enableUpdates() {
	c.Send("update enable led")
	c.Send("update enable hardware")
	c.Send("update enable media")
}

func (c *Client) readLoop() {
	defer func() {
		c.mu.Lock()
		c.lastErr = io.EOF
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
		// Signal done so any blocked Send() returns immediately
		c.doneOnce.Do(func() { close(c.done) })
		// Drain pending so no Send() blocks forever
		select {
		case c.pending <- Reply{OK: false, Text: "connection lost"}:
		default:
		}
	}()

	dec := xml.NewDecoder(c.reader)
	for {
		tok, err := dec.Token()
		if err != nil {
			log.Printf("openMSX readLoop: %v", err)
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "reply":
				var r reply
				if err := dec.DecodeElement(&r, &t); err == nil {
					select {
					case c.pending <- Reply{OK: r.Result == "ok", Text: strings.TrimSpace(r.Text)}:
					case <-c.done:
						return
					}
				}
			case "update":
				var u update
				if err := dec.DecodeElement(&u, &t); err == nil {
					select {
					case c.Updates <- Update{Type: u.Type, Machine: u.Machine, Name: u.Name, Value: strings.TrimSpace(u.Value)}:
					default:
					}
				}
			case "log":
				var l logMsg
				if err := dec.DecodeElement(&l, &t); err == nil {
					select {
					case c.Logs <- fmt.Sprintf("[%s] %s", l.Level, strings.TrimSpace(l.Text)):
					default:
					}
				}
			}
		}
	}
}

func (c *Client) readUntilTag(tag string) error {
	buf := make([]byte, 0, 256)
	b := make([]byte, 1)
	target := "<" + tag + ">"
	for {
		if _, err := c.reader.Read(b); err != nil {
			return err
		}
		buf = append(buf, b[0])
		if strings.Contains(string(buf), target) {
			return nil
		}
		if len(buf) > 4096 {
			return fmt.Errorf("handshake tag not found")
		}
	}
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// ── Port Scanner ─────────────────────────────────────────────────────────────

// ScanPorts tries to find a running openMSX instance on localhost ports 9938–9958.
func ScanPorts() (string, error) {
	for port := 9938; port <= 9958; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return addr, nil
		}
	}
	return "", fmt.Errorf("no openMSX instance found on ports 9938-9958")
}
