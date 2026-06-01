package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// parseForwardSpec parses a forwarding specification in the format:
//
//	[bindaddr:]port:host:hostport   (4 or 3 colon-separated fields)
//	[bindaddr:]port:hostport        (2 fields, host defaults to "localhost")
//
// IPv6 addresses must be enclosed in brackets: [::1]:8080:localhost:80
//
// The forms are disambiguated by field count:
//
//	2 fields → port:hostport
//	3 fields → port:host:hostport
//	4 fields → bindaddr:port:host:hostport
func parseForwardSpec(spec string) (bindAddr string, port uint16, destHost string, destPort uint16, err error) {
	parts := splitForwardSpec(spec)
	switch len(parts) {
	case 2:
		// port:hostport  (bind=localhost, desthost=localhost)
		p, parseErr := strconv.ParseUint(parts[0], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid local port %q: %w", parts[0], parseErr)
		}
		dp, parseErr := strconv.ParseUint(parts[1], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid destination port %q: %w", parts[1], parseErr)
		}
		return "localhost", uint16(p), "localhost", uint16(dp), nil

	case 3:
		// port:host:hostport  (bind=localhost)
		p, parseErr := strconv.ParseUint(parts[0], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid local port %q: %w", parts[0], parseErr)
		}
		dp, parseErr := strconv.ParseUint(parts[2], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid destination port %q: %w", parts[2], parseErr)
		}
		return "localhost", uint16(p), parts[1], uint16(dp), nil

	case 4:
		// bindaddr:port:host:hostport
		p, parseErr := strconv.ParseUint(parts[1], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid local port %q: %w", parts[1], parseErr)
		}
		dp, parseErr := strconv.ParseUint(parts[3], 10, 16)
		if parseErr != nil {
			return "", 0, "", 0, fmt.Errorf("invalid destination port %q: %w", parts[3], parseErr)
		}
		return parts[0], uint16(p), parts[2], uint16(dp), nil

	default:
		return "", 0, "", 0, fmt.Errorf("invalid forward spec %q: expected [bindaddr:]port:host:hostport or [bindaddr:]port:hostport", spec)
	}
}

// splitForwardSpec 分割转发规格，支持 IPv6 地址（用 [] 包裹）
func splitForwardSpec(spec string) []string {
	var parts []string
	var current strings.Builder
	inBracket := false

	for _, ch := range spec {
		switch {
		case ch == '[':
			inBracket = true
			continue // 不写入括号字符
		case ch == ']':
			inBracket = false
			continue // 不写入括号字符
		case ch == ':' && !inBracket:
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 || len(spec) > 0 && spec[len(spec)-1] == ':' {
		parts = append(parts, current.String())
	}
	return parts
}

// startLocalForward starts local port forwarding (-L).
// It listens on the local address and forwards connections through the SSH client
// to the specified remote host and port.
func startLocalForward(client *ssh.Client, spec string, verbose bool) (net.Listener, error) {
	bindAddr, localPort, destHost, destPort, err := parseForwardSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("parsing forward spec: %w", err)
	}

	addr := net.JoinHostPort(bindAddr, strconv.FormatUint(uint64(localPort), 10))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", addr, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Local forward: listening on %s -> %s:%d\n", listener.Addr(), destHost, destPort)
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "[sshell] Local forward accept error: %v\n", err)
				}
				return
			}
			go handleLocalForwardConn(client, conn, destHost, destPort, verbose)
		}
	}()

	return listener, nil
}

func handleLocalForwardConn(client *ssh.Client, localConn net.Conn, destHost string, destPort uint16, verbose bool) {
	defer localConn.Close()

	dest := net.JoinHostPort(destHost, strconv.FormatUint(uint64(destPort), 10))
	remoteConn, err := client.Dial("tcp", dest)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Local forward: failed to connect to %s: %v\n", dest, err)
		}
		return
	}
	defer remoteConn.Close()

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Local forward: connected %s -> %s\n", localConn.RemoteAddr(), dest)
	}

	pipeConns(localConn, remoteConn)
}

