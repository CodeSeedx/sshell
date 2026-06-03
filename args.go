package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// 版本号，编译时通过 ldflags 注入: -X main.version=x.x.x
var version = "dev"

// jumpHost 表示一个跳板机
type jumpHost struct {
	Host string `json:"host"`
	Port uint16 `json:"port,omitempty"`
	User string `json:"user,omitempty"`
}

type args struct {
	host    string
	port    uint16
	user    string
	auth    string
	alive   uint32
	verbose bool
	cmd     string // 远程命令，为空则进入交互 shell

	// SSH Agent
	noAgent bool // --no-agent: 禁用 SSH Agent

	// Agent Forwarding
	agentForward bool // -A / --agent-forward: 启用 SSH Agent 转发

	// ProxyJump（支持多跳板机：-J host1,host2,host3）
	proxyJump     string   // -J jump_host（原始字符串）
	proxyJumpPort uint16   // 仅用于单跳时的 -J host:port
	proxyJumpUser string   // 仅用于单跳时的 -J user@host
	proxyJumps    []jumpHost // 解析后的跳板机列表（多跳时使用）

	// Remote edit
	editFile string // --edit /path/to/remote/file

	// Compression
	compress bool // -C

	// Port forwarding
	localFwd  []string // -L [bind:]port:host:hostport (可重复)
	remoteFwd []string // -R [bind:]port:host:hostport (可重复)
	socksPort string   // -D [bind:]port

	// SCP / SFTP 文件传输
	scpPut string // --put local:remote
	scpGet string // --get remote:local
	sftp   bool   // --sftp: use SFTP protocol instead of SCP for --put/--get

	// Session 日志
	logFile string // --log file

	// Multi-host
	hosts []string // 从 host 字段按逗号拆分

	// Bookmarks
	save   string // --save name
	list   bool   // --list
	delete string // --delete name

	// Reconnect
	reconnect    bool // --reconnect
	reconnectMax int  // --reconnect-max N (默认 3)

	// 命令行显式设置标记（用于正确实现配置优先级）
	cliPort bool // -p 被显式指定
	cliUser bool // -u 被显式指定
	cliAuth bool // -a 被显式指定
	cliAlive bool // -k 被显式指定

	// 安全选项
	insecureHostKey bool // --insecure-host-key: 允许跳过主机密钥校验
}

func parseArgs() args {
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return a
}

func parseArgsVerbose() (args, error) {
	c := loadConfig()
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		return a, err
	}
	applyConfig(&a, c)
	a = finalizeArgs(a)
	// 应用配置后，重新检查必填参数
	// --list 和 --delete 不需要 host
	if a.host == "" && !a.list && a.delete == "" {
		return a, fmt.Errorf("Usage: sshell -u <user> <host> [options] [command]")
	}
	if a.host != "" && a.user == "" {
		return a, fmt.Errorf("Usage: sshell -u <user> <host> [options] [command]")
	}
	return a, nil
}

// parseArgsWithConfig 解析参数并应用配置文件
func parseArgsWithConfig(argv []string) (args, error) {
	c := loadConfig()
	a, err := parseArgsFrom(argv)
	if err != nil {
		return a, err
	}
	applyConfig(&a, c)
	a = finalizeArgs(a)
	// 应用配置后，重新检查必填参数
	// --list 和 --delete 不需要 host
	if a.host == "" && !a.list && a.delete == "" {
		return a, fmt.Errorf("Usage: sshell -u <user> <host> [options] [command]")
	}
	if a.host != "" && a.user == "" {
		return a, fmt.Errorf("Usage: sshell -u <user> <host> [options] [command]")
	}
	return a, nil
}

// finalizeArgs 解析完成后处理派生字段
func finalizeArgs(a args) args {
	// 拆分多主机
	if a.host != "" {
		a.hosts = strings.Split(a.host, ",")
		// 过滤空字符串
		var validHosts []string
		for i := range a.hosts {
			host := strings.TrimSpace(a.hosts[i])
			if host != "" {
				validHosts = append(validHosts, host)
			}
		}
		a.hosts = validHosts
	}
	// reconnect 默认值
	if a.reconnect && a.reconnectMax == 0 {
		a.reconnectMax = 3
	}
	// 应用目标主机的 SSH config（仅第一个主机，多主机各自在 runOnHost 中处理）
	if len(a.hosts) == 1 && a.hosts[0] != "" {
		a = applySSHConfigTarget(a, a.hosts[0])
	}
	return a
}

