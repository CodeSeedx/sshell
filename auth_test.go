package main

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// generateTestEd25519Key 用纯 Go 生成 Ed25519 测试密钥，返回密钥文件路径
func generateTestEd25519Key(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_ed25519")

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		t.Fatalf("verify key parse: %v", err)
	}
	if signer == nil {
		t.Fatal("signer is nil")
	}

	return keyPath
}

func TestLoadKeyFile(t *testing.T) {
	keyPath := generateTestEd25519Key(t)
	methods, err := loadKeyFile(keyPath, false)
	if err != nil {
		t.Fatalf("loadKeyFile failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileNotFound(t *testing.T) {
	_, err := loadKeyFile("/nonexistent/path/key", false)
	if err == nil {
		t.Error("expected error for nonexistent key file")
	}
}

func TestLoadKeyFileVerbose(t *testing.T) {
	keyPath := generateTestEd25519Key(t)
	methods, err := loadKeyFile(keyPath, true)
	if err != nil {
		t.Fatalf("loadKeyFile failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileInvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "bad_key")
	if err := os.WriteFile(keyPath, []byte("not a real key"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	_, err := loadKeyFile(keyPath, false)
	if err == nil {
		t.Error("expected error for invalid key file")
	}
}

// ==================== autoDetectKey 测试 ====================

func TestAutoDetectKeyNoHome(t *testing.T) {
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	_, err := autoDetectKey(false)
	if err == nil {
		t.Error("expected error when HOME is not set")
	}
}

func TestAutoDetectKeyNoKeys(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := autoDetectKey(false)
	if err == nil {
		t.Error("expected error when no key files exist")
	}
}

func TestAutoDetectKeyWithEd25519(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(false)
	if err != nil {
		t.Fatalf("autoDetectKey failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyWithRsa(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_rsa")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(false)
	if err != nil {
		t.Fatalf("autoDetectKey failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyWithEcdsa(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_ecdsa")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(false)
	if err != nil {
		t.Fatalf("autoDetectKey failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(true)
	if err != nil {
		t.Fatalf("autoDetectKey verbose failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyBadKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("bad key content"), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := autoDetectKey(false)
	if err == nil {
		t.Error("expected error for bad key file")
	}
}

// ==================== findAuth 测试 ====================

func TestFindAuthWithPassword(t *testing.T) {
	a := args{
		auth: "testpassword",
		host: "testhost",
		user: "testuser",
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthWithKeyFile(t *testing.T) {
	keyPath := generateTestEd25519Key(t)
	a := args{
		auth: keyPath,
		host: "testhost",
		user: "testuser",
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthWithVerboseKey(t *testing.T) {
	keyPath := generateTestEd25519Key(t)
	a := args{
		auth:    keyPath,
		host:    "testhost",
		user:    "testuser",
		verbose: true,
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthWithVerbosePassword(t *testing.T) {
	a := args{
		auth:    "testpw",
		host:    "testhost",
		user:    "testuser",
		verbose: true,
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthAutoDetect(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{
		auth: "",
		host: "testhost",
		user: "testuser",
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth auto-detect failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthNoAutoDetect(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{
		auth: "",
		host: "testhost",
		user: "testuser",
	}

	_, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Log("findAuth succeeded (system has available keys)")
	}
}

func TestFindAuthInvalidKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	badKeyPath := filepath.Join(tmpDir, "bad_key")
	if err := os.WriteFile(badKeyPath, []byte("not a real key"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	a := args{
		auth: badKeyPath,
		host: "testhost",
		user: "testuser",
	}
	_, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Error("expected error for invalid key file")
	}
}

func TestFindAuthInvalidKeyFileVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	badKeyPath := filepath.Join(tmpDir, "bad_key")
	if err := os.WriteFile(badKeyPath, []byte("not a real key"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	a := args{
		auth:    badKeyPath,
		host:    "testhost",
		user:    "testuser",
		verbose: true,
	}
	_, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err == nil {
		t.Error("expected error for invalid key file verbose")
	}
}

func TestFindAuthAutoDetectVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{
		auth:    "",
		host:    "testhost",
		user:    "testuser",
		verbose: true,
	}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth auto-detect verbose failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

// ==================== 修复验证测试 ====================

func Test_readPassword_NonInteractive(t *testing.T) {
	// 在非交互模式下应该返回错误
	_, err := readPassword("Password: ")
	if err == nil {
		t.Error("Expected error in non-interactive mode")
	}
}

func Test_sshAgentAuth_NoSocket(t *testing.T) {
	// 保存原始环境
	origSocket := os.Getenv("SSH_AUTH_SOCK")
	defer os.Setenv("SSH_AUTH_SOCK", origSocket)
	
	// 清除环境变量
	os.Unsetenv("SSH_AUTH_SOCK")
	
	_, _, err := sshAgentAuth()
	if err == nil {
		t.Error("Expected error when SSH_AUTH_SOCK not set")
	}
}

func Test_findAuth_CleanupOnError(t *testing.T) {
	// 测试：无效密钥文件内容应该报错（文件存在但内容无效）
	tmpDir := t.TempDir()
	badKeyPath := filepath.Join(tmpDir, "bad_key")
	if err := os.WriteFile(badKeyPath, []byte("not a real key"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	a := args{
		auth: badKeyPath,
		host: "testhost",
		user: "testuser",
	}

	_, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}

	// 应该返回错误，因为密钥文件内容无效
	if err == nil {
		t.Error("Expected error for invalid key file content")
	}
}

func Test_findAuth_PasswordAuth(t *testing.T) {
	a := args{
		auth: "testpassword",
		host: "testhost",
		user: "testuser",
	}
	
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	
	if err != nil {
		t.Fatalf("findAuth failed: %v", err)
	}
	
	if len(methods) != 1 {
		t.Fatalf("Expected 1 auth method, got %d", len(methods))
	}
}