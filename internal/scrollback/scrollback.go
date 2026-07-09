package scrollback

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"termlog/internal/ansistrip"
	"termlog/internal/session"
)

func Capture() (string, error) {
	var text string
	var err error
	switch {
	case os.Getenv("TMUX") != "":
		text, err = captureTmux()
	case os.Getenv("TERM_PROGRAM") == "iTerm.app":
		text, err = captureITerm()
	case os.Getenv("TERM_PROGRAM") == "Apple_Terminal":
		text, err = captureAppleTerminal()
	default:
		return "", fmt.Errorf("scrollback capture isn't supported in this terminal (TERM_PROGRAM=%q)", os.Getenv("TERM_PROGRAM"))
	}
	if err != nil {
		return "", err
	}
	return trimTrailingBlankLines(text), nil
}

func trimTrailingBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[:end], "\n") + "\n"
}

// runOsascript captures stderr explicitly -- otherwise callers only ever see a bare "exit status 1" instead of AppleScript's real error.
func runOsascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func captureTmux() (string, error) {
	pane := os.Getenv("TMUX_PANE")
	args := []string{"capture-pane", "-p", "-S", "-10000"}
	if pane != "" {
		args = append(args, "-t", pane)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("tmux", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tmux capture-pane failed: %s", msg)
	}
	return ansistrip.Strip(stdout.String()), nil
}

func captureITerm() (string, error) {
	ttyPath, err := session.CurrentTTY()
	if err != nil {
		return "", err
	}
	script := `
set targetTTY to "` + ttyPath + `"
tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if (tty of s) is targetTTY then
					return contents of s
				end if
			end repeat
		end repeat
	end repeat
end tell
return ""
`
	out, err := runOsascript(script)
	if err != nil {
		return "", fmt.Errorf("iTerm2 AppleScript capture failed: %s (check System Settings > Privacy & Security > Automation)", err)
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("iTerm2 returned no scrollback -- Automation permission may not be granted yet, or no matching session/tty was found")
	}
	// NOTE: iTerm2's AppleScript `contents` only returns the currently VISIBLE pane text, not the full scrollback buffer. True full scrollback needs iTerm2's Python API instead. So I havent added that yet. Maybe in the next version.
	return ansistrip.Strip(out), nil
}

func captureAppleTerminal() (string, error) {
	ttyPath, err := session.CurrentTTY()
	if err != nil {
		return "", err
	}
	// Terminal.app's exact scrollback property name isn't something I could verify without a real macOS box, so try "history" first, then fall back to "contents".
	var lastErr error
	for _, prop := range []string{"history", "contents"} {
		script := `
set targetTTY to "` + ttyPath + `"
tell application "Terminal"
	repeat with w in windows
		repeat with tb in tabs of w
			try
				if (tty of tb) is targetTTY then
					return ` + prop + ` of tb
				end if
			end try
		end repeat
	end repeat
end tell
return ""
`
		out, err := runOsascript(script)
		if err == nil && strings.TrimSpace(out) != "" {
			return ansistrip.Strip(out), nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("Terminal.app AppleScript capture failed: %s (check System Settings > Privacy & Security > Automation)", lastErr)
	}
	return "", fmt.Errorf("Terminal.app returned no scrollback for this tab")
}
