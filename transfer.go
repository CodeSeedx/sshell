package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// shellQuote 用单引号包裹字符串，防止 shell 注入
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// scpPut uploads a local file to the remote server using the SCP protocol over SSH.
func scpPut(client *ssh.Client, localPath, remotePath string, verbose bool) error {
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

	// Determine remote filename
	remoteName := filepath.Base(remotePath)
	if strings.HasSuffix(remotePath, "/") {
		remoteName = filepath.Base(localPath)
	}

	// Create SSH session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Remote directory (strip trailing / and filename if needed)
	remoteDir := remotePath
	if strings.HasSuffix(remotePath, "/") {
		remoteDir = strings.TrimSuffix(remotePath, "/")
	} else {
		remoteDir = filepath.Dir(remotePath)
	}
	if remoteDir == "" {
		remoteDir = "."
	}

	type scpResult struct {
		err error
	}
	resultCh := make(chan scpResult, 1)

	go func() {
		w := bufio.NewWriter(stdin)
		defer stdin.Close()

		// Send "C<perm> <size> <name>\n"
		mode := fmt.Sprintf("%04o", stat.Mode().Perm())
		if _, err := fmt.Fprintf(w, "C%s %d %s\n", mode, stat.Size(), remoteName); err != nil {
			resultCh <- scpResult{fmt.Errorf("write file header: %w", err)}
			return
		}
		if err := w.Flush(); err != nil {
			resultCh <- scpResult{fmt.Errorf("flush file header: %w", err)}
			return
		}

		// Wait for acknowledgment
		ack := make([]byte, 1)
		if _, err := stdout.Read(ack); err != nil {
			resultCh <- scpResult{fmt.Errorf("read ack: %w", err)}
			return
		}
		if ack[0] != 0 {
			resultCh <- scpResult{fmt.Errorf("remote rejected file header (ack=0x%02x)", ack[0])}
			return
		}

		// Send file data
		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Uploading %s (%d bytes) to %s\n", localPath, stat.Size(), remotePath)
		}

		sent := int64(0)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := f.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if _, err := w.Write(chunk); err != nil {
					resultCh <- scpResult{fmt.Errorf("write data: %w", err)}
					return
				}
				w.Flush()
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
				resultCh <- scpResult{fmt.Errorf("read local file: %w", readErr)}
				return
			}
		}

		if verbose && stat.Size() > 1024*1024 {
			fmt.Fprintln(os.Stderr)
		}

		// Send final \x00 to indicate end of file
		if err := w.WriteByte(0x00); err != nil {
			resultCh <- scpResult{fmt.Errorf("write eof marker: %w", err)}
			return
		}
		if err := w.Flush(); err != nil {
			resultCh <- scpResult{fmt.Errorf("flush eof marker: %w", err)}
			return
		}

		// Wait for final acknowledgment
		if _, err := stdout.Read(ack); err != nil {
			resultCh <- scpResult{fmt.Errorf("read final ack: %w", err)}
			return
		}
		if ack[0] != 0x00 {
			resultCh <- scpResult{fmt.Errorf("remote rejected file data (ack=0x%02x)", ack[0])}
			return
		}

		resultCh <- scpResult{nil}
	}()

	// Run the remote scp command
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Remote: scp -t %s\n", remoteDir)
	}
	sessionErr := session.Run(fmt.Sprintf("scp -t %s", shellQuote(remoteDir)))

	// 等待 goroutine 完成后再判断结果，避免竞态
	res := <-resultCh

	if sessionErr != nil {
		return fmt.Errorf("scp put failed: %w", sessionErr)
	}
	if res.err != nil {
		return res.err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Upload complete: %s\n", localPath)
	}
	return nil
}

