// Package termcheck detects whether the current terminal application is one termlog knows how to work with. termlog below/live need a real pty-capable shell environment; above/live's scrollback dump additionally needs a terminal-specific capture mechanism. VS Code's integrated terminal, and anything else outside this list, isn't supported yet.
package termcheck

import "os"

// Supported reports whether the current terminal app is one termlog works in, along with a human-readable name for messaging.
func Supported() (bool, string) {
	switch {
	case os.Getenv("TMUX") != "":
		return true, "tmux"
	case os.Getenv("TERM_PROGRAM") == "iTerm.app":
		return true, "iTerm2"
	case os.Getenv("TERM_PROGRAM") == "Apple_Terminal":
		return true, "Terminal.app"
	default:
		name := os.Getenv("TERM_PROGRAM")
		if name == "" {
			name = "unknown terminal"
		}
		return false, name
	}
}
