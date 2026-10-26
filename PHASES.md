# SockLens phase reports

Phase 6 supersedes the earlier final check-to-signal PID-reuse race limitation.

## Phase 1: core CLI and Linux backend

Features: help/version/check, TCP/UDP IPv4/IPv6 enumeration, local-port inspection,
JSON/table output, PID ownership discovery, confirmed same-user SIGTERM,
Linux/WSL2 scope, platform stubs, README, and GitHub Actions CI.

Validation: parser tests, live listener lookup, dedicated child termination,
confirmation cancellation, protected PIDs, stale identity rejection, race
checks, static analysis, Linux build, and Windows/macOS cross-compilation.

Limitations: Windows/macOS socket operations are stubs; owner discovery is
best-effort; a narrow identity-check-to-signal PID-reuse race remains.

## Phase 2: discovery filters and process names

Features: --listening and --pid filters on list/inspect, combined filtering,
best-effort process names, escaped process names in tables, additive Processes
metadata in JSON, stricter explicit port/PID validation, and updated help/docs.

Functions: filterSockets selects socket snapshots; writeSockets renders tables
or JSON; Linux List now enriches socket owners with process names. Existing
termination behavior is unchanged.

Validation: combined/individual filters, shared and unknown owners, IPv6 rows,
UDP listener semantics, output errors, empty JSON, terminal control escaping,
and live TCP/UDP discovery including PID and name metadata. Passed: go vet, go test -race -cover, the Linux build, Windows amd64 and macOS
arm64 cross-compilation, and command smoke checks. Coverage: CLI 82.9%, Linux
platform 86.0%. GitHub-hosted CI has not run because nothing was pushed.

Limitations: names are short Linux comm values; snapshots can change during
collection; platform and termination limitations from phase 1 remain.

## Phase 3: live port monitoring

Features: watch command, immediate first snapshot, configurable --interval
(default 2s, minimum 100ms), optional --count, all existing discovery filters,
timestamped table history, newline-delimited JSON, and clean Ctrl+C/SIGTERM
shutdown. Read/output errors fail clearly. Uses the Go standard library only.

Functions: RunContext dispatches with cancellation; addSocketFlags and
socketOptions.validate share discovery options; watchCommand validates watch
settings; watchSnapshots collects, filters, renders, waits, and stops.

Validation: snapshot counts and stream shape, per-refresh filtering, empty
results, cancellation before/during collection and during waits, no delay after
the last snapshot, read/write failures, invalid flags, and a live listener
disappearing between snapshots. Passed: go vet, go test -race -cover (CLI 86.9%, Linux platform 86.0%),
Linux build, Windows amd64/macOS arm64 cross-compilation, finite JSON watch,
help/version/check smoke checks, and actual SIGINT/SIGTERM interruption while
the executable was waiting on a one-hour refresh. GitHub CI has not run because
nothing was pushed.

Limitations: refreshes are polling snapshots, so short-lived sockets can be
missed; cancellation waits for a current proc scan; Windows/macOS socket
operations and phase 1 termination limitations remain.

## Phase 4: watch change detection

Features: --changes mode, explicit initial baseline, ADDED/REMOVED events,
before/after state and owner updates, quiet unchanged polls, JSON change batches,
all existing filters, and poll-based --count behavior. Default snapshot mode
is preserved. Standard library only.

Functions: diffSockets compares snapshots; cloneSockets normalizes owner order
and copies data; groupSockets preserves duplicate identities; writeChanges and
writeChangeSection render event batches. watchSnapshots maintains the previous
filtered snapshot.

Validation passed: state/owner/name updates, inode/endpoint/protocol replacement,
owner and row order independence, duplicate multiplicity, nil/empty owners,
deterministic output, quiet poll counting, filtered empty baseline, output
failures, cancellation, real listener removal in integration tests, and actual
binary detection of a listener opening and closing. Ctrl+C and help/check smoke
tests passed. go vet, race tests (CLI 90.0%, Linux platform 86.0%), Linux build,
Windows amd64 and macOS arm64 cross-compilation passed. GitHub CI has not run;
nothing was pushed.

Limitations: changes are observed snapshot differences, not guaranteed kernel
events. Visibility and filters can produce apparent additions/removals.
Ambiguous duplicate updates are paired deterministically; brief socket lifetimes
or identity reuse between polls may be missed. Earlier platform/termination
limitations remain.

## Phase 5: wait for local port conditions

Features: wait PORT with --until present/absent; immediate first poll;
--timeout (30s default, 0 unlimited); --interval (500ms default, 100ms minimum);
shared protocol/PID/listening filters; quiet polling; text or one JSON success
result; exit codes 0 success, 1 error, 2 timeout, 130 interruption. Ctrl+C and
SIGTERM cancel waiting. Uses only the Go standard library.

Functions: waitCommand parses settings and maps outcomes to exit codes;
waitForSockets polls filtered local tables with deadline/cancellation checks;
writeWaitResult renders a successful result. Shared positional-port validation
now covers inspect and wait, and main installs signal handling for both watch
and wait.

