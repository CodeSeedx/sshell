package main

import (
	"testing"
)

func TestParseArgsWithCommand(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantCmd string
		wantH   string
	}{
		{
			name:    "single word command",
			argv:    []string{"-u", "root", "host", "uptime"},
			wantCmd: "uptime",
			wantH:   "host",
		},
		{
			name:    "quoted command",
			argv:    []string{"-u", "root", "host", "df -h"},
			wantCmd: "df -h",
			wantH:   "host",
		},
		{
			name:    "multiple word command",
			argv:    []string{"-u", "root", "host", "tail", "-n", "20", "/var/log/syslog"},
			wantCmd: "tail -n 20 /var/log/syslog",
			wantH:   "host",
		},
		{
			name:    "no command",
			argv:    []string{"-u", "root", "host"},
			wantCmd: "",
			wantH:   "host",
		},
		{
			name:    "command with flags before host",
			argv:    []string{"-u", "root", "-p", "2222", "host", "free", "-m"},
			wantCmd: "free -m",
			wantH:   "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := parseArgsFrom(tt.argv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if a.host != tt.wantH {
				t.Errorf("host = %q, want %q", a.host, tt.wantH)
			}
			if a.cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", a.cmd, tt.wantCmd)
			}
		})
	}
}
