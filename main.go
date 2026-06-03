package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"

	"golang.org/x/crypto/ssh"
)

func main() {
	a, err := parseArgsVerbose()
	if err != nil {
		if err.Error() == "help" {
			printUsage()
			os.Exit(0)
		}
		if err.Error() == "version" {
			fmt.Fprintln(os.Stderr, version)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// --list: 列出书签
	if a.list {
		if err := listBookmarks(); err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// --delete: 删除书签
	if a.delete != "" {
		if err := deleteBookmark(a.delete); err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[sshell] Bookmark '%s' deleted.\n", a.delete)
		os.Exit(0)
	}

	// --edit: 远程文件编辑
	if a.editFile != "" {
		conn, err := connectClientWithRetry(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		if err := remoteEdit(conn.client, a.editFile, a.verbose); err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Edit failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 检查是否是书签名称（没有用户指定且 host 不像 IP/域名）
	if a.user == "" && a.host != "" {
		if bk, ok := lookupBookmark(a.host); ok {
			// 保留 CLI 已设置的标志，只用书签填充未设置的字段
			// 注意：cliPort/cliAuth/cliAlive/cliUser 标志位也需要保留，
			// 因为 a = bk 会替换整个结构体，导致这些标志丢失。
			cliVerbose := a.verbose
			cliAgentFwd := a.agentForward
			cliCompress := a.compress
			cliLocalFwd := a.localFwd
			cliRemoteFwd := a.remoteFwd
			cliSocks := a.socksPort
			cliLog := a.logFile
			cliSftp := a.sftp
			cliNoAgent := a.noAgent
			cliReconnect := a.reconnect
			cliReconnectMax := a.reconnectMax
			cliInsecureHostKey := a.insecureHostKey
			cliScpPut := a.scpPut
			cliScpGet := a.scpGet
			cliEditFile := a.editFile
			cliCmd := a.cmd
			cliPort := a.cliPort
			cliUser := a.cliUser
			cliAuth := a.cliAuth
			cliAlive := a.cliAlive
			// 保存 CLI 显式设置的值（用于在书签值之上恢复）
			cliPortVal := a.port
			cliUserVal := a.user
			cliAuthVal := a.auth
			cliAliveVal := a.alive

			a = bk

			// 恢复 CLI 标志
			if cliVerbose {
				a.verbose = true
			}
			if cliAgentFwd {
				a.agentForward = true
			}
			if cliCompress {
				a.compress = true
			}
			if len(cliLocalFwd) > 0 {
				a.localFwd = cliLocalFwd
			}
			if len(cliRemoteFwd) > 0 {
				a.remoteFwd = cliRemoteFwd
			}
			if cliSocks != "" {
				a.socksPort = cliSocks
			}
			if cliLog != "" {
				a.logFile = cliLog
			}
			if cliSftp {
				a.sftp = true
			}
			if cliNoAgent {
				a.noAgent = true
			}
			if cliReconnect {
				a.reconnect = true
			}
			if cliReconnectMax > 0 {
				a.reconnectMax = cliReconnectMax
			}
			if cliInsecureHostKey {
				a.insecureHostKey = true
			}
			if cliScpPut != "" {
				a.scpPut = cliScpPut
			}
			if cliScpGet != "" {
				a.scpGet = cliScpGet
			}
			if cliEditFile != "" {
				a.editFile = cliEditFile
			}
			if cliCmd != "" {
				a.cmd = cliCmd
			}
			// 恢复 CLI 显式设置的参数值和标志位
			if cliPort {
				a.port = cliPortVal
				a.cliPort = true
			}
			if cliUser {
				a.user = cliUserVal
				a.cliUser = true
			}
			if cliAuth {
				a.auth = cliAuthVal
				a.cliAuth = true
			}
			if cliAlive {
				a.alive = cliAliveVal
				a.cliAlive = true
			}

			// 重新应用默认值
			if a.port == 0 {
				a.port = 22
			}
			if a.alive == 0 {
				a.alive = 30
			}
		}
	}

	// --save: 保存书签
	if a.save != "" {
		if err := saveBookmark(a.save, argsToBookmark(a)); err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[sshell] Bookmark '%s' saved.\n", a.save)
		// 如果没有命令要执行，直接退出
		if a.cmd == "" && len(a.localFwd) == 0 && len(a.remoteFwd) == 0 && a.socksPort == "" {
			os.Exit(0)
		}
	}

	// 多主机模式
	if len(a.hosts) > 1 {
		if err := runMultiHost(a); err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// SFTP 文件传输
	if a.sftp && (a.scpPut != "" || a.scpGet != "") {
		conn, err := connectClientWithRetry(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		if a.scpPut != "" {
			localPath, remotePath := parseSCPPath(a.scpPut)
			if err := sftpPut(conn.client, localPath, remotePath, a.verbose); err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] SFTP put failed: %v\n", err)
				os.Exit(1)
			}
		}
		if a.scpGet != "" {
			remotePath, localPath := parseSCPPath(a.scpGet)
			if err := sftpGet(conn.client, remotePath, localPath, a.verbose); err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] SFTP get failed: %v\n", err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// SCP 文件传输
	if a.scpPut != "" || a.scpGet != "" {
		conn, err := connectClientWithRetry(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		if a.scpPut != "" {
			localPath, remotePath := parseSCPPath(a.scpPut)
			if err := scpPut(conn.client, localPath, remotePath, a.verbose); err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] SCP put failed: %v\n", err)
				os.Exit(1)
			}
		}
		if a.scpGet != "" {
			remotePath, localPath := parseSCPPath(a.scpGet)
			if err := scpGet(conn.client, remotePath, localPath, a.verbose); err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] SCP get failed: %v\n", err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// 端口转发 / SOCKS5 模式
	if len(a.localFwd) > 0 || len(a.remoteFwd) > 0 || a.socksPort != "" {
		conn, err := connectClientWithRetry(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close()

		var closers []io.Closer

		for _, spec := range a.localFwd {
			ln, err := startLocalForward(conn.client, spec, a.verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] Local forward %s failed: %v\n", spec, err)
				os.Exit(1)
			}
			closers = append(closers, ln)
			if a.verbose {
				fmt.Fprintf(os.Stderr, "[sshell] Listening on %s\n", spec)
			}
		}

		for _, spec := range a.remoteFwd {
			if err := startRemoteForward(conn.client, spec, a.verbose); err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] Remote forward %s failed: %v\n", spec, err)
				os.Exit(1)
			}
		}

		if a.socksPort != "" {
			ln, err := startSOCKS5Proxy(conn.client, a.socksPort, a.verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] SOCKS5 proxy failed: %v\n", err)
				os.Exit(1)
			}
			closers = append(closers, ln)
			fmt.Fprintf(os.Stderr, "[sshell] SOCKS5 proxy listening on %s\n", a.socksPort)
		}

		// 等待信号中断，优雅关闭
		fmt.Fprintln(os.Stderr, "[sshell] Forwarding active. Press Ctrl+C to stop.")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, sigInterrupt, sigTerminate)
		<-sigCh
		fmt.Fprintln(os.Stderr, "\n[sshell] Shutting down...")
		signal.Stop(sigCh)
		for _, c := range closers {
			c.Close()
		}
		os.Exit(0)
	}

	// 普通连接（交互或远程命令）
	conn, err := connectWithRetry(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if a.cmd != "" {
		// 非交互模式：执行远程命令
		code, err := runRemoteCommand(conn.Session, conn.client, a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			os.Exit(1)
		}
		os.Exit(code)
	} else {
		// 交互模式
		if err := interactiveShell(conn.Session, conn.client, a); err != nil {
			// 提取远程退出码，避免字符串比较
			if exitErr, ok := err.(*ssh.ExitError); ok {
				if exitErr.ExitStatus() != 0 {
					fmt.Fprintf(os.Stderr, "[sshell] exit status %d\n", exitErr.ExitStatus())
				}
			} else {
				fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
			}
		}
	}
}

// parseSCPPath 解析 "local:remote" 格式的路径
// 跳过 Windows 盘符（如 C:\、D:/），仅当冒号前是单个字母且后跟路径分隔符时视为盘符
func parseSCPPath(spec string) (string, string) {
	for i := 0; i < len(spec); i++ {
		if spec[i] == ':' {
			// 跳过 Windows 盘符：单个字母后跟 :\ 或 :/（如 C:\Users）
			if i == 1 && ((spec[0] >= 'A' && spec[0] <= 'Z') || (spec[0] >= 'a' && spec[0] <= 'z')) && i+1 < len(spec) && (spec[i+1] == '\\' || spec[i+1] == '/') {
				continue
			}
			return spec[:i], spec[i+1:]
		}
	}
	// 没有冒号，两端都用同一个路径
	return spec, spec
}
