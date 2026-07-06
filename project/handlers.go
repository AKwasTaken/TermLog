package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func handleAbove() {
	targetFile := generateUniquePath("")
	buffers, err := getTerminalBufferMap()
	if err != nil {
		fmt.Printf("\n↳ Error parsing histories: %v\n\n", err)
		return
	}

	var completeBuffer string
	for _, buf := range buffers {
		completeBuffer += buf + "\n"
	}

	cleanBuffer := sanitizeOutput(completeBuffer)
	_ = os.WriteFile(targetFile, []byte(strings.ReplaceAll(cleanBuffer, "\r", "\n")), 0644)
	fmt.Printf("\n✓ History logged up to this point: %s\n\n", targetFile)
}

func handleBelow(state *GlobalState, filename string) {
	targetFile := generateUniquePath(filename)
	milestoneToken := fmt.Sprintf("##marker_%d##", time.Now().UnixNano())

	fmt.Printf("# [termlog session anchor: %s]\n", milestoneToken)
	fmt.Printf("\n✓ Logging activated from this point forward: %s\n\n", targetFile)

	_ = os.WriteFile(targetFile, []byte("--- Log started from below ---\n\n"), 0644)

	pid := startDaemonWorker()

	state.Sessions[milestoneToken] = &TerminalSession{
		Mode:         ModeBelow,
		IsOnline:     true,
		OutputFile:   targetFile,
		PID:          pid,
		TerminalType: os.Getenv("TERM_PROGRAM"),
		AnchorMarker: milestoneToken,
		LastUpdated:  time.Now(),
	}
}

func handleLive(state *GlobalState, filename string) {
	targetFile := generateUniquePath(filename)
	milestoneToken := fmt.Sprintf("##marker_%d##", time.Now().UnixNano())

	termApp := os.Getenv("TERM_PROGRAM")
	if termApp == "" {
		termApp = "Apple_Terminal"
	}

	script := `tell application "Terminal" to tell front window to tell selected tab to get history`
	cmd := exec.Command("osascript", "-e", script)
	output, _ := cmd.Output()

	normalizedOldBuffer := strings.ReplaceAll(string(output), "\r", "\n")
	cleanOldBuffer := sanitizeOutput(normalizedOldBuffer)
	cleanOldBuffer = strings.TrimRight(cleanOldBuffer, " \t\n\r") + "\n"

	_ = os.WriteFile(targetFile, []byte(cleanOldBuffer), 0644)

	fmt.Printf("# [termlog session anchor: %s]\n", milestoneToken)
	fmt.Printf("\n✓ Live tracking activated: %s\n\n", targetFile)

	pid := startDaemonWorker()

	state.Sessions[milestoneToken] = &TerminalSession{
		Mode:         ModeLive,
		IsOnline:     true,
		OutputFile:   targetFile,
		PID:          pid,
		TerminalType: termApp,
		AnchorMarker: milestoneToken,
		HistoryCache: cleanOldBuffer,
		LastUpdated:  time.Now(),
	}
}

func handleQuit(state *GlobalState, sessionKey string) {
	if sessionKey == "" {
		fmt.Println("\n⚠ No active logging context found in this window session.\n")
		return
	}
	if session, ok := state.Sessions[sessionKey]; ok {
		if session.PID > 0 {
			proc, err := os.FindProcess(session.PID)
			if err == nil {
				_ = proc.Signal(os.Kill)
			}
		}
		delete(state.Sessions, sessionKey)
		fmt.Println("\n✕ Logging stopped completely for this session.\n")
	}
}

func handleOffline(state *GlobalState, sessionKey string) {
	if sessionKey == "" {
		fmt.Println("\n⚠ Active context missing.\n")
		return
	}
	if session, ok := state.Sessions[sessionKey]; ok {
		session.IsOnline = false
		fmt.Println("\n⏸ Logging paused (Offline).\n")
	}
}

func handleOnline(state *GlobalState, sessionKey string) {
	if sessionKey == "" {
		fmt.Println("\n⚠ Active context missing.\n")
		return
	}
	if session, ok := state.Sessions[sessionKey]; ok {
		session.IsOnline = true
		fmt.Println("\n▶ Logging resumed (Online).\n")
	}
}

func handleStatus(state *GlobalState, sessionKey string) {
	if sessionKey == "" {
		fmt.Println("\nStatus: Unmonitored\nNo active log tracking linked to this session space.\n")
		return
	}
	if session, ok := state.Sessions[sessionKey]; ok {
		statusStr := "offline"
		if session.IsOnline {
			statusStr = "online"
		}
		fmt.Printf("\nStatus: %s\nMode: %s\nFile: %s\nPath: %s\n\n", statusStr, session.Mode, filepath.Base(session.OutputFile), session.OutputFile)
	}
}
