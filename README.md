<div align="center">
<pre style="background: transparent; border: none; padding: 0; margin: 0;">
████████╗███████╗██████╗ ███╗   ███╗██╗      ██████╗  ██████╗ 
╚══██╔══╝██╔════╝██╔══██╗████╗ ████║██║     ██╔═══██╗██╔════╝ 
   ██║   █████╗  ██████╔╝██╔████╔██║██║     ██║   ██║██║  ███╗
   ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║██║     ██║   ██║██║   ██║
   ██║   ███████╗██║  ██║██║ ╚═╝ ██║███████╗╚██████╔╝╚██████╔╝
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚═════╝ 
                                                              

</pre>
</div>

<div align="center">
<a href="https://www.codefactor.io/repository/github/akwastaken/termlog/overview/main"><img src="https://www.codefactor.io/repository/github/akwastaken/termlog/badge/main" alt="CodeFactor" /></a>
</div><br>



A lightweight, zero-dependency session logger built specifically for macOS (`Terminal.app` and `iTerm2`). TermLog lets you capture, sanitize, and preserve your terminal workflows cleanly into individual log files across multiple terminal windows or tabs simultaneously.

---

## Implemented Features

* **Live Logging (`live`):** Captures your entire current window history and seamlessly streams future commands into a file.
* **Point-Forward Logging (`below`):** Starts a clean logging session from the exact moment you run the command.
* **Instant Snapshot (`above`):** Dumps your entire current scrollback history into a file on demand.
* **Zero UI Noise:** Advanced regex layers strip out raw ANSI color escape sequences, arrow key artifacts, and terminal spatial trailing gaps.
* **Seamless Multi-Terminal Mapping:** Automatically maps concurrent logs to separate files across multiple tabs using dynamic, screen-anchored session trackers. No manual session environment variables required.
* **On-the-Fly Control States (`offline`/`online`):** Temporarily pause and resume logging chronologically inside your terminal timeline.

---

## Core Architecture & Performance Optimizations

* **High-Performance Go Engine:** Written entirely in GO and aggressively optimized for minimal system impact.
* **Memory-Leak Free Design:** While the daemon takes an interface snapshot every second, it bypasses internal heap allocation traps by pre-compiling matching expressions globally and enforcing synchronous file descriptor recycling.
* **Centralized JSON State Machine:** A single state database file (`~/.termlog_state.json`) acts as a localized shared registry. Active background processes poll this configuration concurrently to cross-reference terminal session anchor keys against active tracking profiles.

---

## Security & Safe-Tracking Guardrails

* **Zero Password Exposure:** TermLog scrapes visible screen text history buffers. Secure input fields (such as `sudo` or SSH password entries) do not echo characters to the terminal UI canvas, meaning sensitive password strings never hit your log files.
* **Interactive App Boundary:** Fullscreen alternate screen applications (like `nano`, `vim`, or `top`) do not feed into standard scrollback updates during streaming sessions. Standard sequential tracking automatically resumes the moment you exit back to the main shell workspace.

---

## Current Limitations

* **App Boundaries:** Relies on AppleScript UI-Automation hooks. This means it is natively bound to macOS `Terminal.app` and `iTerm2.app`.
* **VS Code / Integrated Panels:** Does not natively capture text inside Electron-based built-in terminals (like VS Code or Cursor). For integrated workflows, external workspace splits using `iTerm2` or native terminal tabs are required.
* **Polling Rate:** Operates on a tight 1-second asynchronous scraping interval.

---

## Installation & Setup

Choose one of the three deployment methods below to install TermLog on your system.

### Method 1: Homebrew Tap (Recommended)

TermLog can be installed via a custom Homebrew Tap. Because it is a self-published formula, Homebrew requires you to explicitly grant a trust permission to the tap before installation:

```bash
# Add the custom repository tap
brew tap AKwasTaken/tap

# Grant explicit trust to the tap to bypass untrusted source errors
brew trust akwastaken/tap

# Install TermLog globally
brew install termlog

```

### Method 2: Pre-Compiled Binary (Tarball)

If you prefer to use the production release assets directly, download the latest release archive (`termlog-{version}.tar.gz`) from the Releases tab and run:

```bash
# Extract the production binary asset
tar -xzf termlog-{version}.tar.gz
# Extract the production binary asset
tar -xzf termlog-{version}.tar.gz

# Make it executable and route it to your local system binaries
chmod +x termlog
sudo mv termlog /usr/local/bin/
```

