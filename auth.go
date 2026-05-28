package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

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
	// 如果指定了 -a 参数
	if a.auth != "" {
		// 尝试作为文件读取
		if _, err := os.Stat(a.auth); err == nil {
			return loadKeyFile(a.auth, a.verbose)
		}
		// 当作密码使用
		if a.verbose {
			fmt.Fprintln(os.Stderr, "[sshell] Auth: password")
		}
		return []ssh.AuthMethod{ssh.Password(a.auth)}, nil
	}

	// 自动探测密钥文件
	if method, err := autoDetectKey(a.verbose); err == nil {
		return method, nil
	}

	// 兜底：提示输入密码
	pw, err := readPassword(fmt.Sprintf("%s@%s's password: ", a.user, a.host))
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return []ssh.AuthMethod{ssh.Password(pw)}, nil
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

func autoDetectKey(verbose bool) ([]ssh.AuthMethod, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

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
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Auth: key (%s)\n", p)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}

	return nil, fmt.Errorf("no key files found")
}