// startRemoteForward starts remote port forwarding (-R).
// It requests the remote SSH server to listen on the specified port
// and forwards incoming connections back through the SSH tunnel.
func startRemoteForward(client *ssh.Client, spec string, verbose bool) error {
	bindAddr, remotePort, destHost, destPort, err := parseForwardSpec(spec)
	if err != nil {
		return fmt.Errorf("parsing forward spec: %w", err)
	}

	localAddr := net.JoinHostPort(destHost, strconv.FormatUint(uint64(destPort), 10))

	// Request remote forwarding from the SSH server.
	ok, _, err := client.SendRequest("tcpip-forward", true, ssh.Marshal(&remoteForwardRequest{
		BindAddr: bindAddr,
		BindPort: remotePort,
	}))
	if err != nil {
		return fmt.Errorf("requesting remote forward on %s:%d: %w", bindAddr, remotePort, err)
	}
	if !ok {
		return fmt.Errorf("remote forward request denied for %s:%d", bindAddr, remotePort)
	}

	remoteAddr := net.JoinHostPort(bindAddr, strconv.FormatUint(uint64(remotePort), 10))
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Remote forward: listening on remote %s -> local %s\n", remoteAddr, localAddr)
	}

	// 注册 channel handler（整个 client 只注册一次）
	registerRemoteForwardHandler(client)

	// 注册转发映射，让 handler 知道该端口对应哪个本地地址
	remoteForwardMappings.Store(remotePort, remoteForwardEntry{localAddr: localAddr, verbose: verbose})

	// 注册清理函数（在client关闭时清理映射）
	go func() {
		// 等待client关闭
		client.Wait()
		remoteForwardMappings.Delete(remotePort)
		remoteForwardHandlers.Delete(client)
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Remote forward: cleaned up mapping for port %d\n", remotePort)
		}
	}()

	return nil
}

// remoteForwardEntry 存储单个远程端口转发的本地地址和 verbose 标志
type remoteForwardEntry struct {
	localAddr string
	verbose   bool
}

// remoteForwardMappings 存储 remote port → local addr 的映射
var remoteForwardMappings sync.Map // map[uint16]remoteForwardEntry

// remoteForwardHandlers 跟踪哪些 client 已注册过 handler
var remoteForwardHandlers sync.Map // map[*ssh.Client]*sync.Once

// registerRemoteForwardHandler 注册 forwarded-tcpip channel handler（每个 client 只注册一次）
func registerRemoteForwardHandler(client *ssh.Client) {
	val, _ := remoteForwardHandlers.LoadOrStore(client, &sync.Once{})
	once := val.(*sync.Once)
	once.Do(func() {
		go func() {
			chans := client.HandleChannelOpen("forwarded-tcpip")
			for newChan := range chans {
				go handleRemoteForwardChan(newChan)
			}
		}()
	})
}

// handleRemoteForwardChan 处理 forwarded-tcpip channel，查找对应的本地地址
func handleRemoteForwardChan(newChan ssh.NewChannel) {
	// 解析 forwarded-tcpip 请求中的端口
	type forwardedTCPIP struct {
		DestAddr   string
		DestPort   uint32
		OriginAddr string
		OriginPort uint32
	}
	var req forwardedTCPIP
	if err := ssh.Unmarshal(newChan.ExtraData(), &req); err != nil {
		newChan.Reject(ssh.ConnectionFailed, "invalid request")
		return
	}

	localAddrI, ok := remoteForwardMappings.Load(uint16(req.DestPort))
	if !ok {
		newChan.Reject(ssh.ConnectionFailed, "no mapping")
		return
	}
	entry := localAddrI.(remoteForwardEntry)

	handleRemoteForwardConn(newChan, entry.localAddr, entry.verbose)
}

type remoteForwardRequest struct {
	BindAddr string
	BindPort uint16
}

