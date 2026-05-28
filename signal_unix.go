//go:build !windows

package main

import "syscall"

var (
	sigResize = syscall.SIGWINCH
	sigInterrupt = syscall.SIGINT
	sigTerminate = syscall.SIGTERM
)
