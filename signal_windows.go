//go:build windows

package main

import "syscall"

var (
	sigResize     = syscall.Signal(0) // no SIGWINCH on Windows, use no-op
	sigInterrupt  = syscall.SIGINT
	sigTerminate  = syscall.SIGTERM
)
