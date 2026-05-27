package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type args struct {
	host    string
	port    uint16
	user    string
	auth    string
	alive   uint32
	verbose bool
}

func parseArgs() args {
	var a args
	argv := os.Args[1:]
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "-p":
			if i+1 < len(argv) {
				fmt.Sscanf(argv[i+1], "%d", &a.port)
				i++
			}
		case "-u":
			if i+1 < len(argv) {
				a.user = argv[i+1]
				i++
			}
		case "-a":
			if i+1 < len(argv) {
				a.auth = argv[i+1]
				i++
			}
		case "-k":
			if i+1 < len(argv) {
				fmt.Sscanf(argv[i+1], "%d", &a.alive)
				i++
			}
		case "-v":
			a.verbose = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			if argv[i][0] != '-' && a.host == "" {
				a.host = argv[i]
			}
		}
	}

	if a.port == 0 {
		a.port = 22
	}
	if a.alive == 0 {
		a.alive = 30
	}

	if a.host == "" || a.user == "" {
		printUsage()
		os.Exit(1)
	}
	return a
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: sshell -u <user> <host> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -p <port>     SSH port (default: 22)")
	fmt.Fprintln(os.Stderr, "  -u <user>     SSH username (required)")
	fmt.Fprintln(os.Stderr, "  -a <auth>     Password or path to private key file")
	fmt.Fprintln(os.Stderr, "  -k <seconds>  Keep-alive interval (default: 30)")
	fmt.Fprintln(os.Stderr, "  -v            Verbose output")
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func findAuth(a args) ([]ssh.AuthMethod, error) {
	if a.auth != "" {
		if _, err := os.Stat(a.auth); err == nil {
			key, err := os.ReadFile(a.auth)
			if err != nil {
				return nil, fmt.Errorf("read key: %w", err)
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				pw, perr := readPassword("Key passphrase: ")
				if perr != nil {
					return nil, fmt.Errorf("parse key: %w", err)
				}
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(pw))
				if err != nil {
					return nil, fmt.Errorf("parse key: %w", err)
				}
			}
			if a.verbose {
				fmt.Fprintf(os.Stderr, "[sshell] Auth: key file (%s)\n", a.auth)
			}
			return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
		}
		if a.verbose {
			fmt.Fprintln(os.Stderr, "[sshell] Auth: password")
		}
		return []ssh.AuthMethod{ssh.Password(a.auth)}, nil
	}

	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		p := filepath.Join(home, ".ssh", name)
		key, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue
		}
		if a.verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Auth: key (%s)\n", p)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}

	pw, err := readPassword(fmt.Sprintf("%s@%s's password: ", a.user, a.host))
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return []ssh.AuthMethod{ssh.Password(pw)}, nil
}

func connect(a args) (*ssh.Session, error) {
	addr := net.JoinHostPort(a.host, fmt.Sprintf("%d", a.port))
	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Connecting to %s...\n", addr)
	}

	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(time.Duration(a.alive) * time.Second)
	}

	authMethods, err := findAuth(a)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            a.user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Handshake OK.")
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("session: %w", err)
	}

	return session, nil
}

func interactiveShell(session *ssh.Session, a args) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		return fmt.Errorf("pty: %w", err)
	}

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	if err := session.Shell(); err != nil {
		return fmt.Errorf("shell: %w", err)
	}
	if a.verbose {
		fmt.Fprintln(os.Stderr, "[sshell] Shell started.")
	}

	// Raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Forward SSH stdout/stderr to local
	outDone := make(chan struct{})
	go func() {
		io.Copy(os.Stdout, stdout)
		close(outDone)
	}()
	go func() {
		io.Copy(os.Stderr, stderr)
	}()

	// Read stdin and forward to SSH
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				stdin.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for remote shell to close
	<-outDone
	return session.Wait()
}

func main() {
	a := parseArgs()

	session, err := connect(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	if err := interactiveShell(session, a); err != nil {
		if err.Error() != "exit status 0" {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
		}
	}
}
