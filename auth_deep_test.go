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

// generateTestRSAKey 生成 RSA 测试密钥
func generateTestRSAKey(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_rsa")

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return keyPath
}

// generateTestECDSAKey 生成 ECDSA 测试密钥
func generateTestECDSAKey(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_ecdsa")

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	pkcs8Bytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return keyPath
}

// generateEncryptedKey 生成加密的 Ed25519 密钥
func generateEncryptedKey(t *testing.T, passphrase string) string {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "encrypted_key")

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs8Bytes, err := ssh.MarshalPrivateKey(priv, passphrase)
	if err != nil {
		t.Fatalf("marshal encrypted key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)

	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return keyPath
}

// ==================== loadKeyFile 更多测试 ====================

func TestLoadKeyFileRSA(t *testing.T) {
	keyPath := generateTestRSAKey(t)
	methods, err := loadKeyFile(keyPath, false)
	if err != nil {
		t.Fatalf("loadKeyFile RSA failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileECDSA(t *testing.T) {
	keyPath := generateTestECDSAKey(t)
	methods, err := loadKeyFile(keyPath, false)
	if err != nil {
		t.Fatalf("loadKeyFile ECDSA failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileRSAVerbose(t *testing.T) {
	keyPath := generateTestRSAKey(t)
	methods, err := loadKeyFile(keyPath, true)
	if err != nil {
		t.Fatalf("loadKeyFile RSA verbose failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileECDSAVerbose(t *testing.T) {
	keyPath := generateTestECDSAKey(t)
	methods, err := loadKeyFile(keyPath, true)
	if err != nil {
		t.Fatalf("loadKeyFile ECDSA verbose failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestLoadKeyFileEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "empty_key")
	os.WriteFile(keyPath, []byte(""), 0600)

	_, err := loadKeyFile(keyPath, false)
	if err == nil {
		t.Error("expected error for empty key file")
	}
}

func TestLoadKeyFileBinaryGarbage(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "garbage_key")
	os.WriteFile(keyPath, []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe}, 0600)

	_, err := loadKeyFile(keyPath, false)
	if err == nil {
		t.Error("expected error for binary garbage key file")
	}
}

func TestLoadKeyFilePermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "noaccess_key")
	os.WriteFile(keyPath, []byte("data"), 0000)

	_, err := loadKeyFile(keyPath, false)
	if err == nil {
		t.Error("expected error for permission denied")
	}
}

// ==================== autoDetectKey 更多测试 ====================

func TestAutoDetectKeyMultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 放置多个密钥，应优先使用 ed25519
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pkcs8Bytes, _ := ssh.MarshalPrivateKey(priv, "")
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)
	os.WriteFile(filepath.Join(sshDir, "id_ed25519"), pemBytes, 0600)

	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8Bytes2, _ := ssh.MarshalPrivateKey(privateKey, "")
	pemBytes2 := pem.EncodeToMemory(pkcs8Bytes2)
	os.WriteFile(filepath.Join(sshDir, "id_rsa"), pemBytes2, 0600)

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

func TestAutoDetectKeyOnlyRSA(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 只放 RSA 密钥
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8Bytes, _ := ssh.MarshalPrivateKey(privateKey, "")
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)
	os.WriteFile(filepath.Join(sshDir, "id_rsa"), pemBytes, 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(true)
	if err != nil {
		t.Fatalf("autoDetectKey RSA only failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyOnlyECDSA(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 只放 ECDSA 密钥
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pkcs8Bytes, _ := ssh.MarshalPrivateKey(privateKey, "")
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)
	os.WriteFile(filepath.Join(sshDir, "id_ecdsa"), pemBytes, 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	methods, err := autoDetectKey(true)
	if err != nil {
		t.Fatalf("autoDetectKey ECDSA only failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestAutoDetectKeyBadRSA(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	// 放一个损坏的 RSA 密钥和一个有效的 ECDSA 密钥
	os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("bad rsa"), 0600)

	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pkcs8Bytes, _ := ssh.MarshalPrivateKey(privateKey, "")
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)
	os.WriteFile(filepath.Join(sshDir, "id_ecdsa"), pemBytes, 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 应该跳过损坏的 RSA，使用 ECDSA
	methods, err := autoDetectKey(false)
	if err != nil {
		t.Fatalf("autoDetectKey should fall through to ECDSA: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

// ==================== findAuth 更多测试 ====================

func TestFindAuthWithRSAKeyFile(t *testing.T) {
	keyPath := generateTestRSAKey(t)
	a := args{auth: keyPath, host: "host", user: "user"}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth RSA key failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthWithECDSAKeyFile(t *testing.T) {
	keyPath := generateTestECDSAKey(t)
	a := args{auth: keyPath, host: "host", user: "user"}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth ECDSA key failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthWithRSAKeyVerbose(t *testing.T) {
	keyPath := generateTestRSAKey(t)
	a := args{auth: keyPath, host: "host", user: "user", verbose: true}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth RSA verbose failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthAutoDetectWithRSA(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	os.MkdirAll(sshDir, 0700)

	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8Bytes, _ := ssh.MarshalPrivateKey(privateKey, "")
	pemBytes := pem.EncodeToMemory(pkcs8Bytes)
	os.WriteFile(filepath.Join(sshDir, "id_rsa"), pemBytes, 0600)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	a := args{host: "host", user: "user"}
	methods, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("findAuth auto-detect RSA failed: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestFindAuthNoAuthSpecifiedNoKeys(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 没有密钥文件，findAuth 会尝试读取密码
	// 在非交互终端会失败，但至少不 panic
	a := args{host: "host", user: "user"}
	_, cleanup, err := findAuth(a)
	if cleanup != nil {
		defer cleanup()
	}
	// 如果系统有密钥可能成功，否则会失败
	if err != nil {
		t.Logf("findAuth without keys (expected in non-interactive): %v", err)
	}
}
