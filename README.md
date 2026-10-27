# SockLens

Inspect local TCP/UDP ports, see their processes, monitor changes, and terminate a process with confirmation. Available for macOS, Windows, Linux, and WSL2.

## Installation

Native macOS and Windows installation requires v0.2.0 or newer.

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
scoop bucket add socklens https://github.com/Thuya-Myint/socklens
scoop install socklens/socklens
socklens version
```

The bucket selects the x64 or ARM64 Windows binary and receives new release manifests automatically.

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
SOCKLENS_INSTALL_DIR="$HOME/bin" SOCKLENS_VERSION=v0.4.0 sh /tmp/socklens-install.sh
```

The installer does not change shell startup files. Add your installation directory to PATH if you want to run socklens from any terminal.

### Upgrade an existing installation

From v0.5.0 onward, upgrade with one command:

```sh
socklens update
```

It checks for a newer stable release and uses Homebrew, Scoop, or a verified
standalone binary download to upgrade the current installation. See [update](#update)
for preview, timeout, and Windows progress options.

**v0.4.0 and older need one initial upgrade using the commands below** to receive
this new command. An already installed binary cannot gain a command until it is upgraded.

With Homebrew:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
```

With Scoop bucket installations:

```powershell
scoop update
scoop update socklens
```

If you installed an older version from a direct manifest URL, migrate once:

```powershell
scoop uninstall socklens
scoop bucket add socklens https://github.com/Thuya-Myint/socklens
scoop install socklens/socklens
```

On Linux/WSL2, rerun the binary installer commands above to replace the installed
binary. Download the installer again so it selects the current release.

Verify the upgrade:

```sh
socklens version
socklens update-check
```

The update-check and updates commands require v0.3.0 or newer.

## Terminal output and colors

Human-readable output uses aligned socket tables, a socket-count summary, clear
empty results, and labeled status messages. Watch output groups additions,
removals, and updates with UTC timestamps. Regular process names are shown
without extra quotes; unusual names are escaped for safe terminal display.

Socket tables adapt to the terminal width on Windows, macOS, and Linux. When a
full table would not fit, each socket is shown as a stacked record with wrapped
values. Endpoints, PIDs, and process names remain complete. Watch checks the
width again for each output, so later snapshots adapt after a resize. Redirected
text keeps the full table; JSON formatting is unchanged.

Colors are automatic in supported Windows, macOS, and Linux terminals:

- Green: success, listening/bound sockets, and added sockets.
- Cyan: headings, information, and connected sockets.
- Yellow: warnings, closing sockets, and updated sockets.
- Red: errors and removed sockets. Labels also describe every status without color.

| Global option | Meaning |
| --- | --- |
| --color auto | Detect color support independently for standard output and standard error; the default |
| --color always | Force ANSI colors, including redirected text |
| --color never | Disable colors |

```sh
socklens list --listening
socklens list --color never
socklens --color always inspect 8080
```

The --color option can appear before the command or among its options. Put it
before PORT for inspect and wait, like other options. Set NO_COLOR to a nonempty
value or TERM=dumb to disable automatic colors. Explicit --color always overrides
these environment settings.

Pipes and files receive plain text by default. JSON output and its diagnostics
never contain colors, including when --color always is supplied. Version output
also remains plain. Text layouts are intended for people; use --json for scripts.

## Commands

| Command | Purpose |
| --- | --- |
| help | Show the command overview |
| version | Show the installed version |
| check | Check whether socket discovery works |
| update-check | Check GitHub for a newer stable release |
| update | Check and install a newer stable release |
| updates | Enable, disable, or view daily update notices |
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
socklens update-check --help
socklens update --help
socklens updates --help
```

These eight commands also accept -h.

### version

```sh
socklens version
socklens --version
```

Shows only the installed version. Takes no additional arguments.

### check

```sh
socklens check
```

Checks socket discovery and reports the number of observed sockets. Takes no options. Process names and ownership may be unavailable because of permissions.

### update-check

```sh
socklens update-check
socklens update-check --json
socklens update-check --timeout 5s
```

Checks the latest published stable GitHub release. Source pushes and unpublished previews do not create a newer installable release. It reports whether an update
is available, the installed version, and the release link. It does not install
updates. Development builds without a comparable release version are skipped
by daily checks; the explicit command reports that the version cannot be compared.

| Option | Default | Meaning |
| --- | --- | --- |
| --json | Off | Return one JSON object with current, latest, update_available, release_url, and status |
| --timeout DURATION | 3s | Network deadline, from 100ms to 30s |
| -h, --help | — | Show update-check options |

Manual checks contact GitHub each time. Offline, timeout, and rate-limit errors
return status 1. An available update still returns status 0; use update_available
in JSON for scripts. No socket or process information is sent.

For GitHub notifications, open the [repository](https://github.com/Thuya-Myint/socklens),
select Watch, then Custom, choose Releases, and save. Each user controls their
own GitHub notification and email preferences.

### update

Requires v0.5.0 or newer; older installations must upgrade once using their installer.

```sh
socklens update
socklens update --dry-run
socklens update --timeout 10m
```

Checks GitHub for the newest stable release and upgrades only when a newer
version is available. A current installation is left as it is.

| Option | Default | Meaning |
| --- | --- | --- |
| --dry-run | Off | Check availability and show the update method without installing |
| --status | Off | Show the latest Windows background update result without contacting GitHub |
| --timeout DURATION | 5m | Total update deadline, from 1s to 30m |
| -h, --help | — | Show update options |

Use --status alone. --dry-run can be combined with --timeout.

- **Homebrew:** refreshes Homebrew and upgrades the formula identified by the
  installation receipt, then checks the installed version.
- **Scoop:** refreshes Scoop and its buckets, upgrades SockLens, and verifies the
  version. Older direct-manifest installations need the one-time bucket migration
  described above. If a release is newer than the bucket, the command asks you to
  retry after the installer has been published.
- **Standalone binaries:** downloads the matching macOS, Windows, or Linux
  amd64/arm64 release over HTTPS, checks SHA-256, architecture, and version, then
  replaces the executable. A failed post-install check attempts to restore the
  previous executable. The install directory must be writable.

On macOS and Linux, wait for the success message. On Windows, the command starts
a background installer so the running executable can exit before replacement.
It prints the log location; check completion with:

```powershell
socklens update --status
socklens version
```

On Windows, an initial successful exit means the installer started; --status
reports whether it completed or failed. Close other SockLens processes before
updating if Windows reports that the executable is in use.

No administrator prompt, sudo, shell-profile change, or automatic installation
is added. Package-manager policies and normal file permissions still apply.
Unknown package-manager locations are refused rather than overwritten. Checksums
come from the same GitHub release as the archive; they detect download corruption
but are not a separate publisher signature.

An interrupted standalone update can leave a reported .update-lock directory.
Before removing that empty lock and retrying, confirm no updater is still running.
If a Windows background installer is stopped unexpectedly, retain its log and
use the original installer to recover the installation.

### updates

Daily CLI notices are off by default.

```sh
socklens updates --enable
socklens updates --status
socklens updates --disable
```

| Option | Meaning |
| --- | --- |
| --enable | Opt into daily background update checks and cached notices |
| --disable | Stop automatic checks and notices |
| --status | Show the current setting; also the default when no option is supplied |
| -h, --help | Show preference options |

Choose only one of --enable, --disable, and --status. Preferences apply to the
current user across terminals.

When enabled, interactive list, inspect, check, watch, or wait runs can schedule
a background check at most once every 24 hours. A cached update notice appears
on a subsequent interactive run, at most once per version per day. The network
request runs separately so it cannot delay the port command; Windows workers
do not open a console window.

Normal commands remain usable if GitHub is offline or the check fails. No
automatic notice or background check runs for JSON output, redirected output,
help, version, or termination. Notices use standard error and never alter socket
JSON. Nothing is installed or replaced automatically.

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
| 1 | Invalid input, discovery/update failure, termination refusal, settings error, or output error |
| 2 | wait timed out |
| 130 | wait or a foreground update was interrupted |

Stopping watch with Ctrl+C exits successfully.
