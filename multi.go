package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// outputMu 保护并发写入 stdout/stderr 时行级完整性
var outputMu sync.Mutex

// runMultiHost 在多台主机上并行执行命令
func runMultiHost(a args) error {
	if len(a.hosts) == 0 {
		return fmt.Errorf("no hosts specified")
	}

	if a.cmd == "" {
		return fmt.Errorf("multi-host mode requires a command (e.g., sshell -u user host1,host2 \"df -h\")")
	}

	if a.verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Executing on %d hosts: %s\n", len(a.hosts), strings.Join(a.hosts, ", "))
	}

	type hostResult struct {
		host   string
		err    error
		exitOK bool
	}

	results := make(chan hostResult, len(a.hosts))
	var wg sync.WaitGroup

	for _, host := range a.hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			err := runOnHost(a, h)
			results <- hostResult{host: h, err: err, exitOK: err == nil}
		}(host)
	}

	// 等待所有 goroutine 完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var failed []string
	var succeeded []string
	for r := range results {
		if r.exitOK {
			succeeded = append(succeeded, r.host)
		} else {
			failed = append(failed, r.host)
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "[sshell] [%s] Error: %v\n", r.host, r.err)
			}
		}
	}

	// 打印摘要
	fmt.Fprintf(os.Stderr, "\n[sshell] --- Summary ---\n")
	fmt.Fprintf(os.Stderr, "[sshell] Total: %d, Success: %d, Failed: %d\n",
		len(a.hosts), len(succeeded), len(failed))
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "[sshell] Failed hosts: %s\n", strings.Join(failed, ", "))
		return fmt.Errorf("%d host(s) failed", len(failed))
	}
	return nil
}

// runOnHost 在单台主机上执行命令
func runOnHost(a args, host string) error {
	// 复制 args，只替换 host
	hostArgs := a
	hostArgs.host = host
	hostArgs.hosts = nil // 避免递归

	// 应用 SSH config 到目标主机（与单主机模式 applySSHConfigTarget 保持一致的优先级）
	// 优先级: CLI > SSH config > sshell config > 默认值
	if cfg := loadSSHConfig(host); cfg != nil {
		if cfg.HostName != "" {
			hostArgs.host = cfg.HostName
		}
		if cfg.Port != 0 && !hostArgs.cliPort {
			hostArgs.port = cfg.Port
		}
		if cfg.User != "" && !hostArgs.cliUser {
			hostArgs.user = cfg.User
		}
		if cfg.IdentityFile != "" && !hostArgs.cliAuth {
			hostArgs.auth = cfg.IdentityFile
		}
		if cfg.ServerAliveInterval > 0 && !hostArgs.cliAlive {
			hostArgs.alive = uint32(cfg.ServerAliveInterval)
		}
	}

	// 多主机模式只需要执行命令，不需要 session（由 runRemoteCommandIO 自己创建）
	conn, err := connectClientWithRetry(hostArgs)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	if a.cmd == "" {
		// 没有命令时不走多主机模式（交互模式只适合单主机）
		return fmt.Errorf("no command specified for multi-host mode")
	}

	// 创建 session 用于执行命令
	session, err := conn.client.NewSession()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer session.Close()

	// Agent Forwarding（与单主机模式 runRemoteCommand 保持一致）
	if hostArgs.agentForward {
		af, afCleanup, err := setupAgentForwarding(conn.client, session, a.verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sshell] [%s] Agent forwarding failed: %v\n", host, err)
		} else {
			defer afCleanup()
			_ = af
		}
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Start(a.cmd); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	prefix := fmt.Sprintf("[%s] ", host)

	// 带前缀的输出转发
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		prefixLines(prefix, stdout, os.Stdout)
	}()
	go func() {
		defer wg.Done()
		prefixLines(prefix, stderr, os.Stderr)
	}()
	wg.Wait()

	return session.Wait()
}

// prefixLines 逐行添加前缀后输出（线程安全）
func prefixLines(prefix string, r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	// 增大缓冲区，避免长行被截断
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		outputMu.Lock()
		fmt.Fprintf(w, "%s%s\n", prefix, line)
		outputMu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		outputMu.Lock()
		fmt.Fprintf(os.Stderr, "%s[read error: %v]\n", prefix, err)
		outputMu.Unlock()
	}
}
