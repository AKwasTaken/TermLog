// Package ansistrip removes terminal escape sequences from captured output so log files contain clean, readable text.
package ansistrip

import (
	"regexp"
	"strings"
)

var (
	csi     = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	osc     = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
	charset = regexp.MustCompile(`\x1b[()][0-9A-Za-z]`)
	single  = regexp.MustCompile(`\x1b[=>78MDEc]`)
)

// Strip removes ANSI/VT escape sequences and resolves carriage-return overwrites (progress bars, spinners) the same way a real terminal renders them, so only the final visible text ends up in the log.
func Strip(s string) string {
	s = csi.ReplaceAllString(s, "")
	s = osc.ReplaceAllString(s, "")
	s = charset.ReplaceAllString(s, "")
	s = single.ReplaceAllString(s, "")

	// Normalize ordinary CRLF line endings to LF first -- otherwise every normal line ending gets mistaken for a progress-bar-style \r overwrite by the loop below and gets wiped out.
	s = strings.ReplaceAll(s, "\r\n", "\n")

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.Contains(line, "\r") {
			parts := strings.Split(line, "\r")
			lines[i] = parts[len(parts)-1]
		}
	}
	s = strings.Join(lines, "\n")

	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TrimBlankEdges(s string) string {
	lines := strings.Split(s, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}
