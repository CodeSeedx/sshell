package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

func readPassword(prompt string) (string, error) {
	// 检查是否是交互式终端
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("password required but no terminal available")
	}
	
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func findAuth(a args) ([]ssh.AuthMethod, func(), error) {
	var methods []ssh.AuthMethod
	var cleanups []func()

	// 1. SSH Agent（除非 --no-agent）
	if !a.noAgent {
		if agentMethod, agentCleanup, err := sshAgentAuth(); err == nil {
			methods = append(methods, agentMethod)
		 cleanups = append(cleanups, agentCleanup)
			if a.verbose {
				fmt.Fprintln(os.Stderr, "[sshell] Auth: SSH Agent")
			}
		}
	}

	// combined cleanup closes all tracked resources
	combinedCleanup := func() {
		for _, c := range cleanups {
			c()
		}
	}

	// 2. 如果指定了 -a 参数
	if a.auth != "" {
		// 尝试作为文件读取
		if _, err := os.Stat(a.auth); err == nil {
			keyMethods, err := loadKeyFile(a.auth, a.verbose)
			if err != nil {
				combinedCleanup()
				return nil, nil, err
			}
			methods = append(methods, keyMethods...)
			return methods, combinedCleanup, nil
		}
		// 当作密码使用（优先从环境变量读取，避免 ps 泄露）
		if envPw := os.Getenv("SSHELL_PASSWORD"); envPw != "" {
			os.Unsetenv("SSHELL_PASSWORD") // 立即清除，防止泄露到子进程（如编辑器）
			if a.verbose {
				fmt.Fprintln(os.Stderr, "[sshell] Auth: password (from SSHELL_PASSWORD)")
			}
			methods = append(methods, ssh.Password(envPw))
		} else {
			// -a 值不是文件路径，直接当密码（向后兼容，但警告）
			fmt.Fprintln(os.Stderr, "[sshell] Warning: password via -a is visible in process list. Use SSHELL_PASSWORD env or key file instead.")
			methods = append(methods, ssh.Password(a.auth))
		}
		return methods, combinedCleanup, nil
	}

	// 3. 自动探测密钥文件
	if keyMethods, err := autoDetectKeys(a.verbose); err == nil {
		methods = append(methods, keyMethods...)
	}

	// 如果已经有 agent 或密钥方法，直接返回
	if len(methods) > 0 {
		return methods, combinedCleanup, nil
	}

	// 4. 兜底：提示输入密码
	pw, err := readPassword(fmt.Sprintf("%s@%s's password: ", a.user, a.host))
	if err != nil {
		combinedCleanup()
		return nil, nil, fmt.Errorf("read password: %w", err)
	}
	return []ssh.AuthMethod{ssh.Password(pw)}, combinedCleanup, nil
}

// sshAgentAuth 连接 SSH Agent 获取认证方法
// 返回 auth method、cleanup 函数（关闭底层 socket 连接）和 error
func sshAgentAuth() (ssh.AuthMethod, func(), error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, nil, fmt.Errorf("connect SSH agent: %w", err)
	}
	cleanup := func() { conn.Close() }
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), cleanup, nil
}

func loadKeyFile(path string, verbose bool) ([]ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		// 密钥有密码保护
		pw, perr := readPassword("Key passphrase: ")
		if perr != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(pw))
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Auth: key file (%s)\n", path)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

// autoDetectKeys 探测所有默认密钥文件，返回所有找到的认证方法
// 对加密密钥，提示用户输入 passphrase 并尝试解密
func autoDetectKeys(verbose bool) ([]ssh.AuthMethod, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var methods []ssh.AuthMethod
	var promptOnce bool // 整个探测过程只提示一次 passphrase
	var savedPassphrase string
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		p := filepath.Join(home, ".ssh", name)
		key, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// 可能是加密密钥，尝试提示 passphrase
			var pw string
			if !promptOnce {
				var perr error
				pw, perr = readPassword("Key passphrase: ")
				if perr != nil {
					promptOnce = true // 用户取消输入，不再提示
					continue
				}
				savedPassphrase = pw
			} else {
				pw = savedPassphrase
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(pw))
			if err != nil {
				continue // 密码错误或已取消，跳过此密钥
			}
			promptOnce = true // 成功解密，后续密钥使用同一密码
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Auth: key (%s)\n", p)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no key files found")
	}
	return methods, nil
}

// autoDetectKey 保留兼容：返回第一个找到的密钥
func autoDetectKey(verbose bool) ([]ssh.AuthMethod, error) {
	methods, err := autoDetectKeys(verbose)
	if err != nil {
		return nil, err
	}
	return methods[:1], nil
}
