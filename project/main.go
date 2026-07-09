package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__daemon_worker__" {
		runDaemonWorker()
		return
	}

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	var filenameArg string
	if len(os.Args) > 2 {
		filenameArg = os.Args[2]
	}

	stateFile := getStateFilePath()
	globalState := loadGlobalState(stateFile)

	termApp := os.Getenv("TERM_PROGRAM")
	isNativeSupported := (termApp == "Apple_Terminal" || termApp == "iTerm.app")

	currentAnchor := findActiveWindowAnchor(&globalState)

	switch command {
	case "above":
		if !isNativeSupported {
			printUnsupportedError("above")
			return
		}
		handleAbove()
	case "below":
		if !isNativeSupported {
			printUnsupportedError("below")
			return
		}
		handleBelow(&globalState, filenameArg)
	case "live":
		if !isNativeSupported {
			printUnsupportedError("live")
			return
		}
		handleLive(&globalState, filenameArg)
	case "quit":
		if !isNativeSupported {
			printUnsupportedError("quit")
			return
		}
		handleQuit(&globalState, currentAnchor)
	case "offline":
		if !isNativeSupported {
			printUnsupportedError("offline")
			return
		}
		handleOffline(&globalState, currentAnchor)
	case "online":
		if !isNativeSupported {
			printUnsupportedError("online")
			return
		}
		handleOnline(&globalState, currentAnchor)
	case "status":
		if !isNativeSupported {
			printUnsupportedError("status")
			return
		}
		handleStatus(&globalState, currentAnchor)
	default:
		fmt.Printf("termlog: unknown command '%s'\n", command)
		printUsage()
		return
	}

	saveGlobalState(stateFile, globalState)
}

func runDaemonWorker() {
	home, _ := os.UserHomeDir()
	stateFile := filepath.Join(home, stateFileName)

	for {
		time.Sleep(1 * time.Second)

		globalState := loadGlobalState(stateFile)
		if len(globalState.Sessions) == 0 {
			continue
		}

		// Get the lightweight line counts map
		currentCounts, err := getTerminalLineCounts()
		if err != nil {
			continue
		}

		stateChanged := false

		for tabID, currentLineCount := range currentCounts {
			// Find which session maps to this active buffer scope via its anchor marker
			deltaText, err := getTerminalDeltaText(tabID, 0, currentLineCount)
			if err != nil {
				continue
			}

			for token, session := range globalState.Sessions {
				fullAnchorPattern := fmt.Sprintf("# [termlog session anchor: %s]", token)

				if strings.Contains(deltaText, fullAnchorPattern) {
					// Check if we have initialized the line coordinate bounds marker for this window yet
					if session.LastLineCount == 0 {
						session.LastLineCount = currentLineCount
						stateChanged = true
						continue
					}

					// If no new lines were typed, skip out immediately!
					if currentLineCount <= session.LastLineCount {
						continue
					}

					// Fetch ONLY the newly added lines between the previous index checkpoint and now
					newRawText, err := getTerminalDeltaText(tabID, session.LastLineCount, currentLineCount)
					if err != nil || strings.TrimSpace(newRawText) == "" {
						continue
					}

					cleanText := sanitizeOutput(newRawText)
					lines := strings.Split(cleanText, "\n")
					var filteredLines []string

					for _, line := range lines {
						trimmedLine := strings.TrimSpace(line)
						if trimmedLine == "" || strings.Contains(line, "termlog ") || strings.Contains(line, "##marker_") {
							continue
						}
						filteredLines = append(filteredLines, line)
					}

					if len(filteredLines) > 0 {
						// Open file with pure APPEND optimization flags. If deleted, it recreates automatically!
						f, err := os.OpenFile(session.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if err == nil {
							_, _ = f.WriteString(strings.Join(filteredLines, "\n") + "\n")
							_ = f.Close()
						}
					}

					// Slide the index window checkpoint up to the current position
					session.LastLineCount = currentLineCount
					session.LastUpdated = time.Now()
					stateChanged = true
				}
			}
		}

		if stateChanged {
			saveGlobalState(stateFile, globalState)
		}
	}
}

func printUnsupportedError(cmd string) {
	termApp := os.Getenv("TERM_PROGRAM")
	if termApp == "" {
		termApp = "Unknown Third-Party Terminal"
	}
	fmt.Printf("\n ✕ Error: 'termlog %s' is unavailable inside %s.\n", cmd, termApp)
	fmt.Println("   This function requires native macOS window event access (Terminal.app or iTerm2).\n")
}
