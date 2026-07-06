package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const stateFileName = ".termlog_state.json"

type LogMode string

const (
	ModeBelow LogMode = "below"
	ModeAbove LogMode = "above"
	ModeLive  LogMode = "live"
	ModeOff   LogMode = "off"
)

type AppState struct {
	Mode         LogMode `json:"mode"`
	IsOnline     bool    `json:"is_online"`
	OutputFile   string  `json:"output_file"`
	PID          int     `json:"pid"`
	TerminalType string  `json:"terminal_type"`
}

func main() {
	// Internal hidden command used to spawn the background daemon loop
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
	state := loadState(stateFile)

	switch command {
	case "below":
		handleBelow(&state, filenameArg, stateFile)
	case "above":
		handleAbove(&state, filenameArg)
	case "live":
		handleLive(&state, filenameArg, stateFile)
	case "quit":
		handleQuit(&state)
	case "offline":
		handleOffline(&state)
	case "online":
		handleOnline(&state)
	case "status":
		handleStatus(&state)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		return
	}

	saveState(stateFile, state)
}

func getTerminalBuffer(termApp string) (string, error) {
	var script string

	switch termApp {
	case "iTerm.app":
		script = `tell application "iTerm2" to tell current session of current window to get text`
	case "Apple_Terminal":
		script = `
			tell application "Terminal"
				tell front window
					tell selected tab
						get history
					end tell
				end tell
			end tell
		`
	default:
		return "", fmt.Errorf("neither (Detected type: '%s')", termApp)
	}

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("AppleScript failed: %v", err)
	}

	return string(output), nil
}

func sanitizeOutput(text string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleaned := ansiRegex.ReplaceAllString(text, "")

	cleaned = strings.ReplaceAll(cleaned, "^[[A", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[B", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[C", "")
	cleaned = strings.ReplaceAll(cleaned, "^[[D", "")

	return cleaned
}

// Generates an auto-incrementing file or timestamped file to avoid overwriting
func generateUniquePath(filename string) string {
	if filename == "" {
		// Formats exactly to: termlog_YYYY_MM_DD_HH_MM_SS.log
		filename = fmt.Sprintf("termlog_%s.log", time.Now().Format("2006_01_02_15_04_05"))
	}

	absPath, _ := filepath.Abs(filename)
	base := strings.TrimSuffix(absPath, filepath.Ext(absPath))
	ext := filepath.Ext(absPath)

	// If the file explicitly exists already, append an incrementor to safeguard it
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

func stopExistingDaemon(state *AppState) {
	if state.PID > 0 {
		proc, err := os.FindProcess(state.PID)
		if err == nil {
			_ = proc.Signal(os.Kill) // Kill the old running collector loop safely
		}
		state.PID = 0
	}
}

func startDaemonWorker(state *AppState, stateFilePath string) {
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = os.Args[0]
	}

	// Direct execution without nohup wrapper to capture the direct OS response
	cmd := exec.Command(selfPath, "__daemon_worker__")
	
	// Create a debug log file on your desktop
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, "Desktop", "termlog_error.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		// Disconnect standard input so it can run backgrounded
		cmd.Stdin = nil
	}

	err = cmd.Start()
	if err == nil && cmd.Process != nil {
		state.PID = cmd.Process.Pid
		_ = cmd.Process.Release()
	} else {
		fmt.Println("Error initializing background logger engine:", err)
	}
	
	if logFile != nil {
		// Close the descriptor in the parent process safely
		_ = logFile.Close()
	}
}

func handleAbove(state *AppState, filename string) {
	targetFile := generateUniquePath(filename)

	buffer, err := getTerminalBuffer(os.Getenv("TERM_PROGRAM"))
	if err != nil {
		if strings.Contains(err.Error(), "neither") {
			fmt.Println("This command is only supported in native macOS Terminal or iTerm2.")
			return
		}
		fmt.Println("Error:", err)
		return
	}

	cleanBuffer := sanitizeOutput(buffer)
	err = os.WriteFile(targetFile, []byte(cleanBuffer), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return
	}

	state.Mode = ModeAbove
	state.OutputFile = targetFile
	fmt.Printf("Logged history up to this point into: %s\n", targetFile)
}

func handleBelow(state *AppState, filename string, stateFilePath string) {
	stopExistingDaemon(state)
	targetFile := generateUniquePath(filename)

	state.Mode = ModeBelow
	state.IsOnline = true
	state.OutputFile = targetFile
	state.TerminalType = os.Getenv("TERM_PROGRAM")

	// 1. Write the clean text file header
	_ = os.WriteFile(targetFile, []byte("--- Log started from below ---\n"), 0644)

	// 2. Generate an absolutely unique token string
	milestoneToken := fmt.Sprintf("##TERMLOG_MARKER_%d##", time.Now().UnixNano())
	
	// 3. Silently drop the marker line into the user's terminal window history stream
	fmt.Printf("\033[A\r") // Move cursor up and clear line to make it invisible to the user
	fmt.Printf("# %s\n", milestoneToken)

	// Save the active marker token to a cache file so the background daemon knows what to look for
	home, _ := os.UserHomeDir()
	_ = os.WriteFile(filepath.Join(home, ".termlog_baseline.tmp"), []byte(milestoneToken), 0644)

	fmt.Printf("Logging everything from this point forward to: %s\n", targetFile)
	startDaemonWorker(state, stateFilePath)
}

