// Package session tracks per-terminal termlog state so multiple tabs can each run their own independent recording session without stepping on each other.
package session

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type State string

const (
	Online  State = "online"
	Offline State = "offline"
)

type Mode string

const (
	ModeBelow Mode = "below"
	ModeLive  Mode = "live"
)

type Session struct {
	TTY       string    `json:"tty"`
	File      string    `json:"file"`
	Mode      Mode      `json:"mode"`
	State     State     `json:"state"`
	PID       int       `json:"pid"`
	SockPath  string    `json:"sock_path"`
	StartedAt time.Time `json:"started_at"`
}

// Dir returns ~/.termlog, creating the sessions/sockets subdirectories if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".termlog")
	for _, sub := range []string{"sessions", "sockets"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// CurrentTTY shells out to the `tty` binary to get the controlling terminal's device path. This works identically on macOS and Linux and needs no cgo or raw syscalls.
func CurrentTTY() (string, error) {
	cmd := exec.Command("tty")
	cmd.Stdin = os.Stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("not attached to a terminal: %s", msg)
	}
	tty := strings.TrimSpace(string(out))
	if tty == "" || tty == "not a tty" {
		return "", fmt.Errorf("not attached to a terminal")
	}
	return tty, nil
}

// Key turns a tty path like /dev/ttys003 into a short filesystem-safe id.
func Key(tty string) string {
	sum := sha1.Sum([]byte(tty))
	return hex.EncodeToString(sum[:])[:12]
}

// PathsFor returns the session-state file and control-socket paths for a given tty.
func PathsFor(tty string) (sessionFile, sockFile string, err error) {
	dir, err := Dir()
	if err != nil {
		return "", "", err
	}
	k := Key(tty)
	return filepath.Join(dir, "sessions", k+".json"), filepath.Join(dir, "sockets", k+".sock"), nil
}

func Load(tty string) (*Session, error) {
	f, _, err := PathsFor(tty)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Session) Save() error {
	f, _, err := PathsFor(s.TTY)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f, b, 0600)
}

func Remove(tty string) error {
	f, sock, err := PathsFor(tty)
	if err != nil {
		return err
	}
	os.Remove(f)
	os.Remove(sock)
	return nil
}

// DefaultFilename builds the default log filename, e.g. termlog_2026_July_08_21_34_51.log
func DefaultFilename() string {
	return "termlog_" + time.Now().Format("2006_January_02_15_04_05") + ".log"
}
