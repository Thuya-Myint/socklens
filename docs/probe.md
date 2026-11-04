# TCP reachability probes (v0.8.0)

probe actively attempts TCP connections for an explicitly requested host and
numeric port list. It does not send application data, read banners, negotiate
TLS, query remote processes, or change firewalls. DNS is used for hostnames;
numeric IPs bypass DNS. Existing list/inspect/audit/watch/wait commands do not
start probes. probe does not start automatic update checks.

## Commands and bounds

```sh
socklens probe --host 127.0.0.1 --ports 8080
socklens probe --host 192.168.1.20 --ports 80,443
socklens probe --host localhost --ports 8080 --family ipv4
socklens probe --host ::1 --ports 8080 --json
socklens probe --host example.com --ports 443 --connect-timeout 2s --timeout 10s
socklens probe --host example.com --ports 80,443 --concurrency 2
```

| Option | Default | Allowed values |
| --- | --- | --- |
| --host | Required | One numeric unicast IP or ASCII DNS hostname; bracketed IPv6 accepted; scoped IPv6 supported where the OS recognizes the zone |
| --ports | Required | Comma-separated numeric ports 1–65535; at most 64 distinct values; duplicates collapse; no ranges or service names |
| --family | any | any, ipv4, ipv6 |
| --connect-timeout | 3s | 100ms–30s per connection attempt |
| --timeout | 15s | 1s–5m total, including DNS and queued work |
| --concurrency | 4 | 1–32 simultaneous connection attempts |
| --json | Off | One structured report |
| --color | auto | auto, always, never; JSON and diagnostics remain color-free |
| -h, --help | — | Show options without DNS or connection attempts |

Host URLs, embedded ports, CIDRs, multiple hosts, unspecified/multicast/broadcast
addresses and invalid names are rejected. Internationalized domain names need
their ASCII/punycode spelling. IPv4-mapped IPv6 is normalized to IPv4. An explicit
IP contradicting --family is invalid. A hostname with no selected-family answers
produces a resolution error without making connections.

A hostname is resolved once. Selected addresses are deduplicated and sorted;
every port/address pair is dialed as a numeric tcp4/tcp6 endpoint, with no second
DNS lookup or hidden Happy Eyeballs address selection. Maximum plan: 16 selected
addresses and 256 total attempts. Oversized plans fail before dialing, with a
reason and instructions to narrow the host/family/port list. Results are ordered
by numeric port then address, independent of worker completion order.

The total deadline can shorten a per-connection deadline. Queued work after a
deadline/cancellation is marked not-attempted, never reported as refused or timed
out as though a connection had been attempted. Attempted means the dial API was
called, not proof that a packet left the device. Each obtained connection is
closed promptly, including cancellation races; no application Read/Write occurs.

## Outcomes and interpretation

| Status | Meaning |
| --- | --- |
| connected | TCP establishment completed from this client before the result deadline; immediately closed |
| refused | Explicit connection refusal; may originate at target or intermediary |
| timed-out | Connection or total deadline expired during an attempt, or a network timeout was reported; cause unknown |
| network-error | Another connection failure; original error is preserved as text |
| cancelled | An active connection attempt was interrupted; no late success is claimed |
| not-attempted | Queued work was skipped after cancellation/total deadline |

Successful TCP establishment is evidence of reachability at that time, not a
service identity, application health, authorization, TLS or vulnerability verdict.
Timeout alone does not prove closed/filtered or identify a firewall. Local and
remote socket addresses are what the OS connection API reported, not a trace of
routing/NAT, public egress IP, or every network filter traversed. A remote process
PID/owner cannot be determined from this TCP check.

Example: localhost can resolve to both 127.0.0.1 and ::1. If a server listens only
on IPv4, the IPv4 attempt can connect while IPv6 is refused. The command reports
both rather than treating one success as proof that every address works. Use
--family ipv4 or the numeric IPv4 address to ask the narrower question.

If DNS fails, there are no endpoint outcomes and no connection attempts. If the
total deadline expires during DNS, the report says that no endpoint results were
established. Partial reports remain available after Ctrl+C or total deadline.
Ports are not reserved after the probe; observations may immediately change.

## JSON report schema 1

A valid invocation produces one object (including DNS/planning failure and
partial/cancelled operation). Invalid options and output failures use standard
error; JSON output contains no ANSI colors.

| Field | Meaning |
| --- | --- |
| schema_version | 1 for this new command schema |
| timestamp | UTC start time |
| host | Normalized requested host, retaining a DNS name when provided |
| protocol | tcp |
| family | Requested family selector |
| ports | Deduplicated ascending numeric ports |
| addresses | Selected normalized IP/family entries for a valid plan |
| connect_timeout_ms, timeout_ms, concurrency | Configured bounds |
| duration_ms | Overall observed duration, including DNS and work |
| status | completed, timed-out, interrupted, resolution-error, target-error |
| summary | planned, attempted, connected, refused, timed_out, network_errors, cancelled, not_attempted |
| error | Optional code/message for DNS/planning/pre-attempt interruption |
| results | One item per planned IP/port, [] when no valid plan was established |
| notes | Interpretation limits |

Each result has address, family, port, endpoint, status, attempted, duration_ms,
optional local_endpoint/remote_endpoint, optional cause, and message. Failure
endpoints identify the numeric address attempted; success also records the
socket's local/remote endpoint when available. Durations include connection API
and local cleanup time, and are not ICMP ping or application response times.

Cause values include connection-refused, connect-timeout, network-timeout,
total-timeout, network-error, and interrupted. Not-attempted results have zero
duration and a reason, not an invented connection measurement. Empty arrays
remain arrays, not null. Existing discovery/audit JSON schemas are unchanged.

## Exit codes

- 0: every planned TCP connection succeeded.
- 2: at least one connection was unsuccessful/incomplete, or total deadline expired.
- 1: invalid options, DNS resolution/target planning, or output failure.
- 130: interrupted; available partial results are printed.

completed describes a completed batch and can still exit 2 when a port was
refused, timed out, or had another network error. A DNS error is not evidence
about any port. Ordinary DNS failure/its own resolver timeout exits 1; expiration
of the configured total deadline, including during DNS, exits 2.

## Implementation and validation

The Go standard library provides context-aware DNS and TCP connections. No new
Go modules, external networking utilities, background workers, retries, UDP or
application probes are introduced. Native local IPv4/IPv6 listeners and refused
ports are tested on Windows/macOS/Linux, alongside controlled resolver/dialer
tests for address pinning, bounds, timeouts, cancellation, concurrency and cleanup.
The CLI tests cover no-network preflight errors, exit codes, JSON, escaping,
colors, narrow layouts and output failures.

- [Go Dialer.DialContext](https://pkg.go.dev/net#Dialer.DialContext)
- [Go Resolver.LookupIPAddr](https://pkg.go.dev/net#Resolver.LookupIPAddr)
