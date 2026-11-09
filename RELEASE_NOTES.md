# SockLens v0.9.0

Watch sockets using OS event notifications on Linux, Windows, and macOS, with
explicit backend selection and a polling fallback when tracing is unavailable.

```sh
socklens watch --changes
socklens watch --backend events --changes --json
socklens watch --backend poll --interval 1s
```

## Included

- Linux eBPF notifications through an installed bpftrace executable.
- Native Windows ETW subscriptions to Winsock-AFD and Kernel-Network events,
  with no helper executable or additional Go dependencies.
- macOS notifications through the system /usr/sbin/dtrace tool.
- Default --backend auto attempts tracing, reports the active backend on stderr,
  and falls back to polling if attachment fails.
- --backend events requires tracing and reports attachment failures; --backend
  poll explicitly selects the previous periodic snapshot behavior.
- An immediate baseline, existing filters, change batches, and unchanged JSON
  record schemas. Notifications trigger fresh socket-table snapshots.
- Coalesced notifications, cancellation, bounded helper diagnostics, and explicit
  failure if a tracing source stops after attachment.

## Requirements and limitations

Tracing requires appropriate privileges, usually root on Linux/macOS and an
administrator or tracing-enabled account on Windows. Linux additionally requires
bpftrace and the relevant kernel tracepoints. macOS SIP can restrict probes even
as root: auto mode falls back, and SockLens never changes SIP or elevates itself.

Event snapshots are best-effort, not a lossless socket history. Short-lived
sockets, dropped notifications, ownership changes, and UDP operations outside
the traced hooks may be missed. Descriptor closes or Windows network activity
can cause unchanged refreshes. ETW buffering can delay updates by about a second.
Use polling if periodic reconciliation is needed.

--interval applies only to polling. --count counts collected snapshots, including
quiet ones with --changes; an idle event watch can wait indefinitely before
reaching the count. --count 1 collects once without starting tracing.

## Distribution and upgrade

Release packages cover Linux, Windows, and macOS on amd64 and arm64, with
SHA256SUMS, Homebrew formula, and Scoop manifest. Installer promotion follows
publication and fresh installation checks. No Go installation is required.

After publication, run socklens update and socklens version. On Windows, use
socklens update --status to wait for completion. Existing Homebrew and Scoop
installations can also use their package manager's upgrade command.

## Validation

CI gates publication on checks, Linux race tests, vulnerability scanning,
native Windows/macOS/Linux tests, and verified packaging. Privileged Linux and
Windows tracing tests exercise TCP/UDP socket opens and closes. macOS tracing
tests explicitly report skips when SIP or unavailable probes prevent attachment;
skips do not establish that tracing works on those hosts.

Licensed under MIT.
