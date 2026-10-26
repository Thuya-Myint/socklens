# SockLens

Local port discovery, monitoring, and confirmed process termination for Linux/WSL2.

MIT licensed. See CHANGELOG.md for release history and SECURITY.md for reporting.

A Go standard-library-only CLI for inspecting TCP/UDP ports on Linux and WSL2.

## Install (no Go or source checkout required)

The published release contains ready-to-run executables. Choose one method:

### Homebrew on Linux / WSL2

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
socklens version
```

This tap installs the prebuilt release binary. It does not install Go or compile
SockLens. The explicit tap URL uses this repository; no separate tap repository
is required. Update with brew update and brew upgrade Thuya-Myint/socklens/socklens.

### Binary installer on Linux / WSL2

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
socklens version
```

The installer detects amd64/arm64, downloads v0.1.0 from GitHub Releases,
verifies SHA-256, and installs into ~/.local/bin. It needs no Go, Git checkout,
sudo, or compilation. Set SOCKLENS_INSTALL_DIR to choose another destination;
SOCKLENS_VERSION selects another published version. It does not edit shell
startup files. Uninstall by removing the installed socklens executable.

Native macOS and Windows binaries are not released yet. On Windows, use WSL2.
Homebrew on a Mac cannot run the Linux release; native platform support follows
this installation work.

### Manual download

