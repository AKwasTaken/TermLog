```text
                                                              .-'''-.              
                                                     .---.   '   _    \            
               __.....__              __  __   ___   |   | /   /` '.   \           
           .-''         '.           |  |/  `.'   `. |   |.   |     \  '  .--./)   
     .|   /     .-''"'-.  `. .-,.--. |   .-.  .-.   '|   ||   '      |  '/.''\\    
   .' |_ /     /________\   \|  .-. ||  |  |  |  |  ||   |\    \     / /| |  | |   
 .'     ||                  || |  | ||  |  |  |  |  ||   | `.   ` ..' /  \`-' /    
'--.  .-'\    .-------------'| |  | ||  |  |  |  |  ||   |    '-...-'`   /("'`     
   |  |   \    '-.____...---.| |  '- |  |  |  |  |  ||   |               \ '---.   
   |  |    `.             .' | |     |__|  |__|  |__||   |                /'""'.\  
   |  '.'    `''-...... -'   | |                     '---'               ||     || 
   |   /                     |_|                                         \'. __//  
   `'-'                                                                   `'---'   
```

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

Download the latest production release archive (`termlog-mac.tar.gz`) from the **Releases** tab and run:

```bash
# Extract the binary
tar -xzf termlog-mac.tar.gz

# Make it executable and route it to your local system binaries
chmod +x termlog
sudo mv termlog /usr/local/bin/

```

### macOS Security Permissions:

1. The first time you run an engine command (like `termlog live`), macOS will prompt for **Automation Permissions** so AppleScript can read the window data. Click **OK**.
2. If the binary is flagged or blocked by Gatekeeper as unsigned, strip the isolation attribute once using the following command:


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