func handleRemoteForwardConn(newChan ssh.NewChannel, localAddr string, verbose bool) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Remote forward: new channel type=%q\n", newChan.ChannelType())
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", localAddr)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Remote forward: failed to connect to local %s: %v\n", localAddr, err)
		}
		newChan.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	channel, requests, err := newChan.Accept()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Remote forward: failed to accept channel: %v\n", err)
		}
		conn.Close()
		return
	}

	// Discard any out-of-band requests on the channel
	go ssh.DiscardRequests(requests)

	// 包装 ssh.Channel 为 net.Conn
	cc := &channelConn{Channel: channel, remoteAddr: conn.RemoteAddr()}
	pipeConns(cc, conn)
	channel.Close()
	conn.Close()
}

// startSOCKS5Proxy starts a SOCKS5 proxy (-D) that forwards connections
// through the SSH tunnel. bindAddr can be "address:port" or just an address
// (defaulting to port 1080).
func startSOCKS5Proxy(client *ssh.Client, bindAddr string, verbose bool) (net.Listener, error) {
	addr := bindAddr
	if addr == "" {
		addr = "localhost"
	}

	// If no port specified, default to 1080
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "1080")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 proxy: listening on %s: %w", addr, err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SOCKS5 proxy: listening on %s\n", listener.Addr())
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "[sshell] SOCKS5 proxy accept error: %v\n", err)
				}
				return
			}
			go handleSOCKS5(client, conn, verbose)
		}
	}()

	return listener, nil
}

// SOCKS5 constants (RFC 1928)
const (
	socks5Version = 0x05

	socks5AuthNone         = 0x00
	socks5AuthNoAcceptable = 0xFF
	socks5CmdConnect       = 0x01
	socks5AddrTypeIPv4     = 0x01
	socks5AddrTypeDomain   = 0x03
	socks5AddrTypeIPv6     = 0x04
	socks5ReplySuccess     = 0x00
	socks5ReplyGeneralFail = 0x01
	socks5ReplyHostUnreach = 0x04
	socks5ReplyCmdNotSupp  = 0x07
)

func handleSOCKS5(client *ssh.Client, conn net.Conn, verbose bool) {
	defer conn.Close()

	// --- Greeting: read version + auth method count ---
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: failed to read greeting: %v\n", err)
		}
		return
	}
	if header[0] != socks5Version {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: unsupported version %d\n", header[0])
		}
		return
	}

	// Read auth methods (we don't care about them, just consume)
	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: failed to read methods: %v\n", err)
		}
		return
	}

	// Reply: no authentication required
	if _, err := conn.Write([]byte{socks5Version, socks5AuthNone}); err != nil {
		return
	}

	// --- Request ---
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: failed to read request: %v\n", err)
		}
		return
	}
	if req[0] != socks5Version {
		sendSOCKS5Reply(conn, socks5ReplyGeneralFail)
		return
	}
	if req[1] != socks5CmdConnect {
		sendSOCKS5Reply(conn, socks5ReplyCmdNotSupp)
		return
	}

	// Parse target address
	targetAddr, err := readSOCKS5Addr(conn, req[3])
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: failed to read address: %v\n", err)
		}
		sendSOCKS5Reply(conn, socks5ReplyGeneralFail)
		return
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: CONNECT %s\n", targetAddr)
	}

	// Open SSH channel to target
	remoteConn, err := client.Dial("tcp", targetAddr)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: dial %s failed: %v\n", targetAddr, err)
		}
		sendSOCKS5Reply(conn, socks5ReplyHostUnreach)
		return
	}
	defer remoteConn.Close()

	if err := sendSOCKS5Reply(conn, socks5ReplySuccess); err != nil {
		return
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SOCKS5: connected %s -> %s\n", conn.RemoteAddr(), targetAddr)
	}

	pipeConns(conn, remoteConn)
}

