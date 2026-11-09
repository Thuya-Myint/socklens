# Changelog

## v0.9.0 — 2026-11-09

- Event-triggered watch snapshots through Linux bpftrace, native Windows ETW, and macOS system DTrace, with explicit auto/events/poll backend selection.
- Automatic polling fallback when tracing is unavailable; existing JSON record schemas are preserved. macOS SIP restrictions are reported without changing security settings.
- Helper lifecycle/cancellation tests, Windows ETW ABI regressions, and opt-in privileged TCP/UDP lifecycle integration tests. CI requires Linux and Windows tracing checks and explicitly reports macOS skips when SIP prevents tracing.
- Event snapshots remain best-effort and can miss short-lived sockets; tracing privileges are required. Only Linux needs an additional helper installed.

## v0.8.0 — 2026-11-04

- Explicit TCP probe command for one host and selected numeric ports, with IPv4/IPv6 and one-time DNS resolution.
- Numeric address pinning, bounded concurrency, per-attempt/total deadlines, cancellation and immediate connection cleanup.
- Connected/refused/timeout/network-error outcomes and explicit cancelled/not-attempted partial results.
- Versioned JSON, responsive escaped/color-controlled output and script-friendly exit codes.
- No application payloads, UDP probes, remote owner guesses or background update checks.
- Native loopback/refusal and controlled DNS/concurrency/deadline regressions across Windows, macOS and Linux.

## v0.7.2 — 2026-11-04

- Native Windows executable-name fallback fills missing names when full-image access is denied.
- Read-only Windows service names/display names/states in audit, kept separate from executable identity and shared-host socket attribution.
- Plain inbound TCP/UDP binding scope and outbound peer visibility on macOS, Windows and Linux.
- Default rule evidence favors explicit endpoint/process relation; broad and identity-unresolved candidates remain in details/JSON.
- Windows dotted subnet masks and structured rule relevance/identity evidence; no inferred allow/block verdict.
- Cross-platform output, metadata/ABI/escaping/cancellation and native Windows regressions.

## v0.7.1 — 2026-11-04

- Concise audit text with combined process/PID details, short next steps, and conditional firewall rule summaries.
- Consolidated profile/source context and interpretation notes reduce repeated log text.
- --details restores full evidence, metadata, restrictions, program paths and raw policy; combine with --rule-limit 0 for all candidates.
- Empty rule restrictions remain unresolved; JSON data, reading priority and collection behavior are preserved.
- Both text views retain terminal wrapping, color controls, safe escaping and explicit coverage limits across platforms.

## v0.7.0 — 2026-11-03

- Read-only local firewall context in audit on Windows, macOS, Linux and WSL.
- Numeric --peer filtering, binding-family explanations and actual interface addresses.
- Conditional inbound/outbound rule candidates, IP/port ranges, application restrictions and source/profile evidence.
- Explicit partial/unknown coverage, source failures, ambiguous rules and no final reachability verdict.
- Windows ordinary-user COM fallback when bulk NetSecurity filters are denied.
- --firewall=false, bounded query deadlines and text rule limits; additive JSON fields.
- Policy/parser/CLI regressions and native read-only collection checks across platforms.

## v0.6.1 — 2026-11-02

- Audit reading priority puts non-loopback service-port hints and other review items before information.
- Wildcard bindings lead within hinted-service reviews; numeric port/address ordering resolves ties.
- Review findings appear before information within each record; text and JSON share deterministic ordering.
- Explain the order in output, help, and documentation without implying vulnerability severity.
- Shared ordering and CLI regressions run on macOS, Windows, and Linux.

## v0.6.0 — 2026-11-02

- Passive local audit command with port, protocol, PID, JSON, and color options.
- Binding classification for IPv4, IPv6, and mapped IPv4, with explicit reachability limits.
- Observed information/review findings and unverified database/administration port hints.
- Versioned JSON reports, responsive records, safe process rendering, and offline operation.
- Shared listener selection and native TCP/UDP audit integration tests on every platform.
- Windows TCP listener/connection queries avoid zero-state entries in combined tables.

## v0.5.2 — 2026-11-02

- Resolve Windows Scoop current junctions and custom installation roots using native file handles.
- Include the failing executable path in update-detection errors.
- Add a real-junction Windows regression for Scoop detection, delegation, and status.

## v0.5.1 — 2026-11-02

- Clarify wait present/absent conditions, immediate success, and both examples.
- Hide the unsupported --port option from wait/inspect help and describe JSON output per command.
- Expand wait documentation and record the proposed security-analysis roadmap.

## v0.5.0 — 2026-10-27

- `socklens update` checks and installs a newer stable release in one command.
- Receipt-aware Homebrew and Scoop upgrades with installed-version verification.
- Verified standalone downloads for macOS, Windows, Linux, and WSL2.
- SHA-256, architecture/version checks, concurrent-change protection, and rollback.
- Windows background installation with `update --status` and a per-update log.
- `update --dry-run` and a configurable total timeout; no automatic installs.
- Update notices point to the new update command.
- Linux and Windows integrity, rollback, package-manager, and command tests.

## v0.4.0 — 2026-10-27

- Version-tag CI publishes verified releases and promotes all installers after tests.
- A Scoop bucket enables normal scoop update upgrades.

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
