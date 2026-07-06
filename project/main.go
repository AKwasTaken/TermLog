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
	currentAnchor := findActiveWindowAnchor(&globalState)

	switch command {
	case "below":
		handleBelow(&globalState, filenameArg)
	case "above":
		handleAbove()
	case "live":
		handleLive(&globalState, filenameArg)
	case "quit":
		handleQuit(&globalState, currentAnchor)
	case "offline":
		handleOffline(&globalState, currentAnchor)
	case "online":
		handleOnline(&globalState, currentAnchor)
	case "status":
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
		buffers, err := getTerminalBufferMap()
		if err != nil {
			continue
		}

		for _, buf := range buffers {
			normalizedBuffer := strings.ReplaceAll(buf, "\r", "\n")

			for token, session := range globalState.Sessions {
				fullAnchorPattern := fmt.Sprintf("# [termlog session anchor: %s]", token)

				if idx := strings.Index(normalizedBuffer, fullAnchorPattern); idx != -1 {
					rawNewContent := normalizedBuffer[idx+len(fullAnchorPattern):]

					if lastNL := strings.LastIndex(rawNewContent, "\n"); lastNL >= 0 {
						committedContent := rawNewContent[:lastNL]

						cleanText := sanitizeOutput(committedContent)
						lines := strings.Split(cleanText, "\n")
						var filteredLines []string

						isCurrentlyWriting := true
						// var lastAddedLine string

						for _, line := range lines {
							trimmedLine := strings.TrimSpace(line)

							// Skip completely empty lines right away to prevent double layout spaces
							if trimmedLine == "" {
								continue
							}

							// Catch the explicit command line inputs, changing states without logging the command itself
							if strings.Contains(line, "termlog offline") {
								if isCurrentlyWriting { // Only add the message if our internal state is actually switching
									isCurrentlyWriting = false
									statusMsg := "\n\n⏸ Logging paused (Offline).\n\n"
									filteredLines = append(filteredLines, statusMsg)
									// lastAddedLine = statusMsg
								}
								continue
							}

							if strings.Contains(line, "termlog online") {
								if !isCurrentlyWriting { // Only add the message if we are genuinely transitioning back to active
									isCurrentlyWriting = true
									statusMsg := "\n\n▶ Logging resumed (Online).\n\n"
									filteredLines = append(filteredLines, statusMsg)
									// lastAddedLine = statusMsg
								}
								continue
							}

							// Strip out any terminal output responses generated natively by the tool executable
							if strings.Contains(line, "Logging paused") || strings.Contains(line, "Logging resumed") || strings.Contains(line, "Logging stopped") {
								continue
							}
							if strings.Contains(line, "termlog:") || strings.Contains(line, "✓ Logging") || strings.Contains(line, "✓ Live") || strings.Contains(line, "✕ Logging") {
								continue
							}
							if strings.Contains(line, "termlog session anchor:") || strings.Contains(line, "##marker_") {
								continue
							}

							// Write normal inputs/outputs cleanly out to disk if the state allows it
							if isCurrentlyWriting {
								filteredLines = append(filteredLines, line)
								// lastAddedLine = line
							}
						}

						var finalPayload []byte
						if session.Mode == ModeLive {
							baseHistory := strings.TrimRight(session.HistoryCache, " \t\n\r") + "\n"
							newLinesText := strings.Join(filteredLines, "\n")
							if len(newLinesText) > 0 {
								finalPayload = []byte(baseHistory + newLinesText + "\n")
							} else {
								finalPayload = []byte(baseHistory)
							}
						} else {
							finalPayload = []byte("--- Log started from below ---\n\n" + strings.Join(filteredLines, "\n") + "\n")
						}

						_ = os.WriteFile(session.OutputFile, finalPayload, 0644)
					}
				}
			}
		}
	}
}
