package main

import (
	"testing"
)

func Test_DoubleDashSeparator(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantHost string
		wantCmd  string
		wantUser string
	}{
		{
			name:     "-- between host and command",
			argv:     []string{"-u", "root", "myhost", "--", "ls", "-la"},
			wantHost: "myhost",
			wantCmd:  "ls -la",
			wantUser: "root",
		},
		{
			name:     "-- before host",
			argv:     []string{"-u", "root", "--", "myhost", "ls", "-la"},
			wantHost: "myhost",
			wantCmd:  "ls -la",
			wantUser: "root",
		},
		{
			name:     "-- with single command arg",
			argv:     []string{"-u", "root", "myhost", "--", "uptime"},
			wantHost: "myhost",
			wantCmd:  "uptime",
			wantUser: "root",
		},
		{
			name:     "-- with no args after",
			argv:     []string{"-u", "root", "myhost", "--"},
			wantHost: "myhost",
			wantCmd:  "",
			wantUser: "root",
		},
		{
			name:     "-- stops flag parsing (args after -- not treated as flags)",
			argv:     []string{"-u", "root", "myhost", "--", "-v", "-p", "2222"},
			wantHost: "myhost",
			wantCmd:  "-v -p 2222",
			wantUser: "root",
		},
		{
			name:     "no -- (normal parsing)",
			argv:     []string{"-u", "root", "myhost", "ls", "-la"},
			wantHost: "myhost",
			wantCmd:  "ls -la",
			wantUser: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := parseArgsFrom(tt.argv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if a.host != tt.wantHost {
				t.Errorf("host = %q, want %q", a.host, tt.wantHost)
			}
			if a.cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", a.cmd, tt.wantCmd)
			}
			if a.user != tt.wantUser {
				t.Errorf("user = %q, want %q", a.user, tt.wantUser)
			}
		})
	}
}

func Test_DoubleDashSeparator_ArgsAfterNotParsedAsFlags(t *testing.T) {
	// 确保 -- 后的 -v 不会被解析为 verbose 标志
	a, err := parseArgsFrom([]string{"-u", "root", "host1", "--", "-v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.verbose {
		t.Error("verbose should be false after --")
	}
	if a.host != "host1" {
		t.Errorf("host = %q, want %q", a.host, "host1")
	}
	if a.cmd != "-v" {
		t.Errorf("cmd = %q, want %q", a.cmd, "-v")
	}
}

func Test_DoubleDashSeparator_OnlyDoubleDash(t *testing.T) {
	// 只有 --，没有后续参数
	a, err := parseArgsFrom([]string{"-u", "root", "host1", "--"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.host != "host1" {
		t.Errorf("host = %q, want %q", a.host, "host1")
	}
	if a.cmd != "" {
		t.Errorf("cmd = %q, want empty", a.cmd)
	}
}
