package main

import "testing"

func TestGenerateCompletionBash(t *testing.T) {
	script := generateCompletion("bash")
	if script == "" {
		t.Error("bash completion should not be empty")
	}
	if len(script) < 100 {
		t.Error("bash completion seems too short")
	}
}

func TestGenerateCompletionZsh(t *testing.T) {
	script := generateCompletion("zsh")
	if script == "" {
		t.Error("zsh completion should not be empty")
	}
}

func TestGenerateCompletionFish(t *testing.T) {
	script := generateCompletion("fish")
	if script == "" {
		t.Error("fish completion should not be empty")
	}
}

func TestGenerateCompletionUnknown(t *testing.T) {
	script := generateCompletion("unknown")
	if script != "" {
		t.Errorf("unknown shell should return empty, got %d bytes", len(script))
	}
}

func TestCompletionContainsFlags(t *testing.T) {
	// bash 和 zsh 使用 --flag 格式
	for _, shell := range []string{"bash", "zsh"} {
		script := generateCompletion(shell)
		for _, flag := range []string{"--put", "--get", "--log", "--save", "-J", "-L", "-R", "-D"} {
			if !contains(script, flag) {
				t.Errorf("%s completion missing flag %s", shell, flag)
			}
		}
	}

	// fish 使用 -l flag 格式（长选项用 -l 而不是 --）
	fishScript := generateCompletion("fish")
	for _, flag := range []string{"put", "get", "log", "save", "J", "L", "R", "D"} {
		if !contains(fishScript, flag) {
			t.Errorf("fish completion missing flag %s", flag)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
