# SockLens v0.9.1

Run `socklens watch` to watch socket changes using OS events. It prints one
baseline, then stays quiet until matching sockets are added, removed, or updated.
No additional flags or OS-specific command is needed for the default workflow.

## Changes

- Default backend is now events, with change-only output enabled. Unavailable
  tracing produces an error instead of silently starting periodic snapshots.
- Interactive Linux watch offers to install missing bpftrace. It shows the exact
  system package-manager commands and asks for confirmation before installing.
  Supported managers are apt-get, dnf, yum, pacman, zypper, and apk.
- When tracing lacks permissions, interactive Linux/macOS watch requests access
  through sudo and restarts the same watch, preserving filters and color options.
- Interactive Windows watch requests UAC approval when ETW returns access denied.
  After approval it opens the same watch in a new administrator window.
- Declining installation or administrator access stops setup. An already elevated
  process does not repeatedly request access if tracing remains unavailable.
- JSON, redirected, and noninteractive runs never prompt, install, or elevate.
  Prepare dependencies and privileges before using those workflows.

Passwords are handled by the OS. Windows and macOS need no additional tracing
helper installation. Linux still requires bpftrace and compatible kernel probes.
macOS SIP may block DTrace even as root; SockLens reports that limitation and
never changes SIP.

## Compatibility

Polling is available explicitly with `--backend poll`. `--backend auto` explicitly
allows polling fallback when tracing is unavailable. Interactive setup failures
or declined access do not trigger fallback.

Use `--changes=false` for full snapshot output. Existing JSON record fields are
unchanged, but plain `watch --json` now defaults to change batches rather than
full snapshots. Scripts requiring the old snapshot shape must pass
`--changes=false`. `--interval` applies only to polling. `--count` still counts
collected snapshots, including quiet ones. `--count 1` does not start tracing.

Events trigger fresh snapshots, not a lossless socket history: very short-lived
sockets, dropped notifications, ownership changes, and operations outside the
traced hooks can be missed.

## Distribution and upgrade

All six Linux, Windows, and macOS amd64/arm64 packages are provided, with
SHA256SUMS, a Homebrew formula, and Scoop metadata. Installer promotion follows
fresh installation checks. No Go installation is required.

Upgrade with `socklens update`, then verify `socklens version` reports v0.9.1.
On Windows use `socklens update --status` to wait for completion. Homebrew and
Scoop users can also use their package manager's upgrade command.

## Validation

Release CI gates publication on checks, race tests, vulnerability scanning,
native platform tests, event-lifecycle integration tests, and verified packages.
Setup tests cover consent, cancellation, failures, no-prompt execution, argument
preservation, and prevention of elevation loops. Windows native ABI tests cover
the UAC launch structure. Interactive OS approval dialogs require manual testing;
the automated setup tests simulate approval and denial. macOS event tests report
skips when SIP or unavailable probes prevent attachment.

Licensed under MIT.
