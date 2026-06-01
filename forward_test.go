package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestParseForwardSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		bindAddr string
		port     uint16
		destHost string
		destPort uint16
		wantErr  bool
	}{
		{
			name:     "two fields: port:hostport",
			spec:     "8080:80",
			bindAddr: "localhost",
			port:     8080,
			destHost: "localhost",
			destPort: 80,
		},
		{
			name:     "three fields: port:host:hostport",
			spec:     "8080:example.com:80",
			bindAddr: "localhost",
			port:     8080,
			destHost: "example.com",
			destPort: 80,
		},
		{
			name:     "four fields: bindaddr:port:host:hostport",
			spec:     "0.0.0.0:9090:internal.local:3306",
			bindAddr: "0.0.0.0",
			port:     9090,
			destHost: "internal.local",
			destPort: 3306,
		},
		{
			name:     "two fields with high port",
			spec:     "65535:65534",
			bindAddr: "localhost",
			port:     65535,
			destHost: "localhost",
			destPort: 65534,
		},
		{
			name:     "three fields with IP host",
			spec:     "3000:192.168.1.1:5432",
			bindAddr: "localhost",
			port:     3000,
			destHost: "192.168.1.1",
			destPort: 5432,
		},
		{
			name:     "four fields with localhost bind",
			spec:     "127.0.0.1:8080:remote.host:443",
			bindAddr: "127.0.0.1",
			port:     8080,
			destHost: "remote.host",
			destPort: 443,
		},
		{
			name:     "two fields port 1",
			spec:     "1:2",
			bindAddr: "localhost",
			port:     1,
			destHost: "localhost",
			destPort: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindAddr, port, destHost, destPort, err := parseForwardSpec(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseForwardSpec(%q) expected error, got nil", tt.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseForwardSpec(%q) unexpected error: %v", tt.spec, err)
			}
			if bindAddr != tt.bindAddr {
				t.Errorf("bindAddr = %q, want %q", bindAddr, tt.bindAddr)
			}
			if port != tt.port {
				t.Errorf("port = %d, want %d", port, tt.port)
			}
			if destHost != tt.destHost {
				t.Errorf("destHost = %q, want %q", destHost, tt.destHost)
			}
			if destPort != tt.destPort {
				t.Errorf("destPort = %d, want %d", destPort, tt.destPort)
			}
		})
	}
}

func TestParseForwardSpecErrors(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{name: "empty string", spec: ""},
		{name: "single field", spec: "8080"},
		{name: "too many fields", spec: "a:b:c:d:e"},
		{name: "invalid local port two fields", spec: "abc:80"},
		{name: "invalid dest port two fields", spec: "8080:xyz"},
		{name: "invalid local port three fields", spec: "abc:example.com:80"},
		{name: "invalid dest port three fields", spec: "8080:example.com:xyz"},
		{name: "invalid local port four fields", spec: "0.0.0.0:abc:example.com:80"},
		{name: "invalid dest port four fields", spec: "0.0.0.0:8080:example.com:xyz"},
		{name: "port overflow", spec: "99999:80"},
		{name: "negative port", spec: "-1:80"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, err := parseForwardSpec(tt.spec)
			if err == nil {
				t.Fatalf("parseForwardSpec(%q) expected error, got nil", tt.spec)
			}
		})
	}
}

func TestReadSOCKS5Port(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  uint16
	}{
		{name: "port 80", input: []byte{0x00, 0x50}, want: 80},
		{name: "port 443", input: []byte{0x01, 0xBB}, want: 443},
		{name: "port 65535", input: []byte{0xFF, 0xFF}, want: 65535},
		{name: "port 1080", input: []byte{0x04, 0x38}, want: 1080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			got, err := readSOCKS5Port(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("empty reader", func(t *testing.T) {
		r := bytes.NewReader(nil)
		_, err := readSOCKS5Port(r)
		if err == nil {
			t.Fatal("expected error from empty reader")
		}
	})
}

func TestReadSOCKS5AddrIPv4(t *testing.T) {
	// 1.2.3.4:8080 → 01 02 03 04 1F 90
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x1F, 0x90}
	r := bytes.NewReader(data)
	addr, err := readSOCKS5Addr(r, socks5AddrTypeIPv4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "1.2.3.4:8080"; addr != want {
		t.Errorf("got %q, want %q", addr, want)
	}
}

func TestReadSOCKS5AddrIPv6(t *testing.T) {
	// ::1:443 → 15 zero bytes, 0x01, then port 0x01BB
	data := make([]byte, 16+2)
	data[15] = 0x01       // ::1
	data[16] = 0x01       // port high
	data[17] = 0xBB       // port low = 443
	r := bytes.NewReader(data)
	addr, err := readSOCKS5Addr(r, socks5AddrTypeIPv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "[::1]:443"; addr != want {
		t.Errorf("got %q, want %q", addr, want)
	}
}

func TestReadSOCKS5AddrDomain(t *testing.T) {
	domain := []byte("example.com")
	// len + domain + port
	data := make([]byte, 1+len(domain)+2)
	data[0] = byte(len(domain))
	copy(data[1:], domain)
	data[1+len(domain)] = 0x00 // port high
	data[2+len(domain)] = 0x50 // port low = 80

	r := bytes.NewReader(data)
	addr, err := readSOCKS5Addr(r, socks5AddrTypeDomain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "example.com:80"; addr != want {
		t.Errorf("got %q, want %q", addr, want)
	}
}

func TestReadSOCKS5AddrUnsupportedType(t *testing.T) {
	r := bytes.NewReader([]byte{})
	_, err := readSOCKS5Addr(r, 0x02)
	if err == nil {
		t.Fatal("expected error for unsupported address type")
	}
}

func TestSendSOCKS5Reply(t *testing.T) {
	var buf bytes.Buffer
	// Use a net.Conn wrapper for testing
	conn := &bufConn{buf: &buf}
	err := sendSOCKS5Reply(conn, socks5ReplySuccess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reply := buf.Bytes()
	if len(reply) != 10 {
		t.Fatalf("reply length = %d, want 10", len(reply))
	}
	if reply[0] != socks5Version {
		t.Errorf("version = 0x%02x, want 0x%02x", reply[0], socks5Version)
	}
	if reply[1] != socks5ReplySuccess {
		t.Errorf("reply = 0x%02x, want 0x%02x", reply[1], socks5ReplySuccess)
	}
	if reply[2] != 0x00 {
		t.Errorf("reserved = 0x%02x, want 0x00", reply[2])
	}
	if reply[3] != socks5AddrTypeIPv4 {
		t.Errorf("addr type = 0x%02x, want 0x%02x", reply[3], socks5AddrTypeIPv4)
	}
}

// bufConn is a minimal net.Conn implementation backed by a bytes.Buffer for testing.
type bufConn struct {
	buf *bytes.Buffer
}

func (c *bufConn) Read(b []byte) (int, error)                          { return c.buf.Read(b) }
func (c *bufConn) Write(b []byte) (int, error)                         { return c.buf.Write(b) }
func (c *bufConn) Close() error                                         { return nil }
func (c *bufConn) LocalAddr() net.Addr                                  { return &net.TCPAddr{} }
func (c *bufConn) RemoteAddr() net.Addr                                 { return &net.TCPAddr{} }
func (c *bufConn) SetDeadline(_ time.Time) error                        { return nil }
func (c *bufConn) SetReadDeadline(_ time.Time) error                    { return nil }
func (c *bufConn) SetWriteDeadline(_ time.Time) error                   { return nil }
