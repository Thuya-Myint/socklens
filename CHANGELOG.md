# Changelog

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