### Method 3: Compile From Source (Manual Build)

If you wish to audit the codebase or optimize the executable compilation for your specific machine architecture, you can clone and build the binary manually using Go:

```bash
# Clone the repository workspace
git clone https://github.com/AKwasTaken/termlog.git
cd termlog

# Strip development debug symbols and compile a highly optimized production binary
go build -ldflags="-s -w" -o termlog *.go

# Install the binary into your system path execution layers
chmod +x termlog
sudo mv termlog /usr/local/bin/
```

---

### Method 3: Compile From Source (Manual Build)

If you wish to audit the codebase or optimize the executable compilation for your specific machine architecture, you can clone and build the binary manually using Go:

```bash
# Clone the repository workspace
git clone https://github.com/AKwasTaken/TermLog.git
cd termlog

# Install GO using homebrew
brew install go

# Strip development debug symbols and compile a production binary
go build -ldflags="-s -w" -o termlog project/*.go

# Install the binary into your system path execution layers
chmod +x termlog
sudo mv termlog /usr/local/bin/

```

---

### MacOS Security Requirements

Because TermLog utilizes underlying system scraping APIs to log separate tab buffers seamlessly, macOS requires two specific user-side authorizations during its initial execution:

1. **Automation Permissions:** The first time you execute an operational command (such as `termlog live` or `termlog below`), macOS will prompt you with a system modal requesting **Automation Permissions** so AppleScript can read window layout text arrays. You must click **OK** to authorize tracking.
2. **Gatekeeper Quarantine Override:** If you install TermLog via the pre-compiled Tarball or manual Go compilation rather than Homebrew, macOS Gatekeeper may flag the binary as unsigned. Strip the isolation attributes once to allow it to run:
1. **Automation Permissions:** The first time you execute an operational command (such as `termlog live` or `termlog below`), macOS will prompt you with a system modal requesting **Automation Permissions** so AppleScript can read window layout text arrays. You must click **OK** to authorize tracking.
2. **Gatekeeper Quarantine Override:** If you install TermLog via the pre-compiled Tarball or manual Go compilation rather than Homebrew, macOS Gatekeeper may flag the binary as unsigned. Strip the isolation attributes once to allow it to run:

```bash
sudo xattr -dr com.apple.quarantine /usr/local/bin/termlog
```

---

## Command Reference

| Command | Usage | Description |
| --- | --- | --- |
| **Log From Below** | `termlog below {filename}` | Starts recording everything *after* this prompt. |
| **Log Snapshot** | `termlog above {filename}` | Captures a static export of your *past* history up to this point. |
| **Live Tracking** | `termlog live {filename}` | Combines both: grabs past history and tracks all future inputs live. |
| **Pause Logging** | `termlog offline` | Temporarily halts logging. Your past logs stay perfectly intact. |
| **Resume Logging** | `termlog online` | Resumes live tracking cleanly with a formatted timeline indicator. |
| **Session Status** | `termlog status` | Displays current mode, active state (`online`/`offline`), and absolute file path. |
| **Stop Engine** | `termlog quit` | Shuts down the background session logging process for this tab. |

---

## Future Roadmap

The current implementation uses a localized macOS automation layout, but the architectural master plan is to migrate TermLog into a low-level cross-platform core application. Future upgrades include:

### Core Engineering Rewrite

* Shift from AppleScript buffer-scraping to a low-level native **Pseudo-Terminal (PTY) wrapper system**. This will make TermLog completely cross-platform (Linux/macOS) and allow it to run flawlessly inside **VS Code integrated terminals** and remote SSH loops.
* **Shell Auto-Start:** Optional automatic daemon initialization for every new terminal shell session via `.zshrc`/`.bashrc` integration. Never lose a terminal history log because you forgot to turn it on manually.

### Real-Time Log Processing

* **Real-time Markdown Conversion:** Automatically format terminal outputs to clean markdown structural code fences.
* **Sensitive Data Redaction:** Automatically scan inputs/outputs for passwords, keys, and tokens, replacing them with customizable `[REDACTED]` markers (`termlog redaction --status online --msg "SECRET"`).
* **Smart Content Deletion (`termlog rm`):** Drop the last executed command block or target specific noise strings (like `pip install --upgrade`) from the active log history completely.
* **Global Blacklist Filters:** Command formatting options to completely skip recording administrative or background noise commands (`termlog blacklist {cmd}`).