// applySSHConfigTarget 将 SSH config 中目标主机的配置应用到 args
// 优先级: 命令行 > SSH config > sshell config > 默认值
func applySSHConfigTarget(a args, hostAlias string) args {
	cfg := loadSSHConfig(hostAlias)
	if cfg == nil {
		return a
	}
	// 仅当命令行未显式指定时才使用 SSH config 的值
	if cfg.HostName != "" && a.host == hostAlias {
		a.host = cfg.HostName
		// 更新 hosts 列表
		a.hosts = []string{a.host}
	}
	if cfg.Port != 0 && !a.cliPort {
		a.port = cfg.Port
	}
	if cfg.User != "" && !a.cliUser {
		a.user = cfg.User
	}
	if cfg.IdentityFile != "" && !a.cliAuth {
		a.auth = cfg.IdentityFile
	}
	if cfg.Compression {
		a.compress = true
	}
	if cfg.ServerAliveInterval > 0 && !a.cliAlive {
		a.alive = uint32(cfg.ServerAliveInterval)
	}
	// 应用 SSH config 中的 ProxyJump（仅当命令行未指定时）
	if cfg.ProxyJump != "" && a.proxyJump == "" {
		a.proxyJump = cfg.ProxyJump
		a.proxyJumps = parseJumpHosts(cfg.ProxyJump)
		if len(a.proxyJumps) == 1 {
			jh := a.proxyJumps[0]
			a.proxyJump = jh.Host
			a.proxyJumpPort = jh.Port
			a.proxyJumpUser = jh.User
		}
	}
	return a
}

