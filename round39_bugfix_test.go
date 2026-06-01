package main

import (
	"os"
	"testing"
)

// R39 Fix 1: replaceTokens %% 替换顺序
func Test_replaceTokens_DoublePercent_R39(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		hostname string
		user     string
		want     string
	}{
		{
			name:     "literal percent-h should not be expanded",
			path:     "~/.ssh/%%h_keys",
			hostname: "myhost",
			user:     "",
			want:     "~/.ssh/%h_keys", // 后续被 ~ 展开，这里先验证逻辑
		},
		{
			name:     "normal percent-h expansion",
			path:     "~/.ssh/%h_key",
			hostname: "example.com",
			user:     "",
			want:     "~/.ssh/example.com_key",
		},
		{
			name:     "mixed: literal percent and expanded",
			path:     "~/.ssh/%%h_%h_key",
			hostname: "host1",
			user:     "",
			want:     "~/.ssh/%h_host1_key",
		},
		{
			name:     "double percent alone",
			path:     "test%%file",
			hostname: "h",
			user:     "",
			want:     "test%file",
		},
		{
			name:     "percent-l expansion",
			path:     "keys/%l/%h",
			hostname: "remote",
			user:     "mylocal",
			want:     "keys/mylocal/remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceTokens(tt.path, tt.hostname, tt.user)
			// replaceTokens 会展开 ~，这里只测试 token 替换部分
			// 对包含 ~ 的用例，用 Contains 检查关键部分
			if tt.name == "literal percent-h should not be expanded" || tt.name == "normal percent-h expansion" || tt.name == "mixed: literal percent and expanded" {
				// 包含 ~ 的路径，检查替换后的关键部分
				if tt.name == "literal percent-h should not be expanded" {
					home, _ := os.UserHomeDir()
					expected := home + "/.ssh/%h_keys"
					if got != expected {
						t.Errorf("replaceTokens() = %q, want %q", got, expected)
					}
				} else if tt.name == "normal percent-h expansion" {
					home, _ := os.UserHomeDir()
					expected := home + "/.ssh/example.com_key"
					if got != expected {
						t.Errorf("replaceTokens() = %q, want %q", got, expected)
					}
				} else {
					home, _ := os.UserHomeDir()
					expected := home + "/.ssh/%h_host1_key"
					if got != expected {
						t.Errorf("replaceTokens() = %q, want %q", got, expected)
					}
				}
			} else {
				if got != tt.want {
					t.Errorf("replaceTokens() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}

// R39 Fix 2: getEditor VISUAL 优先于 EDITOR
func Test_getEditor_VisualPriority_R39(t *testing.T) {
	// 保存原始环境变量
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	defer func() {
		if origVisual != "" {
			os.Setenv("VISUAL", origVisual)
		} else {
			os.Unsetenv("VISUAL")
		}
		if origEditor != "" {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
	}()

	// 测试 1: VISUAL 优先于 EDITOR
	os.Setenv("VISUAL", "visual_editor")
	os.Setenv("EDITOR", "fallback_editor")
	if got := getEditor(); got != "visual_editor" {
		t.Errorf("getEditor() with both set = %q, want %q", got, "visual_editor")
	}

	// 测试 2: 只有 EDITOR 时使用 EDITOR
	os.Unsetenv("VISUAL")
	os.Setenv("EDITOR", "only_editor")
	if got := getEditor(); got != "only_editor" {
		t.Errorf("getEditor() with only EDITOR = %q, want %q", got, "only_editor")
	}

	// 测试 3: 只有 VISUAL 时使用 VISUAL
	os.Setenv("VISUAL", "only_visual")
	os.Unsetenv("EDITOR")
	if got := getEditor(); got != "only_visual" {
		t.Errorf("getEditor() with only VISUAL = %q, want %q", got, "only_visual")
	}

	// 测试 4: 都未设置时 fallback 到内置编辑器
	os.Unsetenv("VISUAL")
	os.Unsetenv("EDITOR")
	got := getEditor()
	if got == "" {
		t.Error("getEditor() returned empty when both unset")
	}
}
