package wrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"termlog/internal/ansistrip"
	"termlog/internal/ipc"
	"termlog/internal/scrollback"
	"termlog/internal/session"
)

// OSC marker sequences printed by the zsh integration around each
// command. Terminals treat unrecognized OSC codes as control data to
// swallow, not text to display -- but we also strip these bytes
// ourselves before forwarding to the real terminal, so display
// behavior never depends on that being true.
const (
	startMarker  = "\x1b]133;C\x07"
	endMarker    = "\x1b]133;D\x07"
	markerMaxLen = 8 // len(startMarker) == len(endMarker)
)

type engine struct {
	mu    sync.Mutex
	state session.State

	// capturing/curOut/holdback are owned exclusively by the pty
	// read-loop goroutine in Run() -- only ever touched from there, so
	// they need no locking.
	capturing bool
	curOut    bytes.Buffer
	holdback  []byte

	logFile *os.File
	sess    *session.Session
	quitCh  chan struct{}
}

// Run starts a below/live recording session in the current terminal.
// It blocks until the session ends (via `termlog quit` or the wrapped
// shell exiting on its own), then returns control to the caller -- at
// which point the ORIGINAL outer shell (the one that ran `termlog
// below`) resumes exactly where it left off.
func Run(mode session.Mode, filename string) error {
	tty, err := session.CurrentTTY()
	if err != nil {
		return err
	}
	_, sockPath, err := session.PathsFor(tty)
	if err != nil {
		return err
	}

	if filename == "" {
		filename = session.DefaultFilename()
	}
	absFile, err := filepath.Abs(filename)
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(absFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	defer logFile.Close()

	if mode == session.ModeLive {
		text, scErr := scrollback.Capture()
		if scErr != nil {
			fmt.Fprintln(os.Stderr, "termlog: warning:", scErr, "(continuing with live recording only)")
		} else {
			logFile.WriteString(text)
			if !strings.HasSuffix(text, "\n") {
				logFile.WriteString("\n")
			}
		}
	}

	sess := &session.Session{
		TTY:       tty,
		File:      absFile,
		Mode:      mode,
		State:     session.Online,
		PID:       os.Getpid(),
		SockPath:  sockPath,
		StartedAt: time.Now(),
	}
	if err := sess.Save(); err != nil {
		return err
	}
	defer session.Remove(tty)

	e := &engine{state: session.Online, logFile: logFile, sess: sess, quitCh: make(chan struct{}, 1)}

	listener, err := ipc.Serve(sockPath, e.handle)
	if err != nil {
		return err
	}
	defer listener.Close()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	cmd := exec.Command(shell, "-i")
	cmd.Env = append(os.Environ(), "TERMLOG_SOCK="+sockPath, "TERMLOG_ACTIVE=1")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start shell in pty: %w", err)
	}
	defer ptmx.Close()

	pty.InheritSize(os.Stdin, ptmx)
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	defer signal.Stop(winch)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Fprintf(os.Stderr, "termlog: recording (%s) to %s\r\n", mode, absFile)

	go io.Copy(ptmx, os.Stdin)

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				clean := e.processChunk(buf[:n])
				os.Stdout.Write(clean)
			}
			if rerr != nil {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
		// wrapped shell exited on its own (e.g. user typed `exit`)
	case <-e.quitCh:
		cmd.Process.Signal(syscall.SIGHUP)
		ptmx.Close()
		cmd.Wait()
	}

	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Fprintf(os.Stderr, "\r\ntermlog: recording stopped, saved to %s\r\n", absFile)
	return nil
}

// processChunk scans a chunk of raw pty output for the start/end
// markers, updating capture state and buffering plain (non-marker)
// bytes into curOut while capturing. It returns the chunk with marker
// bytes removed, safe to forward straight to the real terminal.
//
// Markers can straddle two separate Read() calls, so any bytes at the
// tail of this chunk that could be the beginning of a marker are held
// back in e.holdback and reconsidered on the next call.
func (e *engine) processChunk(chunk []byte) []byte {
	data := append(e.holdback, chunk...)
	e.holdback = nil

	var out bytes.Buffer
	for {
		si := bytes.Index(data, []byte(startMarker))
		ei := bytes.Index(data, []byte(endMarker))

		idx, isStart := -1, false
		switch {
		case si == -1 && ei == -1:
			// no marker in what's left
		case si == -1:
			idx, isStart = ei, false
		case ei == -1:
			idx, isStart = si, true
		case si < ei:
			idx, isStart = si, true
		default:
			idx, isStart = ei, false
		}

		if idx == -1 {
			break
		}

		plain := data[:idx]
		out.Write(plain)

		if e.capturing {
			e.appendCapture(plain)
		}

		if isStart {
			e.beginCapture()
			data = data[idx+len(startMarker):]
		} else {
			e.endCapture()
			data = data[idx+len(endMarker):]
		}
	}

	holdLen := partialMarkerSuffixLen(data)
	keep := data[:len(data)-holdLen]
	e.holdback = append(e.holdback, data[len(data)-holdLen:]...)

	out.Write(keep)

	if e.capturing {
		e.appendCapture(keep)
	}

	return out.Bytes()
}

