sshell
=========

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![GitHub Release](https://img.shields.io/github/v/release/CodeSeedx/sshell)](https://github.com/CodeSeedx/sshell/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/CodeSeedx/sshell)](https://goreportcard.com/report/github.com/CodeSeedx/sshell)
[![Build](https://img.shields.io/github/actions/workflow/status/CodeSeedx/sshell/ci.yml?label=CI)](https://github.com/CodeSeedx/sshell/actions)

[中文文档](docs/README_ZH.md)

A lightweight SSH client written in Go. Single binary, no external runtime dependencies.

Download
--------

Download the binary for your platform from [GitHub Releases](https://github.com/CodeSeedx/sshell/releases):

- sshell-linux-amd64   — Linux x86_64
- sshell-linux-arm64   — Linux ARM64
- sshell-darwin-amd64  — macOS Intel
- sshell-darwin-arm64  — macOS Apple Silicon
- sshell-windows-amd64.exe — Windows

Build
-----

    make build          # Build for current platform
    make build-all      # Build for all 5 platforms

Usage
-----

    sshell -u <user> <host> [options]

    sshell -u root 192.168.1.100          # Login as root, auto-detect keys
    sshell -u root -p 2222 192.168.1.100  # Specify port
    sshell -u root -a ./id_rsa myhost     # Specify key file
    sshell -u root -a mypassword myhost   # Pass password directly
    sshell -u root -v 192.168.1.100       # Verbose output
    sshell -V                             # Show version

Options
-------

    -p <port>     SSH port, default 22
    -u <user>     SSH username (required)
    -a <auth>     Authentication: password or private key path
    -k <seconds>  TCP Keep-Alive interval, default 30s
    -v            Verbose output
    -V            Show version
    -h, --help    Show help

Features
--------

- SSH connection: TCP connect + SSH handshake + Keep-Alive
- Interactive terminal: PTY mode with window resize support
- Key authentication: Auto-detect keys in ~/.ssh/
- Password authentication: Auto-prompt for password, supports encrypted keys
- Host verification: Reads ~/.ssh/known_hosts, auto-accept on first connect
- Signal handling: Ctrl+C forwarding, SIGWINCH dynamic terminal resize
- Idle timeout: Auto-exit after 30 minutes of inactivity

Architecture
------------

    main.go      Entry point
    args.go      Argument parsing
    auth.go      Authentication logic
    connect.go   SSH connection
    shell.go     Interactive terminal
    version.go   Version (injected at build time)

Release
-------

Push a tag to main to trigger automatic release:

    git tag v1.0.0
    git push origin v1.0.0

GitHub Actions will automatically build binaries for all 5 platforms and publish to Release.

Dependencies
------------

    golang.org/x/crypto
    golang.org/x/term

License
-------

MIT