func readSOCKS5Addr(r io.Reader, addrType byte) (string, error) {
	switch addrType {
	case socks5AddrTypeIPv4:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		port, err := readSOCKS5Port(r)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(net.IP(buf).String(), strconv.FormatUint(uint64(port), 10)), nil

	case socks5AddrTypeIPv6:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		port, err := readSOCKS5Port(r)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(net.IP(buf).String(), strconv.FormatUint(uint64(port), 10)), nil

	case socks5AddrTypeDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(r, lenBuf); err != nil {
			return "", err
		}
		domainLen := int(lenBuf[0])
		if domainLen == 0 {
			return "", fmt.Errorf("empty domain name")
		}
		domain := make([]byte, domainLen)
		if _, err := io.ReadFull(r, domain); err != nil {
			return "", err
		}
		port, err := readSOCKS5Port(r)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(string(domain), strconv.FormatUint(uint64(port), 10)), nil

	default:
		return "", fmt.Errorf("unsupported SOCKS5 address type 0x%02x", addrType)
	}
}

func readSOCKS5Port(r io.Reader) (uint16, error) {
	var port uint16
	if err := binary.Read(r, binary.BigEndian, &port); err != nil {
		return 0, err
	}
	return port, nil
}

func sendSOCKS5Reply(conn net.Conn, reply byte) error {
	// VER, REP, RSV, ATYP, BND.ADDR (4 bytes), BND.PORT (2 bytes)
	_, err := conn.Write([]byte{
		socks5Version, reply, 0x00,
		socks5AddrTypeIPv4,
		0, 0, 0, 0, // 0.0.0.0
		0, 0, // port 0
	})
	return err
}

// closeWriter 是一个可选的半关闭写入接口
type closeWriter interface {
	CloseWrite() error
}

// pipeConns bidirectionally copies between two connections and waits for both
// directions to finish. It properly signals half-close on each side.
func pipeConns(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(b, a)
		if cw, ok := b.(closeWriter); ok {
			cw.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		if cw, ok := a.(closeWriter); ok {
			cw.CloseWrite()
		}
	}()

	wg.Wait()
}

// channelConn 包装 ssh.Channel 使其满足 net.Conn 接口
type channelConn struct {
	ssh.Channel
	remoteAddr net.Addr

	mu       sync.Mutex
	deadline time.Time
	timer    *time.Timer
	closed   bool
}

func (c *channelConn) LocalAddr() net.Addr  { return &net.TCPAddr{} }
func (c *channelConn) RemoteAddr() net.Addr { return c.remoteAddr }

// Write 实现 net.Conn 接口，只在检查 closed 时加锁，不阻塞底层 I/O
func (c *channelConn) Write(data []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.mu.Unlock()
	return c.Channel.Write(data)
}

// Read 实现 net.Conn 接口，只在检查 closed 时加锁，不阻塞底层 I/O
func (c *channelConn) Read(data []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.mu.Unlock()
	return c.Channel.Read(data)
}

// Close 实现 net.Conn 接口，加锁保护并发关闭
func (c *channelConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.Channel == nil {
		return nil
	}
	return c.Channel.Close()
}

func (c *channelConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = t
	c.resetTimer()
	return nil
}

func (c *channelConn) SetReadDeadline(t time.Time) error {
	// ssh.Channel 不区分读写 deadline，统一处理
	return c.SetDeadline(t)
}

func (c *channelConn) SetWriteDeadline(t time.Time) error {
	return c.SetDeadline(t)
}

// resetTimer 在持有 c.mu 的情况下调用，重置超时定时器
func (c *channelConn) resetTimer() {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.deadline.IsZero() {
		return // 无 deadline
	}
	if c.Channel == nil {
		return // channel 未初始化（测试场景）
	}
	if c.closed {
		return // 已关闭
	}
	d := time.Until(c.deadline)
	if d <= 0 {
		// 已过期，立即关闭并清除 deadline
		c.deadline = time.Time{}
		// 使用goroutine避免在持锁时调用Close
		go func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if !c.closed {
				c.closed = true
				c.Channel.Close()
			}
		}()
		return
	}
	c.timer = time.AfterFunc(d, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.closed {
			c.deadline = time.Time{}
			c.timer = nil
			c.closed = true
			c.Channel.Close()
		}
	})
}
