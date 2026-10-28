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
<img src='https://img.shields.io/badge/Status-Deprecated-red'></img>
<img src='https://img.shields.io/badge/License-MIT-yellow'></img>
</div><br>

> **This project is deprecated and no longer maintained.** The repository is archived: it remains readable, but no issues or pull requests are accepted.
>
> The code works for the features listed below, but I stopped developing it because it grew beyond what I had time to maintain. If you need something similar and actively maintained, search for terminal session recorders such as `asciinema` or `script`.
>
> If you installed TermLog, see [Uninstallation](#uninstallation) to remove the shell hooks and binary.

---

## What it did

* **Live logging (`live`):** captured the current window's scrollback and appended future commands and output to one log file.
* **Point-forward logging (`below`):** started a clean session from the moment the command ran.
* **Snapshot (`above`):** exported the current scrollback to a file on demand.
* **In-band boundary tracking:** used invisible OSC markers in the PTY stream to find command and output boundaries.
* **Per-tab isolation:** kept concurrent sessions across tabs and windows separate, keyed on the terminal's device node.
* **Pause and resume (`offline`/`online`):** paused and resumed logging within an open shell.

## How it worked

TermLog ran your login shell inside a pseudo-terminal wrapper (`github.com/creack/pty`) and proxied bytes in both directions, so interactive programs, terminal resizing, and colors kept working. It added `preexec` and `precmd` hooks to Zsh that emitted OSC 133 markers. A local UNIX socket at `~/.termlog/sockets/` carried JSON commands such as `status`, `offline`, and `quit`.

## Known limitations

* Snapshot capture (`above` and `live`) read the terminal through AppleScript/JXA, so macOS asked for Automation permission on first use.
* In iTerm2, snapshots only captured the text visible in the current viewport. Full scrollback needed iTerm2's Python API, which was never implemented.


---


## Installation & Setup

### Step 1: Install the Binary

#### Option A: Homebrew Tap (Recommended)

```bash
brew tap AKwasTaken/tap
brew install termlog

```

#### Option B: Compile From Source (Advanced)
Clone the repository workspace and compile the optimized production binary on your native architecture using Go:

```bash
# Clone the repository workspace
git clone https://github.com/akwastaken/termlog.git
cd termlog

# Install GO (if you haven't already)
brew install go

# Compile a production binary
go build -ldflags="-s -w" -o termlog main.go

# Install the binary into your system execution path
chmod +x termlog
sudo mv termlog /usr/local/bin/
```


### Step 2: One-Time Shell Integration

To allow TermLog to pass tracking markers through the PTY stream cleanly, install the Zsh hooks into your profile:

```bash
# Append the integration block to your config profile automatically
termlog install

# Source your profile to activate the hooks in your current tab
source ~/.zshrc
```

---

## Uninstallation

### 1. Quit the program

```bash
termlog quit

# Force-quit any remaining instances
killall termlog 2>/dev/null || true
```

### 2. Remove the binary

Homebrew:

```bash
brew uninstall termlog
brew untap AKwasTaken/tap
```

Compiled from source:

```bash
sudo rm -f /usr/local/bin/termlog
```

### 3. Remove the leftover files

```bash
rm -rf ~/.termlog
```

### 4. Remove the Zsh hooks

Open `~/.zshrc` and delete this block, which is at the bottom of the file:

```bash
# >>> termlog integration >>>
[ -f "~/.termlog/termlog.zsh" ] && source "~/.termlog/termlog.zsh"
# <<< termlog integration <<<
```

### 5. Reload your shell

```bash
source ~/.zshrc
```

---

## Command reference (historical)

| Command | Usage | Description |
| --- | --- | --- |
| Log from below | `termlog below [file]` | Records everything after this prompt. |
| Snapshot | `termlog above [file]` | Exports past history up to this point. |
| Live tracking | `termlog live [file]` | Exports past history and tracks future input. |
| Pause | `termlog offline` | Pauses logging. The shell stays open. |
| Resume | `termlog online` | Resumes logging. |
| Status | `termlog status` | Shows mode, state, target file, and start time. |
| Stop | `termlog quit` | Stops the background loop. |

## License

MIT