// parseArgsFrom 解析参数，出错返回 error 而不是退出，便于测试
// 注意：默认值在这里不设置，由 applyConfig 处理
func parseArgsFrom(argv []string) (args, error) {
	var a args
	for i := 0; i < len(argv); i++ {
		arg := argv[i]

		// 处理 -- 长选项
		if strings.HasPrefix(arg, "--") {
			// POSIX: -- 终止选项解析，后续全部作为位置参数
			if arg == "--" {
				i++ // 跳过 --
				if a.host == "" && i < len(argv) {
					a.host = argv[i]
					i++
				}
				if i < len(argv) {
					remaining := argv[i:]
					if a.cmd != "" {
						a.cmd += " "
					}
					a.cmd += strings.Join(remaining, " ")
				}
				break
			}

			longArg := arg[2:]
			// 处理 --key=value 格式
			if idx := strings.Index(longArg, "="); idx >= 0 {
				key := longArg[:idx]
				val := longArg[idx+1:]
				if err := setLongOpt(&a, key, val); err != nil {
					return a, err
				}
				continue
			}
			switch longArg {
			case "help":
				return a, fmt.Errorf("help")
			case "version":
				return a, fmt.Errorf("version")
			case "list":
				a.list = true
			case "no-agent":
				a.noAgent = true
			case "agent-forward":
				a.agentForward = true
			case "reconnect":
				a.reconnect = true
			case "sftp":
				a.sftp = true
			case "insecure-host-key":
				a.insecureHostKey = true
			default:
				// 带值的长选项
				if i+1 >= len(argv) {
					return a, fmt.Errorf("option --%s requires a value", longArg)
				}
				val := argv[i+1]
				if len(val) > 0 && val[0] == '-' {
					return a, fmt.Errorf("option --%s requires a value", longArg)
				}
				if err := setLongOpt(&a, longArg, val); err != nil {
					return a, err
				}
				i++
			}
			continue
		}

		// 处理 - 短选项
		if len(arg) > 1 && arg[0] == '-' {
			flag := arg[1:]
			switch flag {
			case "p":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -p requires a value")
					}
					p, err := strconv.ParseUint(val, 10, 16)
					if err != nil {
						return a, fmt.Errorf("invalid port: %s", val)
					}
					a.port = uint16(p)
					a.cliPort = true
					i++
				} else {
					return a, fmt.Errorf("option -p requires a value")
				}
			case "u":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -u requires a value")
					}
					a.user = val
					a.cliUser = true
					i++
				} else {
					return a, fmt.Errorf("option -u requires a value")
				}
			case "a":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -a requires a value")
					}
					a.auth = val
					a.cliAuth = true
					i++
				} else {
					return a, fmt.Errorf("option -a requires a value")
				}
			case "k":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -k requires a value")
					}
					p, err := strconv.ParseUint(val, 10, 32)
					if err != nil {
						return a, fmt.Errorf("invalid keep-alive: %s", val)
					}
					a.alive = uint32(p)
					a.cliAlive = true
					i++
				} else {
					return a, fmt.Errorf("option -k requires a value")
				}
			case "v":
				a.verbose = true
			case "V":
				return a, fmt.Errorf("version")
			case "A":
				a.agentForward = true
			case "C":
				a.compress = true
			case "J":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -J requires a value")
					}
					a.proxyJump = val
					a.proxyJumps = parseJumpHosts(val)
					// 兼容单跳：解析 user@host 和 host:port
					if len(a.proxyJumps) == 1 {
						jh := a.proxyJumps[0]
						a.proxyJump = jh.Host
						if jh.Port != 0 {
							a.proxyJumpPort = jh.Port
						}
						a.proxyJumpUser = jh.User
					}
					i++
				} else {
					return a, fmt.Errorf("option -J requires a value")
				}
			case "L":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -L requires a value")
					}
					a.localFwd = append(a.localFwd, val)
					i++
				} else {
					return a, fmt.Errorf("option -L requires a value")
				}
			case "R":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -R requires a value")
					}
					a.remoteFwd = append(a.remoteFwd, val)
					i++
				} else {
					return a, fmt.Errorf("option -R requires a value")
				}
			case "D":
				if i+1 < len(argv) {
					val := argv[i+1]
					if len(val) > 0 && val[0] == '-' {
						return a, fmt.Errorf("option -D requires a value")
					}
					a.socksPort = val
					i++
				} else {
					return a, fmt.Errorf("option -D requires a value")
				}
			case "h":
				return a, fmt.Errorf("help")
			default:
				return a, fmt.Errorf("unknown option: -%s", flag)
			}
			continue
		}

		// 非 flag 参数
		if a.host == "" {
			a.host = arg
		} else {
			// host 已设定，当前参数及之后所有参数都是远程命令
			// 把当前 arg 和剩余参数拼成 cmd
			cmdParts := []string{arg}
			cmdParts = append(cmdParts, argv[i+1:]...)
			if a.cmd != "" {
				a.cmd += " "
			}
			a.cmd += strings.Join(cmdParts, " ")
			break // 不再继续解析
		}
	}

	// 注意：默认值在这里不设置，由 applyConfig 处理
	return a, nil
}

// setLongOpt 设置长选项的值
func setLongOpt(a *args, key, val string) error {
	switch key {
	case "put":
		a.scpPut = val
	case "get":
		a.scpGet = val
	case "log":
		a.logFile = val
	case "save":
		a.save = val
	case "delete":
		a.delete = val
	case "reconnect-max":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid reconnect-max: %s", val)
		}
		a.reconnectMax = n
	case "edit":
		a.editFile = val
	default:
		return fmt.Errorf("unknown option: --%s", key)
	}
	return nil
}

