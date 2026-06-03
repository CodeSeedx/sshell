package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

// bookmark 表示一个保存的 SSH 连接配置
type bookmark struct {
	Host              string     `json:"host"`
	Port              uint16     `json:"port,omitempty"`
	User              string     `json:"user,omitempty"`
	Auth              string     `json:"auth,omitempty"`
	Alive             uint32     `json:"alive,omitempty"`
	ProxyJump         string     `json:"proxy_jump,omitempty"`
	ProxyJumpUser     string     `json:"proxy_jump_user,omitempty"`
	ProxyJumps        []jumpHost `json:"proxy_jumps,omitempty"`
	Compress          bool       `json:"compress,omitempty"`
	AgentForward      bool       `json:"agent_forward,omitempty"`
	Cmd               string     `json:"cmd,omitempty"`
	InsecureHostKey   bool       `json:"insecure_host_key,omitempty"`
}

// bookmarkFile 返回书签文件路径 ~/.sshell/bookmarks.json
func bookmarkFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sshell", "bookmarks.json"), nil
}

// loadBookmarks 加载所有书签
func loadBookmarks() (map[string]bookmark, error) {
	path, err := bookmarkFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bookmark), nil
		}
		return nil, err
	}
	var bookmarks map[string]bookmark
	if err := json.Unmarshal(data, &bookmarks); err != nil {
		return nil, err
	}
	return bookmarks, nil
}

// saveBookmark 保存/更新一个书签
func saveBookmark(name string, b bookmark) error {
	bookmarks, err := loadBookmarks()
	if err != nil {
		return err
	}
	bookmarks[name] = b
	return writeBookmarks(bookmarks)
}

// deleteBookmark 删除一个书签
func deleteBookmark(name string) error {
	bookmarks, err := loadBookmarks()
	if err != nil {
		return err
	}
	if _, ok := bookmarks[name]; !ok {
		return fmt.Errorf("bookmark '%s' not found", name)
	}
	delete(bookmarks, name)
	return writeBookmarks(bookmarks)
}

// listBookmarks 打印所有书签
func listBookmarks() error {
	bookmarks, err := loadBookmarks()
	if err != nil {
		return err
	}
	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks saved.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tHOST\tPORT\tUSER\tPROXY\tINSECURE")
	for name, b := range bookmarks {
		port := b.Port
		if port == 0 {
			port = 22
		}
		insecure := ""
		if b.InsecureHostKey {
			insecure = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", name, b.Host, port, b.User, b.ProxyJump, insecure)
	}
	w.Flush()
	return nil
}

// argsToBookmark 将当前连接参数转为书签
func argsToBookmark(a args) bookmark {
	return bookmark{
		Host:            a.host,
		Port:            a.port,
		User:            a.user,
		Auth:            a.auth,
		Alive:           a.alive,
		ProxyJump:       a.proxyJump,
		ProxyJumpUser:   a.proxyJumpUser,
		ProxyJumps:      a.proxyJumps,
		Compress:        a.compress,
		AgentForward:    a.agentForward,
		Cmd:             a.cmd,
		InsecureHostKey: a.insecureHostKey,
	}
}

// bookmarkToArgs 将书签转回连接参数
func bookmarkToArgs(b bookmark) args {
	a := args{
		host:            b.Host,
		port:            b.Port,
		user:            b.User,
		auth:            b.Auth,
		alive:           b.Alive,
		proxyJump:       b.ProxyJump,
		proxyJumpUser:   b.ProxyJumpUser,
		proxyJumps:      b.ProxyJumps,
		compress:        b.Compress,
		agentForward:    b.AgentForward,
		cmd:             b.Cmd,
		insecureHostKey: b.InsecureHostKey,
	}
	// 应用默认值
	if a.port == 0 {
		a.port = 22
	}
	if a.alive == 0 {
		a.alive = 30
	}
	return a
}

// lookupBookmark 尝试通过书签名查找并返回连接参数
func lookupBookmark(name string) (args, bool) {
	bookmarks, err := loadBookmarks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Warning: failed to load bookmarks: %v\n", err)
		return args{}, false
	}
	b, ok := bookmarks[name]
	if !ok {
		return args{}, false
	}
	return bookmarkToArgs(b), true
}

// writeBookmarks 将书签写入文件（原子写入：先写临时文件再 rename，防止 crash 损坏）
func writeBookmarks(bookmarks map[string]bookmark) error {
	path, err := bookmarkFile()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bookmarks, "", "  ")
	if err != nil {
		return err
	}

	// 写入临时文件
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// 原子 rename
	return os.Rename(tmpPath, path)
}
