# Local listener audit

Available in v0.6.0. This command reads the current local socket tables and explains
binding scope. v0.7.0 adds read-only firewall context and peer-specific candidate
filtering. It does not perform network probes or service/vulnerability tests.

## Commands and filters

```sh
socklens audit
socklens audit --port 8080
socklens audit --protocol tcp --pid 12345
socklens audit --json
socklens audit --color never
socklens audit --help
```

Replace 12345 with an observed PID. --port, --protocol, and --pid combine. Ports
must be 1–65535; PIDs must be positive. No positional port or --listening option
is accepted. TCP LISTEN and UDP BOUND/UNCONN observations are included. Other TCP
states and UDP rows recorded as connected are excluded. Windows UDP tables do
not expose connection state, so Windows UDP observations are shown as bound.

The command reads one snapshot. Process metadata can be unavailable or partial
because of permissions or changes during collection. WSL2 observes its Linux
environment. Containers and network namespaces limit the environment visible.

## Process/service names and communication (v0.7.2)

All platforms show observed TCP/UDP inbound binding scope and outbound visibility.
A loopback binding is described as local applications via loopback, not a firewall
permission. Wildcard/specific bindings do not establish remote access. Windows
UDP tables do not report a remote peer; no UDP peer is invented. TCP listeners
refer to inspect for accepted connection rows rather than claiming an outbound
probe or connection was observed. A supplied --peer is labeled requested.

Windows preferred executable-name lookups now fall back to a native Toolhelp
process snapshot. This improves names in list, inspect, watch, wait and audit
without treating a basename as a full path or verified identity. Audit separately
queries native SCM service status with read-only enumeration rights; no service
control/elevation is attempted. Display names, service names, state, PID, source
and own/shared-host type are best-effort observations. Permissions can silently
omit services; no service listed does not establish that the process is not a
service. Shared-host service entries do not attribute a socket to one service.
Other OSes keep native process discovery and omit Windows service metadata.

JSON adds optional service_lookup status/source/detail and listeners[].services
records (pid/name/display_name/state/process_type/source), retaining schema 1.
Existing discovery arrays retain the same fields; formerly missing Name values
may now be populated on Windows. Termination identities and safeguards are not
changed and never use this display-only fallback.

## Concise and detailed text (v0.7.1)

The default text view emphasizes process/PID, binding, findings, next steps,
coverage, and conditional inbound/outbound rule summaries. Source/profile context
appears once per snapshot. Program summaries show basenames with a prompt to
view full paths; empty restriction arrays are labeled unresolved rather than any.

```sh
socklens audit --port 8080
socklens audit --port 8080 --details
socklens audit --details --rule-limit 0
```

--details shows full evidence, profile/interface metadata, rule IDs, restrictions,
raw policy, program paths, and interpretation notes. The configured text rule
limit still applies. --details --rule-limit 0 displays all candidates in full.
The JSON data, finding levels, reading order and exit codes are unchanged by this
presentation flag. Width wrapping, automatic color and safe escaping apply to
both text views. Unknown source failures remain visible in the concise overview.

## Firewall context (v0.7.0)

Firewall queries are enabled by default. Use --firewall=false to skip them.
--peer IP filters candidate rules by a numeric unicast address; it requires
firewall collection and does not contact the peer. --firewall-timeout defaults
to 20s (1s–2m). --rule-limit defaults to 3 candidates per direction per listener
in text; 0 removes the selected view's limit. Use --details to include unverified
candidates; JSON always contains all collected candidates.

See the [firewall reference](firewall.md) for collectors, matching, uncertainty,
JSON fields, conditions, and examples. These options combine with the socket
filters. An inaccessible firewall source does not erase the socket snapshot:
it produces partial/unknown coverage and a reason. Cancellation still exits 130.

## Binding scopes and rules

| Scope | Meaning | Binding finding |
| --- | --- | --- |
| loopback | Loopback address, such as 127.0.0.1 or ::1 | information, loopback-binding |
| wildcard | IPv4 0.0.0.0 or IPv6 :: | review, wildcard-binding |
| private | Private IPv4 range or IPv6 unique-local address | information, specific-binding |
| link-local | Link-local unicast address | information, specific-binding |
| multicast | Multicast address | information, specific-binding |
| specific-address | Another specific address; no Internet-reachability claim | information, specific-binding |

Mapped IPv4 addresses are classified by their IPv4 address while retaining the
ipv4-mapped-ipv6 family label. IPv6 :: is an IPv6 wildcard observation; IPv4
acceptance is not established by the table. Zone identifiers are preserved.

Binding findings have confidence observed because they explain address data in
the snapshot. The labels do not describe authentication strength or exploitability.
A broad binding can be intentional; a loopback/private address is not a security
verdict. External reachability requires separate evidence.

## Unverified service hints

The initial hint set covers these database/administration port conventions:

| Transport | Port | Hint |
| --- | --- | --- |
| TCP | 22 | SSH |
| TCP | 3306 | MySQL |
| TCP | 3389 | Remote Desktop |
| TCP | 5432 | PostgreSQL |
| TCP | 6379 | Redis |
| UDP | 161 | SNMP |

Every hint has source port-convention and verified false. These are hints only:
any program can use a port, and a service can use another port. They are not
fingerprints. The list is intentionally small rather than a vulnerability catalog.

