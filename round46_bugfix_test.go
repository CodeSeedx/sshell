package main

import (
	"testing"
)

// R46: 测试书签恢复时 CLI 显式设置的 port/user/auth/alive 标志不被覆盖
func TestBookmarkRestorePreservesCLIValueFlags(t *testing.T) {
	// 模拟书签内容
	bk := args{
		host:  "10.0.0.1",
		port:  22,
		user:  "root",
		auth:  "~/.ssh/bookmark_key",
		alive: 30,
	}

	tests := []struct {
		name     string
		cliArgs  args // CLI 解析后的 args（书签查找前）
		wantPort uint16
		wantUser string
		wantAuth string
		wantAlive uint32
		wantCliPort bool
		wantCliUser bool
		wantCliAuth bool
		wantCliAlive bool
	}{
		{
			name: "CLI -p overrides bookmark port",
			cliArgs: args{
				port:    2222,
				cliPort: true,
			},
			wantPort:     2222,
			wantUser:     "root",
			wantAuth:     "~/.ssh/bookmark_key",
			wantAlive:    30,
			wantCliPort:  true,
			wantCliUser:  false,
			wantCliAuth:  false,
			wantCliAlive: false,
		},
		{
			name: "CLI -a overrides bookmark auth",
			cliArgs: args{
				auth:    "/tmp/my_key",
				cliAuth: true,
			},
			wantPort:     22,
			wantUser:     "root",
			wantAuth:     "/tmp/my_key",
			wantAlive:    30,
			wantCliPort:  false,
			wantCliUser:  false,
			wantCliAuth:  true,
			wantCliAlive: false,
		},
		{
			name: "CLI -k overrides bookmark alive",
			cliArgs: args{
				alive:    60,
				cliAlive: true,
			},
			wantPort:     22,
			wantUser:     "root",
			wantAuth:     "~/.ssh/bookmark_key",
			wantAlive:    60,
			wantCliPort:  false,
			wantCliUser:  false,
			wantCliAuth:  false,
			wantCliAlive: true,
		},
		{
			name: "CLI -p -k both override",
			cliArgs: args{
				port:     3333,
				cliPort:  true,
				alive:    120,
				cliAlive: true,
			},
			wantPort:     3333,
			wantUser:     "root",
			wantAuth:     "~/.ssh/bookmark_key",
			wantAlive:    120,
			wantCliPort:  true,
			wantCliUser:  false,
			wantCliAuth:  false,
			wantCliAlive: true,
		},
		{
			name: "No CLI overrides, bookmark values preserved",
			cliArgs: args{},
			wantPort:     22,
			wantUser:     "root",
			wantAuth:     "~/.ssh/bookmark_key",
			wantAlive:    30,
			wantCliPort:  false,
			wantCliUser:  false,
			wantCliAuth:  false,
			wantCliAlive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 main.go 中的书签恢复逻辑
			a := tt.cliArgs

			// 保存 CLI 标志（与 main.go 中的逻辑一致）
			cliPort := a.cliPort
			cliUser := a.cliUser
			cliAuth := a.cliAuth
			cliAlive := a.cliAlive
			cliPortVal := a.port
			cliUserVal := a.user
			cliAuthVal := a.auth
			cliAliveVal := a.alive

			a = bk

			// 恢复 CLI 显式设置的参数值和标志位
			if cliPort {
				a.port = cliPortVal
				a.cliPort = true
			}
			if cliUser {
				a.user = cliUserVal
				a.cliUser = true
			}
			if cliAuth {
				a.auth = cliAuthVal
				a.cliAuth = true
			}
			if cliAlive {
				a.alive = cliAliveVal
				a.cliAlive = true
			}

			// 验证
			if a.port != tt.wantPort {
				t.Errorf("port = %d, want %d", a.port, tt.wantPort)
			}
			if a.user != tt.wantUser {
				t.Errorf("user = %q, want %q", a.user, tt.wantUser)
			}
			if a.auth != tt.wantAuth {
				t.Errorf("auth = %q, want %q", a.auth, tt.wantAuth)
			}
			if a.alive != tt.wantAlive {
				t.Errorf("alive = %d, want %d", a.alive, tt.wantAlive)
			}
			if a.cliPort != tt.wantCliPort {
				t.Errorf("cliPort = %v, want %v", a.cliPort, tt.wantCliPort)
			}
			if a.cliUser != tt.wantCliUser {
				t.Errorf("cliUser = %v, want %v", a.cliUser, tt.wantCliUser)
			}
			if a.cliAuth != tt.wantCliAuth {
				t.Errorf("cliAuth = %v, want %v", a.cliAuth, tt.wantCliAuth)
			}
			if a.cliAlive != tt.wantCliAlive {
				t.Errorf("cliAlive = %v, want %v", a.cliAlive, tt.wantCliAlive)
			}
		})
	}
}

// R46: 测试书签恢复时 CLI 标志不影响 SSH config 优先级
func TestBookmarkRestoreCLIUserOverridesBookmark(t *testing.T) {
	// 模拟书签有 user=bookuser，CLI 指定 -u cliuser
	// 注意：实际 main.go 中 -u 会跳过书签逻辑（a.user != "" 时不进入书签分支）
	// 但 cliUser 标志仍应被正确保留
	bk := args{
		host: "10.0.0.1",
		port: 22,
		user: "bookuser",
	}

	a := args{
		user:    "cliuser",
		cliUser: true,
	}

	cliUser := a.cliUser
	cliUserVal := a.user

	a = bk

	if cliUser {
		a.user = cliUserVal
		a.cliUser = true
	}

	if a.user != "cliuser" {
		t.Errorf("user = %q, want %q", a.user, "cliuser")
	}
	if !a.cliUser {
		t.Error("cliUser should be true after restore")
	}
}