// parseJumpHosts 解析逗号分隔的跳板机列表
// 支持格式: host, user@host, host:port, user@host:port
func parseJumpHosts(spec string) []jumpHost {
	parts := strings.Split(spec, ",")
	var hosts []jumpHost
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		jh := jumpHost{}
		// 解析 user@host
		if atIdx := strings.LastIndex(part, "@"); atIdx >= 0 {
			jh.User = part[:atIdx]
			part = part[atIdx+1:]
		}
		// 解析 host:port
		if host, portStr, err := net.SplitHostPort(part); err == nil {
			jh.Host = host
			if p, err := strconv.ParseUint(portStr, 10, 16); err == nil {
				jh.Port = uint16(p)
			}
		} else {
			jh.Host = part
		}
		hosts = append(hosts, jh)
	}
	return hosts
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: sshell -u <user> <host> [options] [command]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -p <port>       SSH port (default: 22)")
	fmt.Fprintln(os.Stderr, "  -u <user>       SSH username (required)")
	fmt.Fprintln(os.Stderr, "  -a <auth>       Password or path to private key file")
	fmt.Fprintln(os.Stderr, "  -k <seconds>    Keep-alive interval (default: 30)")
	fmt.Fprintln(os.Stderr, "  -v              Verbose output")
	fmt.Fprintln(os.Stderr, "  -V              Show version")
	fmt.Fprintln(os.Stderr, "  -A              Enable SSH Agent forwarding")
	fmt.Fprintln(os.Stderr, "  -C              Enable compression")
	fmt.Fprintln(os.Stderr, "  -J <jump>       ProxyJump through bastion host (comma-separated for multi-hop)")
	fmt.Fprintln(os.Stderr, "  -L <spec>       Local port forwarding ([bind:]port:host:hostport)")
	fmt.Fprintln(os.Stderr, "  -R <spec>       Remote port forwarding ([bind:]port:host:hostport)")
	fmt.Fprintln(os.Stderr, "  -D <port>       SOCKS5 dynamic proxy on local port")
	fmt.Fprintln(os.Stderr, "  --put <L:R>     Upload local file to remote (SCP/SFTP)")
	fmt.Fprintln(os.Stderr, "  --get <R:L>     Download remote file to local (SCP/SFTP)")
	fmt.Fprintln(os.Stderr, "  --sftp          Use SFTP protocol for --put/--get (preserves permissions)")
	fmt.Fprintln(os.Stderr, "  --edit <path>   Edit remote file with local editor (EDITOR/vim/nano)")
	fmt.Fprintln(os.Stderr, "  --log <file>    Log session output to file")
	fmt.Fprintln(os.Stderr, "  --save <name>   Save current connection as bookmark")
	fmt.Fprintln(os.Stderr, "  --list          List saved bookmarks")
	fmt.Fprintln(os.Stderr, "  --delete <name> Delete a bookmark")
	fmt.Fprintln(os.Stderr, "  --no-agent      Disable SSH Agent authentication")
	fmt.Fprintln(os.Stderr, "  --agent-forward Enable SSH Agent forwarding (same as -A)")
	fmt.Fprintln(os.Stderr, "  --insecure-host-key  Skip host key verification (MITM risk)")
	fmt.Fprintln(os.Stderr, "  --reconnect     Auto-reconnect on disconnect")
	fmt.Fprintln(os.Stderr, "  --reconnect-max Max reconnect attempts (default: 3)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  sshell -u root 192.168.1.100                     # Interactive shell")
	fmt.Fprintln(os.Stderr, "  sshell -u root 192.168.1.100 \"df -h\"             # Run remote command")
	fmt.Fprintln(os.Stderr, "  sshell -u root -J bastion 10.0.0.1               # Jump through bastion")
	fmt.Fprintln(os.Stderr, "  sshell -u root -J b1,b2,b3 10.0.0.1             # Multi-hop jump chain")
	fmt.Fprintln(os.Stderr, "  sshell -u root -A -J bastion 10.0.0.1            # Jump with agent forwarding")
	fmt.Fprintln(os.Stderr, "  sshell -u root -L 8080:localhost:80 host          # Local port forward")
	fmt.Fprintln(os.Stderr, "  sshell -u root --put ./local.txt:/tmp/ host       # Upload file")
	fmt.Fprintln(os.Stderr, "  sshell -u root --get /etc/hostname:./ host        # Download file")
	fmt.Fprintln(os.Stderr, "  sshell -u root -D 1080 host                      # SOCKS5 proxy")
	fmt.Fprintln(os.Stderr, "  sshell -u root --save myserver host               # Save bookmark")
	fmt.Fprintln(os.Stderr, "  sshell myserver                                   # Connect via bookmark")
	fmt.Fprintln(os.Stderr, "  sshell -u root --edit /etc/nginx/nginx.conf host  # Edit remote file locally")
}