A non-loopback listener on a hinted port adds a review finding with rule ID
sensitive-service-port and confidence heuristic. Its recommendation is to identify
the actual service and review intended clients, access controls, authentication,
and encryption if the hint applies. No authentication or TLS check was performed;
for example, the Redis port does not establish whether TLS is enabled.

A loopback listener can still show a service hint, but it does not receive the
non-loopback service review finding. Multiple findings for a listener count as
one listener in summary.review_listeners.

## Reading order (v0.6.1 and newer)

Text and JSON listener records use the same reading priority:

1. Non-loopback listeners on a hinted database/administration service port.
2. Other review items (currently wildcard bindings).
3. Other non-loopback specific-address bindings.
4. Loopback bindings.

Within the first group, wildcard bindings precede specific bindings. Within each
remaining tie, numeric port comes first, then protocol and numeric address, with
socket identity and normalized ownership as deterministic final tie breakers.
For example, port 80 precedes 443 and address 10.0.0.2 precedes 10.0.0.10.
Review findings precede information findings inside each listener record.

This order helps choose which configuration questions to read first. It is not
a severity ranking: service hints remain unverified and missing process metadata
does not increase a listener's priority. A loopback record is not a safety verdict.
Filters, finding levels, summary counts, and schema_version 1 remain unchanged.

## Windows TCP collection

Windows discovery queries listener and connection tables separately. Membership
in the native listener-only table establishes LISTEN even on systems that report
zero TCP states in the combined table. Connected rows retain their state from
the connection table. This improves listener filtering for audit and the existing
list/inspect/watch/wait commands without guessing from remote addresses.

## Example interpretation

For TCP 0.0.0.0:5432, the observed fact is an IPv4 wildcard listener. PostgreSQL
is an unverified hint from the port. The recommendation is to confirm the actual
service and whether access beyond localhost is intended. No external reachability,
configuration weakness, or CVE is established from those observations.

For TCP 127.0.0.1:5432, the observed fact is a loopback listener in the local
network environment. Local processes may still connect. Application security
and any forwarding/container configuration need separate checks.

## JSON report

--json returns one object, including when there are no listeners. No result is
printed on discovery failure; the error goes to standard error and exit code 1.

| Field | Meaning |
| --- | --- |
| schema_version | Integer 1; v0.7.0 adds optional firewall fields |
| timestamp | UTC snapshot analysis time |
| scope | local |
| external_reachability | not-tested |
| summary | listeners, review_listeners, information_listeners, findings, missing_process_details |
| listeners | Array, [] when empty |
| notes | Interpretation and visibility limitations |

Each listener has protocol, local, state, optional socket_id, address_family,
binding_scope, pids, processes, process_visibility, optional service_hint, and
findings. Since v0.7.0, an optional firewall context is attached to each listener
and the report includes the collection snapshot. --firewall=false omits these
additive fields. Processes contain lowercase pid and name fields in this report;
existing list/inspect JSON formats are unchanged.

process_visibility is observed when all reported PIDs have names, partial when
some names are missing, and unknown when no PIDs are reported. Even observed
metadata is a best-effort snapshot, not service identity verification.

A finding contains rule_id, level (information/review), confidence
(observed/heuristic), title, evidence (an array of observations), and recommendation.
A service hint contains name, source, and verified false.

Results use the reading order above for repeatability, but timestamps and live observations may
change. Repeated socket rows are kept as separate observations; socket identity
and owner metadata are platform-dependent.

## Exit codes and output

- 0: completed, including a report containing review items.
- 1: invalid options, discovery/analysis failure, or output failure.
- 130: interrupted. Press Ctrl+C to cancel.

Use the JSON summary for scripts. There is no security score or automatic remediation.
Text records wrap to terminal width, process names are escaped for terminal safety,
and color handling follows the global --color option. JSON and its diagnostics
remain color-free. Empty results and information labels do not imply safety.

No network connection or daily update worker is started by audit, even if daily
notices have been enabled. The Go module has no external dependencies. Policy
enrichment uses local OS utilities with explicit missing-tool/permission status.

## References and scope

- [Go net/netip address classification](https://pkg.go.dev/net/netip): numeric address parsing and scope predicates; no DNS lookup.
- [IANA service name/port registry](https://www.iana.org/assignments/service-names-port-numbers): assignments support hints, not confirmation of the actual service.
- [Redis installation](https://redis.io/docs/latest/install/install-redis/): default Redis port convention.
- [Microsoft service port reference](https://learn.microsoft.com/en-us/troubleshoot/windows-server/networking/service-overview-and-network-port-requirements): Remote Desktop and SNMP port conventions.
- [Windows TCP table classes](https://learn.microsoft.com/en-us/windows/win32/api/iprtrmib/ne-iprtrmib-tcp_table_class): listener-only query semantics.
- [NIST SP 800-115](https://csrc.nist.gov/pubs/sp/800/115/final): port/service discovery and vulnerability assessment require different evidence.

Remote host checks and service/protocol security testing remain roadmap work.

Windows owner/service references:

- [PROCESSENTRY32W executable-name observations](https://learn.microsoft.com/en-us/windows/win32/api/tlhelp32/ns-tlhelp32-processentry32w)
- [EnumServicesStatusExW read-only enumeration and permission omissions](https://learn.microsoft.com/en-us/windows/win32/api/winsvc/nf-winsvc-enumservicesstatusexw)
- [Service process/status records and shared hosts](https://learn.microsoft.com/en-us/windows/win32/api/winsvc/ns-winsvc-service_status_process)
