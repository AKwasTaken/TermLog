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

func getTerminalBufferMap() (map[string]string, error) {
	script := `
		var results = [];
		var term = Application("Terminal");
		for (var w = 0; w < term.windows.length; w++) {
			var win = term.windows[w];
			for (var t = 0; t < win.tabs.length; t++) {
				var tab = win.tabs[t];
				results.push(tab.history());
			}
		}
		JSON.stringify(results);
	`
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		fallbackScript := `tell application "Terminal" to tell front window to tell selected tab to get history`
		fallbackCmd := exec.Command("osascript", "-e", fallbackScript)
		fallbackOut, fallbackErr := fallbackCmd.Output()
		if fallbackErr == nil {
			return map[string]string{"front": string(fallbackOut)}, nil
		}
		return nil, fmt.Errorf("AppleScript failed: %v", err)
	}

	var rawHistoryBlocks []string
	_ = json.Unmarshal(output, &rawHistoryBlocks)

	buffers := make(map[string]string)
	for i, block := range rawHistoryBlocks {
		buffers[fmt.Sprintf("tab_%d", i)] = block
	}
	return buffers, nil
}

func findActiveWindowAnchor(state *GlobalState) string {
	script := `tell application "Terminal" to tell front window to tell selected tab to get history`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	buffer := strings.ReplaceAll(string(output), "\r", "\n")
	for anchor := range state.Sessions {
		if strings.Contains(buffer, anchor) {
			return anchor
		}
	}
	return ""
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
	if state.Sessions == nil {
		state.Sessions = make(map[string]*TerminalSession)
	}
	return state
}

func saveGlobalState(path string, state GlobalState) {
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func printUsage() {
	fmt.Println("\nUsage: termlog [below|above|live|quit|offline|online|status] {optional: filename}\n")
}
