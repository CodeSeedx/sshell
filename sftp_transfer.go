package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sftpPut uploads a local file to the remote server using the SFTP protocol.
// Unlike SCP, SFTP preserves file permissions natively.
func sftpPut(client *ssh.Client, localPath, remotePath string, verbose bool) error {
	// Open local file
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	// Determine remote filename and directory
	remoteName := filepath.Base(remotePath)
	remoteDir := remotePath
	if strings.HasSuffix(remotePath, "/") {
		remoteName = filepath.Base(localPath)
		remoteDir = strings.TrimSuffix(remotePath, "/")
	} else {
		remoteDir = filepath.Dir(remotePath)
	}
	if remoteDir == "" {
		remoteDir = "."
	}

	// Create remote directory if needed
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("mkdir remote %s: %w", remoteDir, err)
	}

	// Build final remote path
	finalRemotePath := filepath.Join(remoteDir, remoteName)
	if strings.HasSuffix(remotePath, "/") {
		finalRemotePath = remoteDir + "/" + remoteName
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SFTP uploading %s (%d bytes) to %s\n", localPath, stat.Size(), finalRemotePath)
	}

	// Create remote file with same permissions as local
	outFile, err := sftpClient.OpenFile(finalRemotePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	// 立即设置权限，减少文件以错误权限暴露的窗口期
	if err := sftpClient.Chmod(finalRemotePath, stat.Mode().Perm()); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Warning: could not set permissions: %v\n", err)
		}
	}
	defer outFile.Close()

	// Copy with progress reporting
	sent := int64(0)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write remote: %w", writeErr)
			}
			sent += int64(n)

			if verbose && stat.Size() > 1024*1024 {
				pct := sent * 100 / stat.Size()
				fmt.Fprintf(os.Stderr, "\r[sshell] Progress: %d%%", pct)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read local file: %w", readErr)
		}
	}

	if verbose && stat.Size() > 1024*1024 {
		fmt.Fprintln(os.Stderr)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SFTP upload complete: %s\n", localPath)
	}
	return nil
}

// sftpGet downloads a remote file from the server to local using the SFTP protocol.
// Unlike SCP, SFTP preserves file permissions natively.
func sftpGet(client *ssh.Client, remotePath, localPath string, verbose bool) error {
	// Create SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	// Stat remote file
	remoteStat, err := sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("stat remote file: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SFTP downloading %s (%d bytes)\n", remotePath, remoteStat.Size())
	}

	// Determine local output path
	outPath := localPath
	if info, statErr := os.Stat(localPath); statErr == nil && info.IsDir() {
		outPath = filepath.Join(localPath, filepath.Base(remotePath))
	} else if strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, string(filepath.Separator)) {
		if mkErr := os.MkdirAll(localPath, 0755); mkErr != nil {
			return fmt.Errorf("mkdir %s: %w", localPath, mkErr)
		}
		outPath = filepath.Join(localPath, filepath.Base(remotePath))
	} else {
		dir := filepath.Dir(outPath)
		if dir != "" && dir != "." {
			if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
				return fmt.Errorf("mkdir %s: %w", dir, mkErr)
			}
		}
	}

	// Open remote file
	inFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer inFile.Close()

	// 创建临时文件，下载成功后 rename 为最终路径
	tmpPath := outPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, remoteStat.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	success := false
	defer func() {
		f.Close()
		if success {
			os.Rename(tmpPath, outPath)
		} else {
			os.Remove(tmpPath)
		}
	}()

	// Copy with progress reporting
	received := int64(0)
	size := remoteStat.Size()
	buf := make([]byte, 32*1024)
	for {
		n, readErr := inFile.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write local file: %w", writeErr)
			}
			received += int64(n)

			if verbose && size > 1024*1024 {
				pct := received * 100 / size
				fmt.Fprintf(os.Stderr, "\r[sshell] Progress: %d%%", pct)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read remote file: %w", readErr)
		}
	}

	if verbose && size > 1024*1024 {
		fmt.Fprintln(os.Stderr)
	}

	success = true
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] SFTP download complete: %s\n", outPath)
	}
	return nil
}
