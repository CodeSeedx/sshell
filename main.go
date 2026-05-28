package main

import (
	"fmt"
	"os"
)

func main() {
	a, err := parseArgsVerbose()
	if err != nil {
		if err.Error() == "help" {
			printUsage()
			os.Exit(0)
		}
		if err.Error() == "version" {
			fmt.Fprintln(os.Stderr, version)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	session, err := connect(a)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sshell] Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	if err := interactiveShell(session, a); err != nil {
		// 正常退出不算错误
		if err.Error() != "exit status 0" {
			fmt.Fprintf(os.Stderr, "[sshell] %v\n", err)
		}
	}
}