func handleLive(state *AppState, filename string, stateFilePath string) {
	stopExistingDaemon(state)
	targetFile := generateUniquePath(filename)

	state.TerminalType = os.Getenv("TERM_PROGRAM")
	
	// Dump historical text first
	buffer, _ := getTerminalBuffer(state.TerminalType)
	_ = os.WriteFile(targetFile, []byte(sanitizeOutput(buffer)), 0644)

	state.Mode = ModeLive
	state.IsOnline = true
	state.OutputFile = targetFile

	// Inject milestone token
	milestoneToken := fmt.Sprintf("##TERMLOG_MARKER_%d##", time.Now().UnixNano())
	fmt.Printf("\033[A\r# %s\n", milestoneToken)

	home, _ := os.UserHomeDir()
	_ = os.WriteFile(filepath.Join(home, ".termlog_baseline.tmp"), []byte(milestoneToken), 0644)

	fmt.Printf("Live tracking activated into: %s\n", targetFile)
	startDaemonWorker(state, stateFilePath)
}

func runDaemonWorker() {
	home, _ := os.UserHomeDir()
	stateFile := filepath.Join(home, stateFileName)
	baselineFile := filepath.Join(home, ".termlog_baseline.tmp")

	time.Sleep(200 * time.Millisecond)

	state := loadState(stateFile)
	termType := state.TerminalType
	if termType == "" {
		termType = "Apple_Terminal"
	}

	// Load the single, fixed anchor token
	tokenBytes, _ := os.ReadFile(baselineFile)
	anchorMarker := string(tokenBytes)

	for {
		time.Sleep(1 * time.Second)

		state = loadState(stateFile)
		if state.Mode == ModeOff || state.OutputFile == "" {
			break
		}

		if !state.IsOnline {
			continue
		}

		currentBuffer, err := getTerminalBuffer(termType)
		if err != nil {
			continue
		}

		normalizedBuffer := strings.ReplaceAll(currentBuffer, "\r", "\n")

		// Look for our original start anchor
		if idx := strings.Index(normalizedBuffer, anchorMarker); idx != -1 {
			// Extract absolutely everything that has happened since the session started
			rawNewContent := normalizedBuffer[idx+len(anchorMarker):]

			// Only process if the user has hit enter on something new
			if lastNL := strings.LastIndex(rawNewContent, "\n"); lastNL > 0 {
				committedContent := rawNewContent[:lastNL+1]
				cleanText := sanitizeOutput(committedContent)

				// Overwrite the file with the fresh, complete timeline update
				// (Keeps the header intact at the top)
				finalPayload := "--- Log started from below ---\n" + cleanText

				_ = os.WriteFile(state.OutputFile, []byte(finalPayload), 0644)
			}
		}
	}
}

func handleQuit(state *AppState) {
	stopExistingDaemon(state)
	state.Mode = ModeOff
	state.OutputFile = ""
	fmt.Println("Logging stopped completely.")
}

func handleOffline(state *AppState) {
	state.IsOnline = false
	fmt.Println("Logging paused temporarily (Offline).")
}

func handleOnline(state *AppState) {
	if state.Mode == ModeOff || state.OutputFile == "" {
		fmt.Println("No active logging session found.")
		return
	}
	state.IsOnline = true
	fmt.Println("Logging resumed (Online).")
}

func handleStatus(state *AppState) {
	statusStr := "offline"
	if state.IsOnline && state.Mode != ModeOff {
		statusStr = "online"
	}
	if state.Mode == ModeOff || state.OutputFile == "" {
		fmt.Printf("Status: %s\nNo active log file configured.\n", statusStr)
		return
	}
	fmt.Printf("Status: %s\nFile: %s\nPath: %s\n", statusStr, filepath.Base(state.OutputFile), state.OutputFile)
}

func getStateFilePath() string { home, _ := os.UserHomeDir(); return filepath.Join(home, stateFileName) }
func loadState(path string) AppState {
	var state AppState
	file, err := os.ReadFile(path)
	if err != nil {
		return AppState{Mode: ModeOff, IsOnline: false, OutputFile: ""}
	}
	_ = json.Unmarshal(file, &state)
	return state
}
func saveState(path string, state AppState) {
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}
func printUsage() {
	fmt.Println("Usage: termlog [below|above|live|quit|offline|online|status] {optional: filename}")
}