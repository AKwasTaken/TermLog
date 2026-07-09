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


A lightweight, high-performance terminal session logger built in Go for macOS and Linux (`Terminal.app`, `iTerm2`, and `tmux`). TermLog transparently intercepts, sanitizes, and preserves your command-line workflows directly into individual log files across multiple terminal windows or tabs simultaneously without changing your terminal behavior.

Unlike primitive solutions that modify your prompt or pollute your environment layout, TermLog virtualizes your shell at the system level—leaving you with full character editing, text pasting, and layout stability.

---

## Implemented Features

* **Live Logging (`live`):** Seamlessly captures your entire current window scrollback history and appends future commands/outputs into a single log file in real time.
* **Point-Forward Logging (`below`):** Starts a clean logging session from the exact moment you execute the command (skipping past scrollback history).
* **Instant Snapshot (`above`):** One-shot export that dumps your entire current scrollback history into a file on demand.
* **In-Band Boundary Tracking:** Uses invisible OSC marker sequences passed through the PTY channel to calculate exact command/output boundary shifts—eliminating race conditions and missing bytes.
* **Deterministic Tab Isolation:** Automatically isolates concurrent logging sessions across multiple tabs and windows by mapping control processes to the terminal's unique device node (`/dev/tty`).
* **On-the-Fly Control States (`offline`/`online`):** Temporarily pause and resume logging chronologically within an open shell timeline.

---

## Core Architecture & Engineering

TermLog drops slow system polling models in favor of a low-level, event-driven hybrid design:

<div align="center">
<img src="assets/diagram.svg" alt='Diagram'>
</div>

* **PTY Virtualization Layer:** When running `below` or `live`, TermLog spawns your interactive login shell inside a Pseudo-Terminal wrapper (`github.com/creack/pty`). It proxies raw bytes bidirectionally so interactive sub-apps (`vim`, `nano`, `top`), terminal resizing (`SIGWINCH`), and ANSI colors work natively.
* **In-Band Marker Handshaking:** During shell setup, TermLog injects standard `preexec` and `precmd` tracking hooks into Zsh. These hooks emit invisible OSC boundary sequences (`\x1b]133;C\x07` and `\x1b]133;D\x07`) directly into the single ordered output stream. The parsing engine catches these byte markers to record exactly what you run and see.
* **JSON IPC Protocol over UNIX Sockets:** A localized socket instance (`~/.termlog/sockets/{tty_hash}.sock`) hosts an internal JSON IPC service. Utility commands (`status`, `offline`, `quit`) communicate instantly with your active background PTY manager across isolation boundaries.

---

## Installation & Setup

### Step 1: Install the Binary

#### Option A: Homebrew Tap (Recommended)

```bash
brew tap akwastaken/tap
brew install termlog

```

#### Option B: Compile From Source (Advanced)
Clone the repository workspace and compile the optimized production binary on your native architecture using Go:

```bash
# Clone the repository workspace
git clone [https://github.com/akwastaken/termlog.git](https://github.com/akwastaken/termlog.git)
cd termlog

# Install GO (if you haven't already)
brew install go

# Compile a production binary
go build -ldflags="-s -w" -o termlog main.go

# Install the binary into your system execution path
chmod +x termlog
sudo mv termlog /usr/local/bin/

```

---

### Step 2: One-Time Shell Integration

To allow TermLog to pass tracking markers through the PTY stream cleanly, install the Zsh hooks into your profile:

```bash
# Append the integration block to your config profile automatically
termlog install

# Source your profile to activate the hooks in your current tab
source ~/.zshrc

```

---

## Command Reference

| Command | Usage | Description |
| --- | --- | --- |
| **Log From Below** | `termlog below {optional: file}` | Starts recording everything *after* this prompt. |
| **Log Snapshot** | `termlog above {optional: file}` | Captures a static export of your *past* history up to this point. |
| **Live Tracking** | `termlog live {optional: file}` | Combines both: grabs past history and tracks all future inputs live. |
| **Pause Logging** | `termlog offline` | Temporarily halts logging. Your active sub-shell remains open. |
| **Resume Logging** | `termlog online` | Resumes live tracking appends cleanly. |
| **Session Status** | `termlog status` | Displays active mode, state (`online`/`offline`), target file, and tracking start time. |
| **Stop Engine** | `termlog quit` | Terminates the background PTY loop and resumes the root shell context. |

---

## Troubleshooting & Constraints

### 1. macOS Automation Requirements

Because the snapshot capture commands (`above` and `live`) read historical scrollback bounds via AppleScript/JXA, macOS requires a system-level authorization step. The first time you execute a snapshot command, macOS will prompt you with a dialog requesting **Automation Permissions**. You must click **OK** to authorize history gathering.

### 2. AppleScript Quirks inside iTerm2

Due to native configuration boundaries inside iTerm2's automation surface, the `.contents` snapshot block will only scrape text fitting inside your current *visible view-port canvas bounding box* at that exact millisecond. Complete, deep-buffer historical snapshot recovery requires iTerm2's Python API, which will be evaluated in a later release.

### 3. Deleting Log Files Mid-Session

If you delete your target `.log` file mid-session while a logging process is active, TermLog's appending write flags (`os.O_CREATE|os.O_APPEND`) will gracefully catch the event on the next command return, recreate a fresh log file automatically, and keep recording without throwing an error.
