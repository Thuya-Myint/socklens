# Watch reliability and diagnostics

`watch` observes socket-table snapshots triggered by OS events. It also refreshes
after an idle reconciliation timeout so persistent changes can be recovered when
notifications are missed. This is a best-effort view, not a complete socket history.

```sh
socklens watch --stats
socklens watch --json --stats --port 8080
socklens watch --event-interval 250ms --reconcile 5s
socklens watch --backend poll --interval 1s --stats
socklens watch --count 1 --stats
```

## Backend and permissions

| Mode | Behavior |
| --- | --- |
| `--backend events` (default) | Requires a working event subscription; setup failures stop the command. |
| `--backend auto` | Explicitly permits polling fallback if subscription fails. The reason is printed to stderr. |
| `--backend poll` | Explicit socket-table polling; event counters are not applicable. |
| `--count 1` | Single immediate snapshot; tracing is not started. |

Linux uses bpftrace and requires tracing privileges. Windows uses ETW and requires
administrator/sufficient tracing permissions. macOS uses DTrace; SIP may restrict
it even with administrator privileges. Interactive setup can request access;
JSON, redirected and noninteractive runs never prompt or elevate. `doctor` checks
prerequisites without proving that attachment will succeed. A runtime source
failure stops the watch; `auto` does not silently switch backends mid-session.

bpftrace/DTrace helper attachment has a 30-second deadline to allow probe
compilation/attachment on busy hosts. Cancellation interrupts startup sooner.
Attachment timeouts include bounded helper stderr diagnostics; increasing the
deadline does not bypass missing privileges or unavailable probes.

## Event bursts and idle reconciliation

The baseline is immediate. `--event-interval` defaults to `100ms`, with a minimum
of `100ms` and a maximum equal to `--reconcile`. After each event-mode snapshot
finishes writing, the next event-triggered collection waits for this minimum gap.
An event arriving after the gap triggers collection immediately. Notifications
received while collecting or waiting share a one-entry pending queue.

New notifications do not restart the delay. A continuous burst therefore cannot
postpone the pending collection indefinitely or trigger an unlimited number of
collections per second. Collection and output are synchronous and never overlap.
This limits collection work, not OS tracing volume, callback CPU, or helper output.

`--reconcile` defaults to `20s` and has a minimum of `1s`. If no event is delivered
within that idle wait, the next collection is attributed to reconciliation.
Event throttling does not extend the reconciliation wait. Collection/output time
and scheduler latency add to the wait; it is not a hard wall-clock guarantee.
Cancellation interrupts waits and pending burst delays. An in-flight native
collection still has to return before the command can stop.

## `--stats` output

When monitoring finishes, `--stats` writes a plain-text diagnostic summary to
stderr, including on collection/event errors after successful setup. It does not
change the JSON records on stdout. Setup failures retain their direct error
messages and do not produce a monitoring summary.

| Field | Meaning |
| --- | --- |
| `backend` | Selected `events`, `poll`, or single `snapshot` mode. |
| `source` | `bpftrace`, `etw`, or `dtrace` for events; `none` otherwise. |
| `collections` | Number of attempted socket-table collections, including failed attempts. |
| `collection_total` | Cumulative collection-call time, excluding filtering and output. |
| `baseline` | Initial collection attempts, normally 1. |
| `events` | Collection attempts following a delivered event invalidation. |
| `reconciliations` | Collection attempts after the idle event wait timed out. |
| `polls` | Collection attempts after a polling delay. |
| `collection_errors` | Failed socket-table collection calls. |
| `event_errors` | Runtime event-source wait failures; normal idle timeouts/cancellation are excluded. |
| `notifications` | Notifications delivered to SockLens's helper reader or ETW callback. Not a socket-change count. |
| `coalesced` | Notifications delivered while an invalidation was already pending. Not an OS event-loss count. |
| `backend_drops` | `unavailable`: kernel/tracer event loss is not measured by the current backends. Never infer zero loss. |
| `reconcile` / `event_interval` | Configured idle reconciliation wait / minimum event-mode gap. |

`baseline + events + reconciliations + polls` equals `collections`. These counts
include quiet snapshots; output may contain only the baseline if nothing changed.
`notifications` and `coalesced` are cumulative concurrent counters, so a read is
not an atomic snapshot of the pair. A source without diagnostics reports
`unavailable`. Event-specific counters report `not-applicable` in polling and
single-snapshot modes. Future consumers should not treat the text summary as a
versioned JSON API.

## What these statistics cannot establish

- A socket that opens and closes between snapshots can be missed even when
  notification delivery is perfect.
- Coalesced notifications can include unrelated activity and do not establish
  missing sockets or kernel drops.
- Reconciliation can recover a persistent observed state, not reconstruct past
  transient activity or prove external reachability.
- Ownership, socket visibility and namespaces still depend on OS permissions.
- macOS tracing may be unavailable under SIP; backend buffering can delay events.

Automated coverage includes burst rate limiting, reconciliation, cancellation,
source failures, empty/unchanged output, and notification counters. Privileged
event attachment and lifecycle coverage runs separately in platform CI; macOS
CI records a skip when tracing is restricted. Compilation alone is not runtime
validation of a tracer.