// partialMarkerSuffixLen returns the length of the longest suffix of
// data that is a proper prefix of either marker -- i.e. bytes that
// might be the start of a marker split across a read boundary.
func partialMarkerSuffixLen(data []byte) int {
	max := markerMaxLen - 1
	if max > len(data) {
		max = len(data)
	}
	for l := max; l > 0; l-- {
		suffix := string(data[len(data)-l:])
		if strings.HasPrefix(startMarker, suffix) || strings.HasPrefix(endMarker, suffix) {
			return l
		}
	}
	return 0
}

func (e *engine) appendCapture(b []byte) {
	if e.capturing && len(b) > 0 {
		e.curOut.Write(b)
	}
}

func (e *engine) beginCapture() {
	e.capturing = true
	e.curOut.Reset()
}

func (e *engine) endCapture() {
	if !e.capturing {
		return
	}
	e.capturing = false
	raw := e.curOut.String()
	e.curOut.Reset()

	e.mu.Lock()
	online := e.state == session.Online
	e.mu.Unlock()
	if !online {
		return
	}

	out := ansistrip.TrimBlankEdges(ansistrip.Strip(raw))
	if out == "" {
		return
	}
	e.mu.Lock()
	fmt.Fprintln(e.logFile, out)
	e.mu.Unlock()
}

func (e *engine) handle(req ipc.Request) ipc.Response {
	switch req.Cmd {
	case "preexec":
		prompt := formatPrompt(req.User, req.Host, req.PWD)
		cmdline := req.Arg

		e.mu.Lock()
		online := e.state == session.Online
		e.mu.Unlock()
		if online {
			e.mu.Lock()
			fmt.Fprintf(e.logFile, "%s %s\n", prompt, cmdline)
			e.mu.Unlock()
		}
		return ipc.Response{OK: true}

	case "precmd":
		// Output boundaries are now determined by in-stream markers
		// (see processChunk), not by hook arrival timing -- this
		// eliminates the pty-stream/socket race entirely. This handler
		// is kept for protocol compatibility and future use (e.g.
		// annotating non-zero exit codes).
		return ipc.Response{OK: true}

	case "offline":
		e.mu.Lock()
		e.state = session.Offline
		e.sess.State = session.Offline
		saveErr := e.sess.Save()
		e.mu.Unlock()
		if saveErr != nil {
			return ipc.Response{OK: false, Error: saveErr.Error()}
		}
		return ipc.Response{OK: true}

	case "online":
		e.mu.Lock()
		e.state = session.Online
		e.sess.State = session.Online
		saveErr := e.sess.Save()
		e.mu.Unlock()
		if saveErr != nil {
			return ipc.Response{OK: false, Error: saveErr.Error()}
		}
		return ipc.Response{OK: true}

	case "status":
		e.mu.Lock()
		e.sess.State = e.state
		snapshot := *e.sess
		e.mu.Unlock()
		data, _ := json.Marshal(snapshot)
		return ipc.Response{OK: true, Data: data}

	case "quit":
		select {
		case e.quitCh <- struct{}{}:
		default:
		}
		return ipc.Response{OK: true}

	case "mark":
		e.mu.Lock()
		online := e.state == session.Online
		e.mu.Unlock()

		if online {
			e.mu.Lock()
			fmt.Fprintf(e.logFile, "\n\n%s\n\n\n", req.Arg)
			e.mu.Unlock()
		}
		return ipc.Response{OK: true}
	}
	return ipc.Response{OK: false, Error: "unknown command"}
}

// formatPrompt mimics a typical zsh prompt: "user@host dir %"
func formatPrompt(user, host, pwd string) string {
	home, _ := os.UserHomeDir()
	dir := pwd
	if pwd == home {
		dir = "~"
	} else if pwd != "" {
		dir = filepath.Base(pwd)
	}
	sym := "%"
	if os.Geteuid() == 0 {
		sym = "#"
	}
	return fmt.Sprintf("%s@%s %s %s", user, host, dir, sym)
}