Validation passed: immediate and delayed present/absent conditions, combined
filters, timeout during sleep, rejection of late success, cancellation before
and during collection, collection/filter/output errors, argument validation,
JSON shape, and live listener integration tests. Actual executable smoke tests
confirmed pending waits complete on listener opening/closing, timeout code 2,
SIGINT/SIGTERM code 130, and help/check behavior. go vet, race tests (CLI 90.9%,
Linux platform 86.0%), Linux build, Windows amd64 and macOS arm64
cross-compilation passed. Nothing was pushed; GitHub CI has not run.

Limitations: socket presence is not application health; absence does not reserve
a port or guarantee bind success. Without --listening, all matching states count.
Owner visibility is best-effort. Deadlines/cancellation wait for an active proc
scan to finish, and polling can miss brief changes. Earlier platform/termination
limitations remain.

## Phase 6: stable termination targets and dry-run

Features: terminate --dry-run previews without prompting/signaling; actual
termination captures a Linux pidfd before confirmation and signals that stable
kernel object after rechecking identity and ownership. Missing/blocked handles
cause refusal; there is no PID-based fallback. Prompt output failures cancel
before signaling. Handles close on completion, cancellation, and errors.

Functions: Target defines Identity/Terminate/Close; OpenTarget captures and
validates a process handle; linuxTarget owns its lifetime; pidfdOpen and
pidfdSignal provide standard-library-only syscalls for Linux amd64/arm64.
The identity-based Terminate helper also uses stable handles.

Validation passed: stale identities, protected PIDs, closed handles, invalid
handles, exited/reaped target returning ESRCH, no-signal dry runs, failed-prompt
refusal, and prior confirmed/cancelled termination tests. Actual binary tests
verified a pidfd exists while confirmation is pending, cancellation leaves its
child alive, confirmation sends SIGTERM, and target exit during confirmation
causes refusal. go vet, race checks (CLI 90.6%, Linux platform 85.7%), Linux
amd64 build, Linux arm64/s390x and Windows/macOS cross-compilation passed.
CI definitions include the new architecture checks; no remote CI has run and
nothing was pushed.

Limits: actual termination requires Linux 5.3+ on amd64/arm64 with pidfd syscalls
permitted. Other Linux architectures refuse actual termination while preserving
read-only commands and dry-run. Dry-run does not test handle availability.
SIGTERM delivery does not guarantee exit. Windows/macOS operations remain stubs.

References:
- https://man7.org/linux/man-pages/man2/pidfd_open.2.html
- https://man7.org/linux/man-pages/man2/pidfd_send_signal.2.html

## Phase 7: installable builds and local release packages

Features: make build/test/check/install/release/help; install to ~/.local/bin;
version/commit/build-date metadata; dependency-free Go release packager for
Linux amd64/arm64; SHA256SUMS; validated labels; deterministic archives with
fixed source/toolchain/metadata; no-overwrite behavior; atomic completed package
publication; CI Go 1.22/stable matrix and local package artifact checks.

Repository: Go module github.com/Thuya-Myint/socklens; origin is
https://github.com/Thuya-Myint/socklens.git. No initial commit or push is created.

Functions: versionInfo formats build metadata; run parses release settings;
packageRelease builds and stages packages; writeArchive writes reproducible
archives; buildEnvironment isolates target OS/architecture/CGO settings.

Validation: Make help/check/test/build passed; race tests passed including
archive contents and permissions, checksums, identical-input reproducibility,
failed-build cleanup, unsafe-label rejection, and existing-output preservation.
Install/package smoke checks passed: installed and extracted Linux amd64
binaries are byte-identical, version metadata and check work, both architecture
checksums verify, and a new login shell finds ~/.local/bin/socklens. Windows,
macOS, and Linux s390x still cross-compile. Linux ARM64 packages compile.
GitHub CI has not run because nothing was pushed.

Limits: Go is required to rebuild, Make for the convenience targets, and a C
compiler for race tests. ARM64 is compiled only. Commit metadata is unknown
until an initial local commit exists. Older platform limitations remain.

## Production preparation: v0.1.0

MIT selected by the owner. Added license, changelog, release notes, contributor
guidance, security policy, and a release runbook. Release archives include
LICENSE and CHANGELOG.md. Build minimum is now Go 1.26, tested with Go 1.26.10
and current Go 1.27.3. GitHub Actions are pinned to official release commits,
with vulnerability scanning and a manually dispatched unpublished-draft workflow.

Audit fixes: metadata/help commands reject unexpected arguments and report
output failures; check reports output failure; confirmation input is bounded;
an empty proc table cannot silently imply that a port is absent.

Validation passed: formatting/static analysis, full race-enabled tests on both
tested Go releases, Windows/macOS/Linux ARM64/s390x builds, actionlint workflow
validation, source secret-pattern check, and govulncheck v1.9.0 (no known
vulnerabilities found). Existing socket lifecycle and pidfd safety tests pass.

Publication status: prepared locally. Owner approval is required before pushing
main or v0.1.0 and publishing a GitHub release. Remote CI is pending that push.
ARM64 is compile-tested, not runtime-tested. No release has been published.
