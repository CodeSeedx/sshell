package main

import (
	"fmt"
	"os"
	"strconv"
)

// 版本号，编译时通过 ldflags 注入: -X main.version=x.x.x
var version = "dev"

type args struct {
	host    string
	port    uint16
	user    string
	auth    string
	alive   uint32
	verbose bool
}

func parseArgs() args {
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return a
}

func parseArgsVerbose() (args, error) {
	a, err := parseArgsFrom(os.Args[1:])
	if err != nil {
		return a, err
	}
	return a, nil
}

// parseArgsFrom 解析参数，出错返回 error 而不是退出，便于测试
func parseArgsFrom(argv []string) (args, error) {
	var a args
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "-p":
			if i+1 < len(argv) {
				p, err := strconv.ParseUint(argv[i+1], 10, 16)
				if err != nil {
					return a, fmt.Errorf("invalid port: %s", argv[i+1])
				}
				a.port = uint16(p)
				i++
			}
		case "-u":
			if i+1 < len(argv) {
				a.user = argv[i+1]
				i++
			}
		case "-a":
			if i+1 < len(argv) {
				a.auth = argv[i+1]
				i++
			}
		case "-k":
			if i+1 < len(argv) {
				p, err := strconv.ParseUint(argv[i+1], 10, 32)
				if err != nil {
					return a, fmt.Errorf("invalid keep-alive: %s", argv[i+1])
				}
				a.alive = uint32(p)
				i++
			}
		case "-v":
			a.verbose = true
		case "-h", "--help":
			return a, fmt.Errorf("help")
		case "-V", "--version":
			return a, fmt.Errorf("version")
		default:
			if argv[i][0] != '-' && a.host == "" {
				a.host = argv[i]
			}
		}
	}

	if a.port == 0 {
		a.port = 22
	}
	if a.alive == 0 {
		a.alive = 30
	}

	if a.host == "" || a.user == "" {
		return a, fmt.Errorf("Usage: sshell -u <user> <host> [options]")
	}
	return a, nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: sshell -u <user> <host> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -p <port>     SSH port (default: 22)")
	fmt.Fprintln(os.Stderr, "  -u <user>     SSH username (required)")
	fmt.Fprintln(os.Stderr, "  -a <auth>     Password or path to private key file")
	fmt.Fprintln(os.Stderr, "  -k <seconds>  Keep-alive interval (default: 30)")
	fmt.Fprintln(os.Stderr, "  -v            Verbose output")
	fmt.Fprintln(os.Stderr, "  -V            Show version")
}
