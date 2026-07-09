package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Structural container to fetch counts from JXA cleanly
type TabLineCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// Fetches a map of tab identifiers and their current absolute line lengths
func getTerminalLineCounts() (map[string]int, error) {
	termApp := os.Getenv("TERM_PROGRAM")
	var script string

	if termApp == "iTerm.app" {
		script = `
            var results = [];
            var term = Application("iTerm2");
            for (var w = 0; w < term.windows.length; w++) {
                var win = term.windows[w];
                for (var t = 0; t < win.tabs.length; t++) {
                    var tab = win.tabs[t];
                    for (var s = 0; s < tab.sessions.length; s++) {
                        results.push({ id: "w"+w+"t"+t+"s"+s, count: tab.sessions[s].numberOfLines() });
                    }
                }
            }
            JSON.stringify(results);
        `
	} else {
		script = `
            var results = [];
            var term = Application("Terminal");
            for (var w = 0; w < term.windows.length; w++) {
                var win = term.windows[w];
                for (var t = 0; t < win.tabs.length; t++) {
                    var tab = win.tabs[t];
                    // Native terminal fallback calculation approximation
                    var histLines = tab.history().split("\n").length;
                    results.push({ id: "w"+w+"t"+t, count: histLines });
                }
            }
            JSON.stringify(results);
        `
	}

	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var counts []TabLineCount
	_ = json.Unmarshal(output, &counts)

	resMap := make(map[string]int)
	for _, item := range counts {
		resMap[item.ID] = item.Count
	}
	return resMap, nil
}

// Pulls ONLY the delta range of text added between startLine and endLine indices
func getTerminalDeltaText(tabID string, startLine int, endLine int) (string, error) {
	termApp := os.Getenv("TERM_PROGRAM")
	var script string

	// Parse coordinates out of our compound tab ID token keys
	if termApp == "iTerm.app" {
		var w, t, s int
		_, _ = fmt.Sscanf(tabID, "w%dt%ds%d", &w, &t, &s)

		// iTerm JXA lets us read lines cleanly by setting starting and ending indices
		script = fmt.Sprintf(`
            var term = Application("iTerm2");
            var session = term.windows[%d].tabs[%d].sessions[%d];
            var totalLines = session.numberOfLines();
            var outLines = [];
            // Safe guard layout bounds checking
            var end = Math.min(%d, totalLines);
            for (var i = %d; i < end; i++) {
                outLines.push(session.getLineAt({index: i}).text);
            }
            outLines.join("\n");
        `, w, t, s, endLine, startLine)
	} else {
		var w, t int
		_, _ = fmt.Sscanf(tabID, "w%dt%d", &w, &t)
		script = fmt.Sprintf(`
            var term = Application("Terminal");
            var hist = term.windows[%d].tabs[%d].history();
            var lines = hist.split("\n");
            lines.slice(%d, %d).join("\n");
        `, w, t, startLine, endLine)
	}

	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// Keep your existing working anchor and path helpers below unchanged
func findActiveWindowAnchor(state *GlobalState) string {
	termApp := os.Getenv("TERM_PROGRAM")
	var script string
	if termApp == "iTerm.app" {
		script = `var term = Application("iTerm2"); term.currentWindow.currentSession.text();`
	} else {
		script = `tell application "Terminal" to tell front window to tell selected tab to get history`
	}
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		// Fallback context validation routing string
		cmdApple := exec.Command("osascript", "-e", script)
		output, err = cmdApple.Output()
		if err != nil {
			return ""
		}
	}
	buffer := strings.ReplaceAll(string(output), "\r", "\n")
	for anchor := range state.Sessions {
		if strings.Contains(buffer, anchor) {
			return anchor
		}
	}
	return ""
}

func getTerminalBufferMap() (map[string]string, error) {
	termApp := os.Getenv("TERM_PROGRAM")
	var script string
	if termApp == "iTerm.app" {
		script = `var results = []; var term = Application("iTerm2"); for (var w=0; w<term.windows.length; w++) { var win = term.windows[w]; for (var t=0; t<win.tabs.length; t++) { var tab = win.tabs[t]; for (var s=0; s<tab.sessions.length; s++) { results.push(tab.sessions[s].text()); } } } JSON.stringify(results);`
	} else {
		script = `var results = []; var term = Application("Terminal"); for (var w=0; w<term.windows.length; w++) { var win = term.windows[w]; for (var t=0; t<win.tabs.length; t++) { var tab = win.tabs[t]; results.push(tab.history()); } } JSON.stringify(results);`
	}
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var rawHistoryBlocks []string
	_ = json.Unmarshal(output, &rawHistoryBlocks)
	buffers := make(map[string]string)
	for i, block := range rawHistoryBlocks {
		buffers[fmt.Sprintf("tab_%d", i)] = block
	}
	return buffers, nil
}

func sanitizeOutput(text string) string {
	cleaned := ansiRegex.ReplaceAllString(text, "")
	cleaned = strings.ReplaceAll(cleaned, "^[[A", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[B", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[C", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[D", "")
	return cleaned
}

func generateUniquePath(filename string) string {
	if filename == "" {
		filename = fmt.Sprintf("termlog_%s.log", time.Now().Format("2006_01_02_15_04_05"))
	}
	absPath, _ := filepath.Abs(filename)
	base := strings.TrimSuffix(absPath, filepath.Ext(absPath))
	ext := filepath.Ext(absPath)
	counter := 1
	finalPath := absPath
	for {
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			break
		}
		finalPath = fmt.Sprintf("%s_%d%s", base, counter, ext)
		counter++
	}
	return finalPath
}

func startDaemonWorker() int {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = os.Args[0]
	}
	cmd := exec.Command(selfPath, "__daemon_worker__")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	err = cmd.Start()
	if err == nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		_ = cmd.Process.Release()
		return pid
	}
	return 0
}

func getStateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, stateFileName)
}

func loadGlobalState(path string) GlobalState {
	var state GlobalState
	state.Sessions = make(map[string]*TerminalSession)
	file, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	_ = json.Unmarshal(file, &state)
	return state
}

func saveGlobalState(path string, state GlobalState) {
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func printUsage() {
	fmt.Println("\nUsage: termlog [below|above|live|quit|offline|online|status] {optional: filename}\n")
}
