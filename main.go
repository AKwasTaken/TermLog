package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"termlog/internal/ipc"
	"termlog/internal/scrollback"
	"termlog/internal/session"
	"termlog/internal/termcheck"
	"termlog/internal/wrap"
)

const zshIntegration = `# termlog integration - added by "termlog install". Do not edit by hand;
# re-run "termlog install" to regenerate.
_termlog_preexec() {
  [ -n "$TERMLOG_SOCK" ] || return
  case "$1" in
    termlog\ *) return ;;
  esac
  printf '\033]133;C\007'
  command termlog __hook preexec --pwd "$PWD" --host "$(hostname -s)" --user "$USER" -- "$1"
}
_termlog_precmd() {
  local ec=$?
  [ -n "$TERMLOG_SOCK" ] || return
  printf '\033]133;D\007'
  command termlog __hook precmd "$ec"
}
autoload -Uz add-zsh-hook
add-zsh-hook preexec _termlog_preexec
add-zsh-hook precmd _termlog_precmd
`

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// These commands need a terminal termlog actually supports. install/help/__hook are exempt: install just writes config files, __hook is only ever invoked from inside an already-running, already-validated session.
	switch os.Args[1] {
	case "above", "below", "live", "offline", "online", "status", "quit":
		if ok, name := termcheck.Supported(); !ok {
			fmt.Fprintf(os.Stderr, "termlog: not supported in this terminal app (%s)\n", name)
			fmt.Fprintln(os.Stderr, "termlog currently works in Terminal.app, iTerm2, and tmux only.")
			os.Exit(1)
		}
	}

	var err error
	switch os.Args[1] {
	case "above":
		err = cmdAbove(argOrEmpty(2))
	case "below":
		err = wrap.Run(session.ModeBelow, argOrEmpty(2))
	case "live":
		err = wrap.Run(session.ModeLive, argOrEmpty(2))
	case "quit":
		err = cmdControl("quit")
	case "offline":
		err = cmdControl("offline")
	case "online":
		err = cmdControl("online")
	case "status":
		err = cmdStatus()
	case "install":
		err = cmdInstall()
	case "__hook":
		err = cmdHook(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "termlog:", err)
		os.Exit(1)
	}
}

func argOrEmpty(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

func usage() {
	fmt.Println(`termlog - transparent terminal session logger

Usage:
  termlog above [file]   Dump scrollback so far to file (one-shot)
  termlog live [file]    Dump scrollback, then record live from now on
  termlog below [file]   Record live from now on (no scrollback dump)
  termlog offline        Pause logging (session stays open)
  termlog online         Resume logging
  termlog status         Show current session state
  termlog quit           Stop recording and end the session
  termlog install        One-time zsh integration setup`)
}

func cmdAbove(file string) error {
	text, err := scrollback.Capture()
	if err != nil {
		return err
	}
	name := file
	if name == "" {
		name = session.DefaultFilename()
	}
	if err := os.WriteFile(name, []byte(text), 0644); err != nil {
		return err
	}
	abs, _ := filepath.Abs(name)
	fmt.Println("termlog: scrollback saved to", abs)
	return nil
}

func cmdControl(cmd string) error {
	sock, err := resolveSock()
	if err != nil {
		return err
	}
	resp, err := ipc.Send(sock, ipc.Request{Cmd: cmd})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	switch cmd {
	case "quit":
		fmt.Println("termlog: recording stopped")
	case "offline":
		fmt.Println("termlog: logging paused (offline)")
	case "online":
		fmt.Println("termlog: logging resumed (online)")
	}
	return nil
}

func cmdStatus() error {
	sock, err := resolveSock()
	if err != nil {
		fmt.Println("termlog: not recording in this terminal")
		return nil
	}
	resp, err := ipc.Send(sock, ipc.Request{Cmd: "status"})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	var s session.Session
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return err
	}
	fmt.Printf("termlog: %s (%s)\nfile:  %s\nsince: %s\n",
		s.State, s.Mode, s.File, s.StartedAt.Format(time.RFC1123))
	return nil
}

func cmdHook(args []string) error {
	if len(args) == 0 {
		return nil
	}
	sock := os.Getenv("TERMLOG_SOCK")
	if sock == "" {
		return nil // not inside a termlog session; no-op
	}
	switch args[0] {
	case "preexec":
		var pwd, host, user, cmdline string
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--pwd":
				i++
				if i < len(rest) {
					pwd = rest[i]
				}
			case "--host":
				i++
				if i < len(rest) {
					host = rest[i]
				}
			case "--user":
				i++
				if i < len(rest) {
					user = rest[i]
				}
			case "--":
				cmdline = strings.Join(rest[i+1:], " ")
				i = len(rest)
			}
		}
		_, err := ipc.Send(sock, ipc.Request{Cmd: "preexec", Arg: cmdline, PWD: pwd, Host: host, User: user})
		return err
	case "precmd":
		ec := ""
		if len(args) > 1 {
			ec = args[1]
		}
		_, err := ipc.Send(sock, ipc.Request{Cmd: "precmd", Arg: ec})
		return err
	}
	return nil
}

func cmdInstall() error {
	dir, err := session.Dir()
	if err != nil {
		return err
	}
	zshPath := filepath.Join(dir, "termlog.zsh")
	if err := os.WriteFile(zshPath, []byte(zshIntegration), 0644); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rcPath := filepath.Join(home, ".zshrc")
	marker := "# >>> termlog integration >>>"
	endMarker := "# <<< termlog integration <<<"

	existing, _ := os.ReadFile(rcPath)
	if strings.Contains(string(existing), marker) {
		fmt.Println("termlog: already installed in", rcPath)
		return nil
	}

	block := fmt.Sprintf("\n%s\n[ -f \"%s\" ] && source \"%s\"\n%s\n", marker, zshPath, zshPath, endMarker)
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return err
	}
	fmt.Println("termlog: installed. Restart your terminal (or run `source ~/.zshrc`) to activate.")
	return nil
}

// resolveSock finds the control socket for the termlog session running in the current terminal tab, whether we're being invoked from inside a wrapped shell (TERMLOG_SOCK is set) or from a plain shell (fall back to looking up the session by tty).
func resolveSock() (string, error) {
	if s := os.Getenv("TERMLOG_SOCK"); s != "" {
		return s, nil
	}
	tty, err := session.CurrentTTY()
	if err != nil {
		return "", err
	}
	_, sock, err := session.PathsFor(tty)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(sock); err != nil {
		return "", fmt.Errorf("no active termlog session on this terminal")
	}
	return sock, nil
}
