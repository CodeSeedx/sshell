package main

import (
	"testing"
)

// ==================== Bug #12: completion.go 补全脚本 ====================

func TestCompletionBashContainsFlagsExtended(t *testing.T) {
	script := generateCompletion("bash")
	if script == "" {
		t.Error("bash completion should not be empty")
	}
	
	// 检查是否包含关键标志
	flags := []string{"-p", "-u", "-a", "-k", "-v", "-V", "-C", "-J", "-L", "-R", "-D",
		"--put", "--get", "--sftp", "--log", "--save", "--delete", "--list",
		"--no-agent", "--reconnect", "--reconnect-max", "--help", "--version"}
	
	for _, flag := range flags {
		if !contains(script, flag) {
			t.Errorf("bash completion missing flag %s", flag)
		}
	}
}

func TestCompletionZshContainsFlagsExtended(t *testing.T) {
	script := generateCompletion("zsh")
	if script == "" {
		t.Error("zsh completion should not be empty")
	}
	
	flags := []string{"-p", "-u", "-a", "-k", "-v", "-V", "-C", "-J", "-L", "-R", "-D",
		"--put", "--get", "--sftp", "--log", "--save", "--delete", "--list",
		"--no-agent", "--reconnect", "--reconnect-max", "--help", "--version"}
	
	for _, flag := range flags {
		if !contains(script, flag) {
			t.Errorf("zsh completion missing flag %s", flag)
		}
	}
}

func TestCompletionFishContainsFlagsExtended(t *testing.T) {
	script := generateCompletion("fish")
	if script == "" {
		t.Error("fish completion should not be empty")
	}
	
	// fish 使用 -l 格式
	flags := []string{"put", "get", "sftp", "log", "save", "delete", "list",
		"no-agent", "reconnect", "reconnect-max", "help", "version"}
	
	for _, flag := range flags {
		if !contains(script, flag) {
			t.Errorf("fish completion missing flag %s", flag)
		}
	}
}

func TestCompletionUnknownShellExtended(t *testing.T) {
	script := generateCompletion("unknown")
	if script != "" {
		t.Errorf("unknown shell should return empty, got %d bytes", len(script))
	}
}

func TestCompletionBashSyntaxExtended(t *testing.T) {
	script := generateCompletion("bash")
	
	// 检查基本语法
	if !contains(script, "_sshell()") {
		t.Error("bash completion should define _sshell function")
	}
	if !contains(script, "complete -F _sshell sshell") {
		t.Error("bash completion should register completion function")
	}
}

func TestCompletionZshSyntaxExtended(t *testing.T) {
	script := generateCompletion("zsh")
	
	// 检查基本语法
	if !contains(script, "#compdef sshell") {
		t.Error("zsh completion should have #compdef directive")
	}
	if !contains(script, "_sshell()") {
		t.Error("zsh completion should define _sshell function")
	}
}

func TestCompletionFishSyntaxExtended(t *testing.T) {
	script := generateCompletion("fish")
	
	// 检查基本语法
	if !contains(script, "complete -c sshell") {
		t.Error("fish completion should use 'complete -c sshell'")
	}
}