// scpGet downloads a remote file from the server to local using the SCP protocol over SSH.
func scpGet(client *ssh.Client, remotePath, localPath string, verbose bool) error {
	// Create SSH session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	type scpResult struct {
		err error
	}
	resultCh := make(chan scpResult, 1)

	go func() {
		r := bufio.NewReader(stdout)
		w := bufio.NewWriter(stdin)
		defer stdin.Close()

		// Send initial \x00 to start transfer
		if err := w.WriteByte(0x00); err != nil {
			resultCh <- scpResult{fmt.Errorf("write start marker: %w", err)}
			return
		}
		if err := w.Flush(); err != nil {
			resultCh <- scpResult{fmt.Errorf("flush start marker: %w", err)}
			return
		}

		// Read "C<mode> <size> <name>\n" or error
		line, err := r.ReadString('\n')
		if err != nil {
			resultCh <- scpResult{fmt.Errorf("read scp header: %w", err)}
			return
		}

		if len(line) == 0 {
			resultCh <- scpResult{fmt.Errorf("empty scp response")}
			return
		}

		// Check for error (starts with \x01 or \x02)
		if line[0] == 0x01 || line[0] == 0x02 {
			msg := strings.TrimPrefix(line[1:], "scp: ")
			msg = strings.TrimSpace(msg)
			resultCh <- scpResult{fmt.Errorf("remote error: %s", msg)}
			return
		}

		// Parse "C<mode> <size> <name>\n"
		if line[0] != 'C' {
			resultCh <- scpResult{fmt.Errorf("unexpected scp response: %q", line)}
			return
		}

		parts := strings.Fields(line[1:]) // strip leading 'C'
		if len(parts) < 3 {
			resultCh <- scpResult{fmt.Errorf("malformed scp header: %q", line)}
			return
		}

		mode := parts[0] // e.g., "0644"
		size, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			resultCh <- scpResult{fmt.Errorf("invalid file size: %s", parts[1])}
			return
		}
		name := strings.TrimSpace(parts[2])
		// 防止路径穿越：只取文件名，丢弃路径前缀
		name = filepath.Base(name)
		if name == "." || name == "/" {
			resultCh <- scpResult{fmt.Errorf("invalid remote filename: %q", parts[2])}
			return
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "[sshell] Receiving %s (%d bytes)\n", name, size)
		}

		// Determine local output path
		outPath := localPath
		if info, statErr := os.Stat(localPath); statErr == nil && info.IsDir() {
			outPath = filepath.Join(localPath, name)
		} else if strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, string(filepath.Separator)) {
			// localPath ends with / — create dir and use remote filename
			if mkErr := os.MkdirAll(localPath, 0755); mkErr != nil {
				resultCh <- scpResult{fmt.Errorf("mkdir %s: %w", localPath, mkErr)}
				return
			}
			outPath = filepath.Join(localPath, name)
		} else {
			// Ensure parent directory exists
			dir := filepath.Dir(outPath)
			if dir != "" && dir != "." {
				if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
					resultCh <- scpResult{fmt.Errorf("mkdir %s: %w", dir, mkErr)}
					return
				}
			}
		}

		// Acknowledge the header — tell remote to send data
		if err := w.WriteByte(0x00); err != nil {
			resultCh <- scpResult{fmt.Errorf("write header ack: %w", err)}
			return
		}
		if err := w.Flush(); err != nil {
			resultCh <- scpResult{fmt.Errorf("flush header ack: %w", err)}
			return
		}

		// 创建临时文件，下载成功后 rename 为最终路径
		tmpPath := outPath + ".tmp"
		f, err := os.Create(tmpPath)
		if err != nil {
			resultCh <- scpResult{fmt.Errorf("create local file: %w", err)}
			return
		}
		// 应用远程文件权限
		if parsedMode, parseErr := strconv.ParseUint(mode, 8, 32); parseErr == nil {
			os.Chmod(tmpPath, os.FileMode(parsedMode))
		}
		success := false
		fileClosed := false
		defer func() {
			if !fileClosed {
				f.Close()
			}
			if success {
				// 传输成功，rename 为最终路径
				os.Rename(tmpPath, outPath)
			} else {
				// 传输失败，删除临时文件
				os.Remove(tmpPath)
			}
		}()

		received := int64(0)
		buf := make([]byte, 32*1024)
		for received < size {
			remaining := size - received
			toRead := int64(len(buf))
			if remaining < toRead {
				toRead = remaining
			}
			n, readErr := r.Read(buf[:toRead])
			if n > 0 {
				if _, writeErr := f.Write(buf[:n]); writeErr != nil {
					resultCh <- scpResult{fmt.Errorf("write local file: %w", writeErr)}
					return
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
				resultCh <- scpResult{fmt.Errorf("read file data: %w", readErr)}
				return
			}
		}

		if verbose && size > 1024*1024 {
			fmt.Fprintln(os.Stderr)
		}

		// Read trailing \x00 from remote (end of file marker)
		ack := make([]byte, 1)
		if _, err := r.Read(ack); err != nil {
			resultCh <- scpResult{fmt.Errorf("read eof marker: %w", err)}
			return
		}
		if ack[0] != 0x00 {
			resultCh <- scpResult{fmt.Errorf("unexpected eof marker: %d", ack[0])}
			return
		}

		// Send final \x00 to acknowledge
		if err := w.WriteByte(0x00); err != nil {
			resultCh <- scpResult{fmt.Errorf("write final ack: %w", err)}
			return
		}
		if err := w.Flush(); err != nil {
			resultCh <- scpResult{fmt.Errorf("flush final ack: %w", err)}
			return
		}

		// 显式关闭文件，检查 flush 错误（防止静默数据损坏）
		if err := f.Close(); err != nil {
			fileClosed = true
			resultCh <- scpResult{fmt.Errorf("close local file: %w", err)}
			return
		}
		fileClosed = true

		success = true
		resultCh <- scpResult{nil}
	}()

	// Run remote scp in source mode
	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Remote: scp -f %s\n", remotePath)
	}
	sessionErr := session.Run(fmt.Sprintf("scp -f %s", shellQuote(remotePath)))

	// 等待 goroutine 完成后再判断结果，避免竞态
	res := <-resultCh

	if sessionErr != nil {
		return fmt.Errorf("scp get failed: %w", sessionErr)
	}
	if res.err != nil {
		return res.err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[sshell] Download complete: %s\n", localPath)
	}
	return nil
}
