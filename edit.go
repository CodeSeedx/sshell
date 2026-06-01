package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// remoteEdit 下载远程文件到本地临时目录，用编辑器编辑后上传回远程
func remoteEdit(client *ssh.Client, remotePath string, verbose bool) error {
	// 获取文件名用于临时文件命名
	baseName := filepath.Base(remotePath)
	if baseName == "" || baseName == "/" || baseName == "." {
		baseName = "remote_file"
	}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "sshell-edit-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// SIGINT 时确保清理临时目录（Go 默认 SIGINT 会跳过 defer）
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sigInterrupt, sigTerminate)
	go func() {
		<-sigCh
		os.RemoveAll(tmpDir)
		os.Exit(130) // 标准 SIGINT 退出码
	}()
	defer signal.Stop(sigCh)

	localPath := filepath.Join(tmpDir, baseName)

	// 创建 SFTP 客户端，复用于所有操作（stat/download/upload）
	sftpClient, sftpErr := sftp.NewClient(client)
	if sftpClient != nil {
		defer sftpClient.Close()
	}

	// 获取远程文件权限
	var remotePerms os.FileMode = 0644
	if sftpErr == nil {
		if info, statErr := sftpClient.Stat(remotePath); statErr == nil {
			remotePerms = info.Mode().Perm()
		}
	}

	// 下载远程文件（优先 SFTP 复用连接，回退 SCP）
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Downloading %s...\n", remotePath)
	}
	downloaded := false
	if sftpErr == nil {
		if err := sftpGetClient(sftpClient, remotePath, localPath, verbose); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[sshell] SFTP failed, trying SCP: %v\n", err)
			}
		} else {
			downloaded = true
		}
	}
	if !downloaded {
		if err := scpGet(client, remotePath, localPath, verbose); err != nil {
			return fmt.Errorf("download: %w", err)
		}
	}

	// 计算原始文件哈希
	origHash, err := fileSHA256(localPath)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}

	// 恢复文件权限以便编辑器能正确处理
	if err := os.Chmod(localPath, remotePerms); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: cannot set local file permissions: %v\n", err)
		}
	}

	// 打开编辑器
	editor := getEditor()
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Opening %s with %s...\n", localPath, editor)
	}

	editCmd := buildEditCmd(editor, localPath)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	// 计算编辑后文件哈希
	newHash, err := fileSHA256(localPath)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}

	// 比较哈希，如果没变化则跳过上传
	if origHash == newHash {
		fmt.Fprintln(os.Stderr, "[sshell] No changes detected.")
		return nil
	}

	// 确保文件权限正确（可能被编辑器修改）
	if err := os.Chmod(localPath, remotePerms); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: cannot restore file permissions: %v\n", err)
		}
	}

	// 上传修改后的文件（优先 SFTP 复用连接，回退 SCP）
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Uploading %s to %s...\n", localPath, remotePath)
	}
	uploaded := false
	if sftpErr == nil {
		if err := sftpPutClient(sftpClient, localPath, remotePath, verbose); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "[sshell] SFTP failed, trying SCP: %v\n", err)
			}
		} else {
			uploaded = true
		}
	}
	if !uploaded {
		if err := scpPut(client, localPath, remotePath, verbose); err != nil {
			return fmt.Errorf("upload: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "[sshell] File updated: %s\n", remotePath)
	return nil
}

// sftpGetClient 使用已有的 SFTP 客户端下载文件（复用连接）
func sftpGetClient(sftpClient *sftp.Client, remotePath, localPath string, verbose bool) error {
	remoteStat, err := sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("stat remote file: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SFTP downloading %s (%d bytes)\n", remotePath, remoteStat.Size())
	}

	inFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer inFile.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(f, inFile, buf); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}

// sftpPutClient 使用已有的 SFTP 客户端上传文件（复用连接）
func sftpPutClient(sftpClient *sftp.Client, localPath, remotePath string, verbose bool) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	outFile, err := sftpClient.OpenFile(remotePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}

	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(outFile, f, buf); err != nil {
		outFile.Close()
		return fmt.Errorf("copy: %w", err)
	}

	// 关闭文件，捕获可能的 flush 错误
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("close remote file: %w", err)
	}

	// 写入完成后再设置权限
	if err := sftpClient.Chmod(remotePath, stat.Mode().Perm()); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: could not set permissions: %v\n", err)
		}
	}

	return nil
}

// getEditor 获取编辑器，优先级: EDITOR > VISUAL > vim > vi > nano > notepad
func getEditor() string {
	// POSIX 惯例: VISUAL 优先于 EDITOR
	// VISUAL 用于全屏编辑器，EDITOR 用于非交互场景的兜底
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// 尝试常见编辑器
	for _, name := range []string{"vim", "vi", "nano", "emacs"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	// 兜底
	return "vi"
}

// buildEditCmd 构建编辑器命令，支持 EDITOR 含空格和参数（如 "code --wait"）
// 使用 sh -c 执行，正确处理 EDITOR 值中的引号和特殊字符
func buildEditCmd(editor, filePath string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+` "$1"`, "--", filePath)
}

// fileSHA256 计算文件的 SHA256 哈希
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// isAbsolutePath 判断是否为绝对路径
func isAbsolutePath(path string) bool {
	return strings.HasPrefix(path, "/") || (len(path) >= 2 && path[1] == ':')
}