Download the archive for your Linux CPU and SHA256SUMS from
[GitHub Releases](https://github.com/Thuya-Myint/socklens/releases/tag/v0.1.0).
Verify it with sha256sum -c SHA256SUMS --ignore-missing, extract it, and copy
the socklens executable into a directory on PATH. No Go installation is needed.

## Build from source (contributors only)

Building SockLens is optional for users. Contributors need a supported,
patched Go toolchain (Go 1.26+), GNU Make, and a C compiler for race tests.

```sh
make build
make check
make test
make install
```

make install builds source and installs into ~/.local/bin; it is a development
target, not the normal user install path. PREFIX, BINDIR, and DESTDIR can
override its destination. If Go is outside PATH, set GO=/path/to/go/bin/go
and GOFMT=/path/to/go/bin/gofmt when invoking Make.


No third-party Go modules or runtime tools are required.

```sh
go build -o bin/socklens ./cmd/socklens
./bin/socklens help
./bin/socklens version
./bin/socklens check
./bin/socklens list
./bin/socklens list --protocol tcp --port 8080
./bin/socklens list --listening
./bin/socklens list --pid 12345
./bin/socklens inspect --listening --protocol tcp 8080
./bin/socklens inspect --json 8080
./bin/socklens terminate --pid 12345 --dry-run
./bin/socklens terminate --pid 12345
./bin/socklens watch --listening
./bin/socklens watch --port 8080 --interval 1s --count 5
./bin/socklens watch --protocol tcp --json --count 3
```

Flags must precede positional arguments. List includes all socket states;
inspect filters by **local** port. IPv4 and IPv6 are supported. A successful
empty lookup returns no rows (JSON: []). Errors normally return status 1; wait
uses 2 for timeout and 130 for interruption.

To embed a release version:

```sh
make build VERSION=v0.1.0
```

## Safe termination

Termination accepts one explicit PID, displays the process name, and requires
typing that PID followed by Enter. It sends SIGTERM only; it never escalates
to SIGKILL or claims that the process has exited. PID 1, invalid PIDs, SockLens
itself, and processes owned by another user are refused. Process start time
and ownership are rechecked after confirmation to detect PID reuse.

Before asking for confirmation, SockLens captures a Linux process handle
(pidfd) and holds it until completion or cancellation. SIGTERM is sent through
that handle, preventing the final check-to-signal PID-reuse race. If the target
exits or the handle is unavailable, termination fails without falling back to
PID-based signaling.

Actual termination currently requires Linux amd64 or arm64, Linux 5.3 or newer,
and permission to use pidfd_open/pidfd_send_signal (some sandbox policies block
these calls). Other Linux architectures keep listing, monitoring, waiting, and
dry-run support, while actual termination reports unsupported. Avoid running
SockLens as root; it needs no elevated privileges for normal use.

See the Linux manuals for [pidfd_open](https://man7.org/linux/man-pages/man2/pidfd_open.2.html)
and [pidfd_send_signal](https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html).

## Linux / WSL2 scope

Socket data comes from /proc/net/{tcp,tcp6,udp,udp6}; owners are matched through
/proc/PID/fd. Tables reflect the current network namespace. WSL2 reports the
Linux environment's sockets, not every Windows host process. Permission
restrictions, exiting processes, and kernel sockets can leave PIDs unknown
(shown as "-"). Socket and owner snapshots are not atomic.

## Architecture

- cmd/socklens: executable entry and injected version.
- internal/cli: parsing, validation, output, confirmation.
- internal/platform: shared socket/identity types and build-tagged backends.
- Linux backend: proc parsing, owner matching, and SIGTERM.
- Other platforms: help/version work; OS operations return a clear unsupported
  error. Add windows.go and darwin.go with matching functions to extend support,
  and narrow the unsupported build tag accordingly.

## Development

```sh
gofmt -w cmd internal
go vet ./...
go test -race -cover ./...
```

Tests cover parser failures, IPv4/IPv6, a real local listener and owner lookup,
protected PIDs, stale identities, and termination of a dedicated test child.
CI also cross-compiles Windows/macOS stubs, Linux arm64 handles, and a Linux
s390x build with termination disabled.

The module path is github.com/Thuya-Myint/socklens. The configured remote is
[Thuya-Myint/socklens](https://github.com/Thuya-Myint/socklens).
All build/install/release commands work locally and never push or publish.

## Discovery filters (phase 2)

- --listening includes TCP LISTEN sockets and UDP UNCONN sockets. UDP has no
  TCP-style listen operation; connected UDP sockets are excluded by this flag.
- --pid PID includes sockets with that observed owner. Shared sockets retain all
  observed owners. Sockets with unknown owners cannot match this filter.
- Filters combine with --port, --protocol, and --json; inspect accepts the same
  filters and takes its local port as a positional argument.
- Tables now include PROCESSES entries such as 12345:"node". Process names are
  read from Linux comm (short names, typically limited to 15 bytes), not full
  command lines. Unreadable names show as "-"; terminal control bytes are escaped.
- Existing JSON fields remain intact. Each socket also has a Processes array of
  objects with PID and Name fields; an unavailable name is an empty string.
- Explicit --port 0 and nonpositive --pid values are rejected.

See PHASES.md for completed phases and validation.

## Live monitoring (phase 3)

Use watch to collect successive snapshots. The first appears immediately.
By default, SockLens waits two seconds after each completed snapshot before
collecting the next one. --interval accepts Go durations such as 500ms, 1s,
and 1m, with a minimum of 100ms. Collection never overlaps.

--count N stops after N snapshots; the default (0) watches until Ctrl+C.
SIGTERM also stops watching cleanly on Linux. A stopped watch exits with status
0; invalid options, read failures, or output failures exit with status 1.
Cancellation is checked between snapshots and after collection; an in-progress
proc scan must complete before watch exits.

All list filters work with watch: --port, --protocol, --pid, and --listening.
Each refresh reads new socket and process information. Text output appends a
timestamp and table for every snapshot, preserving the history in your terminal.
It does not clear your screen.

watch --json emits newline-delimited JSON (one compact object per snapshot):

```json
{"timestamp":"2026-10-26T00:00:00Z","sockets":[]}
```

timestamp is the UTC time at completion of collection and filtering. sockets
uses the same socket fields as list --json, including Processes, and is [] when
no sockets match. list/inspect JSON remains a single formatted array.
Default watch mode records full snapshots. Use --changes for additions,
removals, and updates. Polling cannot capture every short-lived connection
between refreshes.

## Change detection (phase 4)

```sh
./bin/socklens watch --changes --listening
./bin/socklens watch --changes --port 8080 --interval 500ms
./bin/socklens watch --changes --json --count 10
```

The first poll always emits a baseline, even if it is empty. Existing sockets
appear in its added list; baseline:true distinguishes initial inventory from
a socket opening later. Later polls emit only changes:

- ADDED: a socket appeared in the filtered results.
- REMOVED: a socket disappeared from the filtered results.
- UPDATED BEFORE / UPDATED AFTER: the same observed socket changed its state,
  PID ownership, or process name. The paired tables retain matching row order.

Change-mode JSON uses one compact object per event batch:

```json
{"timestamp":"2026-10-26T00:00:00Z","baseline":true,"added":[],"removed":[],"updated":[]}
```

added and removed contain socket objects. updated contains objects with before
and after socket objects. Arrays are always present, including empty arrays.
Unchanged polls emit no line. --count still counts polls, including quiet ones,
so --count 10 can emit fewer than ten event batches. Ctrl+C still exits cleanly.

Comparisons happen after filtering. A state change that makes a socket leave
--listening results is therefore a removal; entering the filter is an addition.
Identity uses protocol, inode, and local/remote endpoints. An inode or endpoint
change is a removal plus addition. Owner ordering does not generate false
events. Duplicate rows are counted rather than collapsed; for ambiguous duplicate
identities, exact matches are retained first and remaining updates are paired
deterministically.

These are differences between observed snapshots, not a kernel event stream.
A removal can reflect a filter change or missing visibility; an owner update
can reflect a permission/process-discovery change. Brief sockets and an identity
reused between polls can be missed.

## Waiting for a port condition (phase 5)

```sh
# Wait up to 30 seconds for a local TCP listener:
./bin/socklens wait --listening --protocol tcp 8080

# Wait up to 10 seconds for matching listeners to disappear:
./bin/socklens wait --until absent --listening --timeout 10s 8080

# Wait for a particular process, returning one JSON result:
./bin/socklens wait --pid 12345 --listening --json 8080

# Wait without a deadline; Ctrl+C cancels:
./bin/socklens wait --timeout 0 --interval 1s 8080
```

Flags must precede the required local PORT argument. --until defaults to present;
absent succeeds when no sockets match. Existing --protocol, --pid, and
--listening filters apply. The port is positional, as with inspect; do not
combine it with --port.

The first poll is immediate. --interval defaults to 500ms (minimum 100ms).
--timeout defaults to 30s; 0 disables the deadline. Polling is quiet until
success, timeout, or interruption. An in-progress proc scan must finish before
a deadline or cancellation is handled; a result returned after the deadline is
rejected.

Success emits a confirmation and, if present, matching socket rows. --json emits
one object containing timestamp (UTC), condition, port, and sockets (an array,
including [] for absent). Errors go to stderr and produce no JSON success object.

Exit codes for wait:

| Code | Meaning |
| --- | --- |
| 0 | Condition matched, or help was requested |
| 1 | Invalid options, collection error, or output error |
| 2 | Deadline expired |
| 130 | Interrupted by Ctrl+C or SIGTERM |

wait observes local socket tables and does not make a network connection.
Without --listening, any matching socket state counts, including TCP TIME_WAIT;
use --listening when waiting for listeners to disappear. A present listener
does not prove application health, and an absent result does not guarantee a
subsequent bind will succeed. PID filters depend on best-effort owner visibility.
Conditions can change immediately after a successful poll.

A shell can chain a subsequent step only after success:

```sh
./bin/socklens wait --listening --protocol tcp --timeout 30s 8080 && echo "Listener found"
```

## Termination preview and stable targets (phase 6)

```sh
./bin/socklens terminate --pid 12345 --dry-run
./bin/socklens terminate --pid 12345
```

--dry-run displays the observed process name and PID without reading a
confirmation or sending any signal. It applies the same protected-PID and
same-user ownership checks. It is a preview, not a guarantee that an actual
termination will succeed; it does not require or probe pidfd availability.

Normal termination still requires typing the exact PID followed by Enter.
A stable process handle is acquired before the prompt, identity and ownership
are checked again before sending SIGTERM, and the handle closes on cancellation
and every error path. If the confirmation prompt cannot be written, no signal
is sent. Closing a handle does not terminate its target.

A successful send still does not prove the process exited. No SIGKILL,
confirmation bypass, or automatic escalation is provided.

## Local release packages (phase 7)

```sh
make release VERSION=v0.1.0
cd dist/v0.1.0
sha256sum -c SHA256SUMS
```

This produces:

- socklens_v0.1.0_linux_amd64.tar.gz
- socklens_v0.1.0_linux_arm64.tar.gz
- SHA256SUMS

Each archive contains a versioned folder with the executable, README.md,
LICENSE, CHANGELOG.md, and PHASES.md. The binaries are built with CGO disabled. Only Linux is packaged;
Windows/macOS are still stubs. ARM64 is cross-compiled and not runtime-tested
on this development machine.

The packager validates version/commit labels, builds in temporary folders, and
publishes the completed local package directory in one rename. It refuses to
overwrite an existing version directory or follow an existing destination
symlink. A build failure leaves no completed partial release. Concurrent
packagers use an exclusive version lock; if a process is forcibly killed, inspect
and remove its leftover dist/VERSION.lock before retrying.

For repeatable packages, pin all metadata and use identical source files and
the same Go toolchain:

```sh
make release VERSION=v0.1.0 COMMIT=abcdef123456 BUILD_DATE=2026-10-26T00:00:00Z
```

COMMIT normally comes from the current local Git commit. A source tree without
Git history records unknown. VERSION defaults to dev;
BUILD_DATE defaults to the current UTC time. The version command displays all
three fields. No release number, Git tag, commit, or GitHub release is created
automatically.

make help lists the available targets: build, test, check, install, and release.
The release tool itself uses only the Go standard library. CI tests Go 1.26 and the current stable toolchain, runs a vulnerability check,
and prepares workflow artifacts. The manual release workflow packages an
existing version tag and creates an unpublished GitHub release draft. Nothing is uploaded by the local release command.

## Release installation

Download the matching archive and SHA256SUMS
from [GitHub Releases](https://github.com/Thuya-Myint/socklens/releases).
The release page includes verification and manual installation instructions.

## License

MIT. See LICENSE.
