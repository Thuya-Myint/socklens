# Changelog

## Unreleased

- Automatic terminal colors with --color auto|always|never and NO_COLOR support.
- Native Windows virtual-terminal support; plain output for redirected streams.
- Grouped help, labeled statuses, readable empty results, and socket summaries.
- Aligned colored tables and safely escaped, more readable process names.
- Terminal-width detection and responsive stacked socket records on all supported platforms.
- Full values wrap in narrow terminals; watch adapts on subsequent snapshots.
- Compact watch timestamps and counted change sections.
- Clear termination confirmation and missing-process guidance.
- Unchanged JSON schemas and exit codes, with color-free JSON diagnostics.

## v0.3.0 — 2026-10-27

- Version output contains only the version number.
- Explicit update-check with stable semantic version comparison and JSON.
- Opt-in daily background checks and cached update notices.
- Update preferences and GitHub release-notification instructions.
- Cross-platform tests for Windows/Linux/macOS update behavior.
- Concurrent Windows settings access supports atomic file replacement.

## v0.2.0 — 2026-10-27

- Native Windows IPv4/IPv6 TCP/UDP tables and process-owner/name discovery.
- Same-user Windows process handles with explicit --force for immediate termination.
- Native macOS sysctl socket tables and version-checked process signaling.
- Native hardware CI for Windows, Intel/Apple Silicon macOS, and ARM64 Linux.
- Six prebuilt release targets; Windows ZIP and Linux/macOS tar archives.
- Generated binary-only Homebrew/Scoop recipes, standalone PowerShell installer.
- No Go requirement for end-user installation.
- Native PID range checks and cross-platform ABI/integration tests.

## v0.1.0 — 2026-10-26

First release for Linux and WSL2.

- TCP/UDP IPv4/IPv6 socket listing and local-port inspection.
- Protocol, local-port, PID, and listener filters with process names.
- Table and JSON output.
- Watch snapshots and change detection with clean interruption.
- Wait for socket presence/absence with timeouts and script-friendly exit codes.
- Same-user SIGTERM with typed confirmation, stable pidfd targets, and dry-run.
- Build metadata, user-local installation, Linux amd64/arm64 archives and checksums.
- Standard-library-only Go implementation, race tests, and CI.
- MIT license.

Windows/macOS socket operations are not implemented. Linux ARM64 is
cross-compiled; runtime validation is on Linux amd64/WSL2.
