package main

import (
	"fmt"
	"os"
)

// generateCompletion 生成指定 shell 的补全脚本
func generateCompletion(shell string) string {
	switch shell {
	case "bash":
		return completionBash
	case "zsh":
		return completionZsh
	case "fish":
		return completionFish
	default:
		return ""
	}
}

// printCompletion 打印指定 shell 的补全脚本
func printCompletion(shell string) {
	script := generateCompletion(shell)
	if script == "" {
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s (supported: bash, zsh, fish)\n", shell)
		os.Exit(1)
	}
	fmt.Print(script)
}

const completionBash = `# sshell bash completion
_sshell() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-p -u -a -k -v -V -A -C -J -L -R -D --put --get --sftp --edit --log --save --delete --list --no-agent --agent-forward --insecure-host-key --reconnect --reconnect-max --help --version"

    case "${prev}" in
        -u|-a|-p|-k|-J|-L|-R|-D|--put|--get|--edit|--log|--save|--delete|--reconnect-max)
            return 0
            ;;
    esac

    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi

    # Complete bookmarks and SSH config hosts
    local bookmarks=""
    if [ -f ~/.sshell/bookmarks.json ]; then
        if command -v jq &>/dev/null; then
            bookmarks=$(jq -r 'keys[]' ~/.sshell/bookmarks.json 2>/dev/null)
        else
            # Fallback: extract top-level keys (lines with "key": { pattern)
            bookmarks=$(grep -oE '"[^"]+"\s*:\s*\{' ~/.sshell/bookmarks.json 2>/dev/null | sed 's/"//g; s/\s*:\s*{//' )
        fi
    fi
    local ssh_hosts=""
    if [ -f ~/.ssh/config ]; then
        ssh_hosts=$(grep -i "^Host " ~/.ssh/config 2>/dev/null | awk '{for(i=2;i<=NF;i++) if($i!="*") print $i}')
    fi
    COMPREPLY=( $(compgen -W "${bookmarks} ${ssh_hosts}" -- ${cur}) )
    return 0
}
complete -F _sshell sshell
`

const completionZsh = `#compdef sshell
_sshell() {
    local -a opts hosts bookmarks
    opts=(
        '-p[SSH port]:port'
        '-u[SSH username]:user'
        '-a[Password or key file]:auth'
        '-k[Keep-alive interval]:seconds'
        '-v[Verbose output]'
        '-V[Show version]'
        '-C[Enable compression]'
        '-A[Enable SSH Agent forwarding]'
        '-J[ProxyJump]:jump host'
        '-L[Local port forwarding]:spec'
        '-R[Remote port forwarding]:spec'
        '-D[SOCKS5 proxy]:port'
        '--put[Upload file]:local:remote'
        '--get[Download file]:remote:local'
        '--sftp[Use SFTP protocol for file transfers]'
        '--edit[Edit remote file with local editor]:path'
        '--log[Log session]:file'
        '--save[Save bookmark]:name'
        '--list[List bookmarks]'
        '--delete[Delete bookmark]:name'
        '--no-agent[Disable SSH Agent]'
        '--agent-forward[Enable SSH Agent forwarding]'
        '--insecure-host-key[Skip host key verification]'
        '--reconnect[Auto-reconnect]'
        '--reconnect-max[Max reconnects]:count'
        '--help[Show help]'
        '--version[Show version]'
    )

    # Collect bookmark names
    if [ -f ~/.sshell/bookmarks.json ]; then
        if (( $+commands[jq] )); then
            bookmarks=(${(f)"$(jq -r 'keys[]' ~/.sshell/bookmarks.json 2>/dev/null)"})
        else
            bookmarks=(${(f)"$(grep -oE '"[^"]+"\s*:\s*\{' ~/.sshell/bookmarks.json 2>/dev/null | sed 's/"//g; s/\s*:\s*{//')"})
        fi
    fi

    # Collect SSH config hosts
    if [ -f ~/.ssh/config ]; then
        hosts=(${(f)"$(grep -i '^Host ' ~/.ssh/config 2>/dev/null | awk '{for(i=2;i<=NF;i++) if($i!="*") print $i}')"})
    fi

    _arguments -s \
        "${opts[@]}" \
        ':host:->hosts' \
        '*::arg:->args'

    case $state in
        hosts) _describe 'host' "($hosts $bookmarks)" ;;
    esac
}

_sshell "$@"
`

const completionFish = `# sshell fish completion
complete -c sshell -f
complete -c sshell -s p -d 'SSH port' -r
complete -c sshell -s u -d 'SSH username' -r
complete -c sshell -s a -d 'Password or key file' -r
complete -c sshell -s k -d 'Keep-alive interval' -r
complete -c sshell -s v -d 'Verbose output'
complete -c sshell -s V -d 'Show version'
complete -c sshell -s C -d 'Enable compression'
complete -c sshell -s A -d 'Enable SSH Agent forwarding'
complete -c sshell -s J -d 'ProxyJump host' -r
complete -c sshell -s L -d 'Local port forwarding' -r
complete -c sshell -s R -d 'Remote port forwarding' -r
complete -c sshell -s D -d 'SOCKS5 proxy port' -r
complete -c sshell -l put -d 'Upload file (local:remote)' -r
complete -c sshell -l get -d 'Download file (remote:local)' -r
complete -c sshell -l sftp -d 'Use SFTP protocol for file transfers'
complete -c sshell -l edit -d 'Edit remote file with local editor' -r
complete -c sshell -l log -d 'Log session to file' -r
complete -c sshell -l save -d 'Save bookmark' -r
complete -c sshell -l list -d 'List bookmarks'
complete -c sshell -l delete -d 'Delete bookmark' -r
complete -c sshell -l no-agent -d 'Disable SSH Agent'
complete -c sshell -l agent-forward -d 'Enable SSH Agent forwarding'
complete -c sshell -l insecure-host-key -d 'Skip host key verification'
complete -c sshell -l reconnect -d 'Auto-reconnect'
complete -c sshell -l reconnect-max -d 'Max reconnect attempts' -r
complete -c sshell -l help -d 'Show help'
complete -c sshell -l version -d 'Show version'
`
