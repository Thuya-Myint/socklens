# SockLens

Inspect local TCP/UDP ports, see their processes, monitor changes, and terminate a process with confirmation. Available for macOS, Windows, Linux, and WSL2.

## Installation

The current public release is v0.1.0 for Linux/WSL2. Native macOS and Windows installation requires v0.2.0, which is not published yet; those instructions become usable when it is released.

Install a ready-to-run binary. Go, a compiler, and a source checkout are not required.

### Homebrew — macOS and Linux

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
socklens version
```

Homebrew selects the binary for Intel/AMD 64-bit or ARM64, including Apple Silicon.

### Scoop — Windows

With Scoop installed, run in PowerShell:

```powershell
scoop install https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens.json
socklens version
```

The manifest selects the x64 or ARM64 Windows binary.

### Linux — binary installer

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
socklens version
```

The installer detects your CPU, verifies the download checksum, and installs into ~/.local/bin without sudo. Run the commands inside WSL2 if you use this method on Windows.

To choose another installation directory or published version:

```sh
SOCKLENS_INSTALL_DIR="$HOME/bin" SOCKLENS_VERSION=v0.2.0 sh /tmp/socklens-install.sh
```

The installer does not change shell startup files. Add your installation directory to PATH if you want to run socklens from any terminal.

## Commands

| Command | Purpose |
| --- | --- |
| help | Show the command overview |
| version | Show version and build information |
| check | Check whether socket discovery works |
| list | List local TCP/UDP sockets |
| inspect | Inspect a local port |
| watch | Monitor sockets or changes |
| wait | Wait for matching sockets to appear or disappear |
| terminate | Preview or confirm process termination |

### help

```sh
socklens help
socklens --help
socklens -h
```

Running socklens without a command also shows help.

For command-specific options:

```sh
socklens list --help
socklens inspect --help
socklens watch --help
socklens wait --help
socklens terminate --help
```

These five commands also accept -h.

### version

```sh
socklens version
socklens --version
```

Shows the installed version, commit, and build date. Takes no additional arguments.

### check

```sh
socklens check
```

Checks socket discovery and reports the number of observed sockets. Takes no options. Process names and ownership may be unavailable because of permissions.

### Shared discovery options

| Option | Available on | Meaning |
| --- | --- | --- |
| --port PORT | list, watch | Filter by local port, from 1 to 65535 |
| --protocol tcp or --protocol udp | list, inspect, watch, wait | Filter by protocol; the default includes both |
| --pid PID | list, inspect, watch, wait | Filter by an observed process ID |
| --listening | list, inspect, watch, wait | Select TCP listeners and unconnected/bound UDP sockets |
| --json | list, inspect, watch, wait | Produce JSON output |

Filters combine: a socket must match every supplied filter. inspect and wait take their port as a positional argument; do not also pass --port.

Put options before the final port argument:

```sh
socklens inspect --protocol tcp --json 8080
socklens wait --listening --timeout 10s 8080
```

### list

```sh
socklens list
socklens list --listening
socklens list --port 8080 --protocol tcp
socklens list --pid 12345
socklens list --json
```

Lists sockets in all states by default. Text output includes protocol, local/remote endpoints, state, process IDs, and process names. Unavailable owner information appears as a dash.

IPv4 and IPv6 are supported. On Windows, UDP rows are BOUND because connection state is not provided by the system table. On macOS, process attribution is the last recorded PID and may not include every holder of a shared socket. WSL2 shows Linux sockets in that WSL environment.

### inspect

```sh
socklens inspect 8080
socklens inspect --listening --protocol tcp 8080
socklens inspect --pid 12345 --json 8080
```

Shows sockets matching the required local port. Accepts --protocol, --pid, --listening, and --json.

An empty result means no matching socket was observed. Empty JSON output is [].

### watch

```sh
socklens watch
socklens watch --listening --interval 1s
socklens watch --port 8080 --count 5
socklens watch --changes --protocol tcp
socklens watch --changes --json
```

Takes the first snapshot immediately. Stop with Ctrl+C.

| Option | Default | Meaning |
| --- | --- | --- |
| --interval DURATION | 2s | Wait between completed snapshots; minimum 100ms |
| --count N | 0 | Stop after N polls; 0 keeps watching |
| --changes | Off | Emit a baseline, then only additions, removals, and updates |

All shared discovery options are supported.

In normal mode, each refresh prints a timestamp and socket table. With --changes, the first output is the baseline. Later output uses ADDED, REMOVED, and UPDATED BEFORE/AFTER; unchanged polls stay quiet. --count counts polls, including quiet ones.

With --json, each emitted record is one JSON line. Normal records contain timestamp and sockets; change records contain timestamp, baseline, added, removed, and updated. Updates include before/after socket details.

Polling can miss connections that open and close between refreshes.

### wait

```sh
socklens wait --listening --protocol tcp 8080
socklens wait --until absent --listening --timeout 10s 8080
socklens wait --pid 12345 --interval 1s --json 8080
socklens wait --timeout 0 8080
```

Waits quietly until the required local port matches the condition.

| Option | Default | Meaning |
| --- | --- | --- |
| --until present or --until absent | present | Succeed when matching sockets exist, or when none exist |
| --timeout DURATION | 30s | Maximum wait; 0 waits until interrupted |
| --interval DURATION | 500ms | Wait between polls; minimum 100ms |

Accepts --protocol, --pid, --listening, and --json. Options must precede the port.

The initial poll is immediate. Success prints a confirmation and any matching sockets. With --json, success is one object containing timestamp, condition, port, and sockets. Timeout and errors go to standard error.

Without --listening, sockets in other states also count. Socket presence does not establish application health, and absence does not reserve the port. Ctrl+C cancels the wait.

### terminate

Find the process first:

```sh
socklens list --listening
```

Replace 12345 below with an actual PID from the PIDS column.

Preview without signaling or prompting:

```sh
socklens terminate --pid 12345 --dry-run
```

On Linux or macOS:

```sh
socklens terminate --pid 12345
```

On Windows:

```powershell
socklens terminate --pid 12345 --force
```

| Option | Meaning |
| --- | --- |
| --pid PID | Required process ID |
| --dry-run | Display the target without sending a signal or asking for confirmation |
| --force | Windows only: explicitly permit immediate process termination |
| -h, --help | Show termination options |

Actual termination requires typing the displayed PID again and pressing Enter. A mismatched or incomplete confirmation cancels the operation.

Linux/macOS send SIGTERM. Windows termination is immediate and can interrupt unsaved work; --force does not bypass PID confirmation. It is rejected on Linux/macOS.

Protected system PIDs, SockLens itself, and processes owned by another user are refused. Identity checks prevent signaling a replacement process. If safe signaling is unavailable, termination is refused. A successful request does not guarantee that the process has finished exiting.

## JSON output

list and inspect produce a JSON array. Each socket includes Protocol, Local, Remote, State, Inode, PIDs, and Processes. Each process entry includes PID and Name. Inode is a platform-specific identity value and may be empty.

watch emits newline-delimited records; wait emits one result on success. JSON strings escape control characters in process names.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Command succeeded or help was displayed |
| 1 | Invalid input, discovery failure, termination refusal, or output error |
| 2 | wait timed out |
| 130 | wait was interrupted |

Stopping watch with Ctrl+C exits successfully.
