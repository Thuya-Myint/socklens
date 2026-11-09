# SockLens

Inspect local TCP/UDP ports, review listener exposure, test TCP reachability to selected hosts and ports, see processes, monitor changes, and terminate a process with confirmation. Available for macOS, Windows, Linux, and WSL2.

## Installation

Native macOS and Windows installation requires v0.2.0 or newer.

Install a ready-to-run binary. Go, a compiler, and a source checkout are not required.

### Homebrew — macOS and Linux

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
socklens version
```

Homebrew selects the binary for Intel/AMD 64-bit or ARM64, including Apple Silicon.

### Scoop — Windows

With Scoop installed, run in PowerShell:

```powershell
scoop bucket add socklens https://github.com/Thuya-Myint/socklens
scoop install socklens/socklens
socklens version
```

The bucket selects the x64 or ARM64 Windows binary and receives new release manifests automatically.

### Linux — binary installer

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
socklens version
```

The installer detects your CPU, verifies the download checksum, and installs into ~/.local/bin without sudo. Run the commands inside WSL2 if you use this method on Windows.

To choose another installation directory or published version:

```sh
SOCKLENS_INSTALL_DIR="$HOME/bin" SOCKLENS_VERSION=v0.9.0 sh /tmp/socklens-install.sh
```

The installer does not change shell startup files. Add your installation directory to PATH if you want to run socklens from any terminal.

### Upgrade an existing installation

From v0.5.0 onward, upgrade with one command:

```sh
socklens update
```

It checks for a newer stable release and uses Homebrew, Scoop, or a verified
standalone binary download to upgrade the current installation. See [update](#update)
for preview, timeout, and Windows progress options.

**v0.4.0 and older need one initial upgrade using the commands below** to receive
this new command. An already installed binary cannot gain a command until it is upgraded.

With Homebrew:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
```

With Scoop bucket installations:

```powershell
scoop update
scoop update socklens
```

If you installed an older version from a direct manifest URL, migrate once:

```powershell
scoop uninstall socklens
scoop bucket add socklens https://github.com/Thuya-Myint/socklens
scoop install socklens/socklens
```

On Linux/WSL2, rerun the binary installer commands above to replace the installed
binary. Download the installer again so it selects the current release.

Verify the upgrade:

```sh
socklens version
socklens update-check
```

The update-check and updates commands require v0.3.0 or newer.

## Terminal output and colors

Human-readable output uses aligned socket tables, a socket-count summary, clear
empty results, and labeled status messages. Watch output groups additions,
removals, and updates with UTC timestamps. Regular process names are shown
without extra quotes; unusual names are escaped for safe terminal display.

Socket tables adapt to the terminal width on Windows, macOS, and Linux. When a
full table would not fit, each socket is shown as a stacked record with wrapped
values. Endpoints, PIDs, and process names remain complete. Watch checks the
width again for each output, so later snapshots adapt after a resize. Redirected
text keeps the full table; JSON formatting is unchanged.

Colors are automatic in supported Windows, macOS, and Linux terminals:

- Green: success, listening/bound sockets, and added sockets.
- Cyan: headings, information, and connected sockets.
- Yellow: warnings, closing sockets, and updated sockets.
- Red: errors and removed sockets. Labels also describe every status without color.

| Global option | Meaning |
| --- | --- |
| --color auto | Detect color support independently for standard output and standard error; the default |
| --color always | Force ANSI colors, including redirected text |
| --color never | Disable colors |

```sh
socklens list --listening
socklens list --color never
socklens --color always inspect 8080
```

The --color option can appear before the command or among its options. Put it
before PORT for inspect and wait, like other options. Set NO_COLOR to a nonempty
value or TERM=dumb to disable automatic colors. Explicit --color always overrides
these environment settings.

Pipes and files receive plain text by default. JSON output and its diagnostics
never contain colors, including when --color always is supplied. Version output
also remains plain. Text layouts are intended for people; use --json for scripts.

## Commands

| Command | Purpose |
| --- | --- |
| help | Show the command overview |
| version | Show the installed version |
| check | Check whether socket discovery works |
| update-check | Check GitHub for a newer stable release |
| update | Check and install a newer stable release |
| updates | Enable, disable, or view daily update notices |
| list | List local TCP/UDP sockets |
| inspect | Inspect a local port |
| audit | Explain local listener bindings and relevant firewall rule candidates |
| probe | Test TCP reachability to one explicit host and selected ports |
| watch | Monitor sockets or changes |
| wait | Wait for matching sockets to appear or disappear |
| terminate | Preview or confirm process termination |

### help

```sh
socklens help
socklens --help
socklens -h
```

Running socklens without a command also shows help.

For command-specific options:

```sh
socklens list --help
socklens inspect --help
socklens audit --help
socklens probe --help
socklens watch --help
socklens wait --help
socklens terminate --help
socklens update-check --help
socklens update --help
socklens updates --help
```

These ten commands also accept -h.

### version

```sh
socklens version
socklens --version
```

Shows only the installed version. Takes no additional arguments.

### check

```sh
socklens check
```

Checks socket discovery and reports the number of observed sockets. Takes no options. Process names and ownership may be unavailable because of permissions.

### update-check

```sh
socklens update-check
socklens update-check --json
socklens update-check --timeout 5s
```

Checks the latest published stable GitHub release. Source pushes and unpublished previews do not create a newer installable release. It reports whether an update
is available, the installed version, and the release link. It does not install
updates. Development builds without a comparable release version are skipped
by daily checks; the explicit command reports that the version cannot be compared.

| Option | Default | Meaning |
| --- | --- | --- |
| --json | Off | Return one JSON object with current, latest, update_available, release_url, and status |
| --timeout DURATION | 3s | Network deadline, from 100ms to 30s |
| -h, --help | — | Show update-check options |

Manual checks contact GitHub each time. Offline, timeout, and rate-limit errors
return status 1. An available update still returns status 0; use update_available
in JSON for scripts. No socket or process information is sent.

For GitHub notifications, open the [repository](https://github.com/Thuya-Myint/socklens),
select Watch, then Custom, choose Releases, and save. Each user controls their
own GitHub notification and email preferences.

### update

Requires v0.5.0 or newer; older installations must upgrade once using their installer.

```sh
socklens update
socklens update --dry-run
socklens update --timeout 10m
```

Checks GitHub for the newest stable release and upgrades only when a newer
version is available. A current installation is left as it is.

| Option | Default | Meaning |
| --- | --- | --- |
| --dry-run | Off | Check availability and show the update method without installing |
| --status | Off | Show the latest Windows background update result without contacting GitHub |
| --timeout DURATION | 5m | Total update deadline, from 1s to 30m |
| -h, --help | — | Show update options |

Use --status alone. --dry-run can be combined with --timeout.

- **Homebrew:** refreshes Homebrew and upgrades the formula identified by the
  installation receipt, then checks the installed version.
- **Scoop:** refreshes Scoop and its buckets, upgrades SockLens, and verifies the
  version. Older direct-manifest installations need the one-time bucket migration
  described above. If a release is newer than the bucket, the command asks you to
  retry after the installer has been published.
- **Standalone binaries:** downloads the matching macOS, Windows, or Linux
  amd64/arm64 release over HTTPS, checks SHA-256, architecture, and version, then
  replaces the executable. A failed post-install check attempts to restore the
  previous executable. The install directory must be writable.

On macOS and Linux, wait for the success message. On Windows, the command starts
a background installer so the running executable can exit before replacement.
It prints the log location; check completion with:

```powershell
socklens update --status
socklens version
```

On Windows, an initial successful exit means the installer started; --status
reports whether it completed or failed. Close other SockLens processes before
updating if Windows reports that the executable is in use.

No administrator prompt, sudo, shell-profile change, or automatic installation
is added. Package-manager policies and normal file permissions still apply.
Unknown package-manager locations are refused rather than overwritten. Checksums
come from the same GitHub release as the archive; they detect download corruption
but are not a separate publisher signature.

An interrupted standalone update can leave a reported .update-lock directory.
Before removing that empty lock and retrying, confirm no updater is still running.
If a Windows background installer is stopped unexpectedly, retain its log and
use the original installer to recover the installation.

### Windows Scoop update troubleshooting

In v0.5.0 and v0.5.1, update and update --status can fail with
"The system cannot find the path specified" when resolving Scoop's current
junction. Daily update notices are independent of installation; enabling them
does not resolve this error.

Upgrade once through Scoop to receive the fix in v0.5.2 or newer:

```powershell
scoop update
scoop update socklens
socklens version
```

v0.5.2 resolves Windows installation paths through native file handles. After
this one-time upgrade, use socklens update and socklens update --status for
future releases. v0.5.1 contains the help improvements but still has this bug.

### updates

Daily CLI notices are off by default.

```sh
socklens updates --enable
socklens updates --status
socklens updates --disable
```

| Option | Meaning |
| --- | --- |
| --enable | Opt into daily background update checks and cached notices |
| --disable | Stop automatic checks and notices |
| --status | Show the current setting; also the default when no option is supplied |
| -h, --help | Show preference options |

Choose only one of --enable, --disable, and --status. Preferences apply to the
current user across terminals.

When enabled, interactive list, inspect, check, watch, or wait runs can schedule
a background check at most once every 24 hours. A cached update notice appears
on a subsequent interactive run, at most once per version per day. The network
request runs separately so it cannot delay the port command; Windows workers
do not open a console window.

Normal commands remain usable if GitHub is offline or the check fails. No
automatic notice or background check runs for JSON output, redirected output,
help, version, or termination. Notices use standard error and never alter socket
JSON. Nothing is installed or replaced automatically. Audit and probe never schedule these background checks, including when notices are enabled.

### Shared discovery options

| Option | Available on | Meaning |
| --- | --- | --- |
| --port PORT | list, watch, audit | Filter by local port, from 1 to 65535 |
| --protocol tcp or --protocol udp | list, inspect, watch, wait, audit | Filter by protocol; the default includes both |
| --pid PID | list, inspect, watch, wait, audit | Filter by an observed process ID |
| --listening | list, inspect, watch, wait | Select TCP listeners and unconnected/bound UDP sockets |
| --json | list, inspect, watch, wait, audit | Produce JSON output |

Filters combine: a socket must match every supplied filter. inspect and wait take their port as a positional argument; do not also pass --port.

Put options before the final port argument:

```sh
socklens inspect --protocol tcp --json 8080
socklens wait --listening --timeout 10s 8080
```

### list

```sh
socklens list
socklens list --listening
socklens list --port 8080 --protocol tcp
socklens list --pid 12345
socklens list --json
```

Lists sockets in all states by default. Text output includes protocol, local/remote endpoints, state, process IDs, and process names. Unavailable owner information appears as a dash. From v0.7.2, Windows fills missing executable names from a native process snapshot when its preferred full-image lookup is denied. Names are best-effort observations; this does not reveal or verify the executable path.

IPv4 and IPv6 are supported. On Windows, UDP rows are BOUND because connection state is not provided by the system table. On macOS, process attribution is the last recorded PID and may not include every holder of a shared socket. WSL2 shows Linux sockets in that WSL environment.

### inspect

```sh
socklens inspect 8080
socklens inspect --listening --protocol tcp 8080
socklens inspect --pid 12345 --json 8080
```

Shows sockets matching the required local port. Accepts --protocol, --pid, --listening, and --json.

An empty result means no matching socket was observed. Empty JSON output is [].

### audit

Requires v0.6.0 or newer. Explains a snapshot of local TCP listeners and bound or
unconnected UDP sockets, with observed binding facts and configuration review
items. It uses the same native discovery backends as list and inspect.

```sh
socklens audit
socklens audit --port 8080
socklens audit --protocol tcp
socklens audit --pid 12345
socklens audit --protocol tcp --port 5432 --json
socklens audit --port 5432 --peer 192.168.1.20
socklens audit --firewall=false
socklens audit --rule-limit 0
socklens audit --port 8080 --details
socklens audit --details --rule-limit 0
socklens audit --color never
socklens audit --help
```

Replace 12345 with an actual observed PID. All supplied filters combine.

| Option | Default | Meaning |
| --- | --- | --- |
| --port PORT | Any | Filter by local port, from 1 to 65535 |
| --protocol tcp or --protocol udp | Both | Filter by transport protocol |
| --pid PID | Any | Filter by an observed positive process ID |
| --json | Off | Return one versioned JSON report with summary, listeners, findings, and limitations |
| --peer IP | None | Filter firewall candidates for a numeric unicast peer IP; never contacts that peer |
| --firewall=false | Collection enabled | Skip firewall queries; retain socket/process observations and available Windows service metadata |
| --firewall-timeout DURATION | 20s | Total local policy query deadline, from 1s to 2m |
| --rule-limit N | 3 | Maximum rules per listener/direction in the selected text view; 0 removes that limit; --details includes unverified candidates; JSON always includes all |
| --details | Off | Show full evidence, metadata, rule restrictions, raw policy, and program paths; JSON is always complete |
| -h, --help | — | Show audit options and examples |

Audit always selects listeners/bound UDP sockets, so it has no --listening option.
It takes --port rather than a positional PORT. Use list to see connected sockets.

Binding scopes are loopback, wildcard, private, link-local, multicast, and
specific-address. IPv4, IPv6, and IPv4-mapped IPv6 are identified separately.
An IPv6 wildcard does not establish that the socket also accepts IPv4.

Each listener includes available PIDs/process names, metadata visibility, and
findings with a rule ID, information/review level, confidence, evidence, and a
suggested next step. Wildcard bindings produce review items. A small set of
common database/administration ports can produce an additional non-loopback
service review; these service hints are explicitly unverified. Process names are
observations rather than confirmed service fingerprints.

Review means "check whether this configuration is intended," not "a vulnerability
was found." Information is not a safety verdict. External reachability is always
reported as not tested. Firewall rules, routing, NAT, containers, and network
namespaces can change actual reachability. UDP observations do not prove a
responding service; Windows does not expose UDP connection state in its table.

From v0.7.0, audit also reads available local firewall policy. It shows collection
coverage, source errors, interface addresses, profile/default settings, and
conditional inbound/outbound rule candidates with protocol, local/remote ports,
IP ranges, application and other restrictions. Allow and block candidates can
coexist; applicability and precedence remain unresolved. --peer filters numeric
address conditions; it is not a remote connection test. No candidates does not
mean allowed or blocked. An IPv4 peer is not assumed to reach an IPv6 wildcard.

Windows uses built-in PowerShell read-only policy queries, with a COM fallback
when bulk filters are denied. Linux uses available nftables or iptables-save
utilities; missing tools/permissions give explicit unknown coverage. macOS reads
Application Firewall and PF; PF syntax is preserved for review rather than fully
evaluated. Coverage is partial even after successful queries. WSL sees its own
Linux namespace, not the host Windows firewall. Third-party filters and other
layers may change the outcome. No elevation or firewall changes are attempted.

Audit is passive and works offline: no service probes, DNS/CVE lookups, remote
connections, or background update checks. --firewall=false skips policy queries;
--peer requires firewall collection. Authentication, TLS, application health, and
CVEs are not tested. The executable is built with the Go standard library only;
Go is not required to run it. Firewall enrichment uses the local OS tools above.

From v0.7.2, audit also explains the observed incoming transport/binding scope
and outgoing peer visibility. UDP names its local endpoint and explicitly says
when no remote peer is available. TCP listener output points to inspect for
accepted connection rows. --peer is a user-supplied policy question, never a
claim that a remote peer was observed. These summaries do not test communication.

On Windows, native Service Control Manager queries add hosted service names,
display names and states when available, separately from executable names. A
process can host several services; PID association does not identify which one
owns an individual socket. Missing or inaccessible service metadata stays explicit.
No service query is performed on macOS/Linux; native process discovery continues.

The default view shows endpoint/process-related rule evidence. Broad rules,
missing executable-path matches and unresolved package/service/user/security
restrictions stay in --details and JSON. Related evidence still does not establish
permission; other rules and filtering layers may apply. Windows dotted subnet
masks such as 127.0.0.0/255.0.0.0 are recognized for numeric peer filtering.

From v0.7.1, text is concise by default: a short snapshot overview, combined
process/PID details, findings and next steps, and conditional rule summaries.
Profiles and source availability appear once per report; repeated caveats are
consolidated. Unknown restrictions are labeled unresolved, rather than any.
Program basenames are shown in summaries; --details includes their full paths.

Use --details for full evidence, collector/profile/interface metadata, rule IDs,
restrictions, raw policy, and interpretation notes. --rule-limit still applies;
--details --rule-limit 0 shows every candidate in full. JSON retains all the
structured evidence and candidate data regardless of --details or --rule-limit.

Text records adapt to terminal width and use normal color controls. JSON remains
color-free. A completed audit exits 0 even when review items exist; errors exit 1,
and interruption exits 130. Scripts can inspect summary.review_listeners instead
of interpreting review items as a failed command. Empty results are a snapshot,
not proof that a machine is secure.

From v0.6.1, read top to bottom: non-loopback database/administration port hints,
other review items, other specific-address bindings, then loopback entries.
Wildcard bindings come first within the hinted-service group; ties use numeric
port, protocol, and address order. Review findings appear before informational
findings inside each record. Text and JSON share this order. This is a reading
priority, not a vulnerability score; service hints remain unverified.

See the [audit reference](https://github.com/Thuya-Myint/socklens/blob/main/docs/audit.md)
for rule definitions, the JSON schema, service hints, and interpretation examples.

### probe

Requires v0.8.0 or newer. Make bounded TCP connection attempts to one explicit
host and selected ports. This command sends TCP connection traffic and DNS
queries for hostnames; it sends no application payloads.

```sh
socklens probe --host 127.0.0.1 --ports 8080
socklens probe --host 192.168.1.20 --ports 80,443
socklens probe --host localhost --ports 8080 --family ipv4
socklens probe --host ::1 --ports 8080 --json
socklens probe --host example.com --ports 443 --connect-timeout 2s --timeout 10s
```

| Option | Default | Meaning |
| --- | --- | --- |
| --host HOST | Required | One numeric unicast IP or ASCII hostname; no URL, CIDR, or embedded port |
| --ports LIST | Required | Comma-separated integers, 1–65535; at most 64 distinct ports; duplicates are checked once; ranges/service names are unsupported |
| --family any, ipv4, or ipv6 | any | Select which resolved address family to test |
| --connect-timeout DURATION | 3s | Deadline per attempt, from 100ms to 30s; the total deadline can shorten it |
| --timeout DURATION | 15s | Total budget including DNS, active attempts and queued work, from 1s to 5m |
| --concurrency N | 4 | Maximum simultaneous TCP attempts, from 1 to 32 |
| --json | Off | One report, including partial results after interruption or a total deadline |
| -h, --help | — | Show probe options and interpretation |

The global --color controls apply. A hostname is resolved once, then every
selected numeric IP/port pair is checked separately. Results show the address
attempted, outcome, duration, and available local/remote socket endpoints.
Limits are 16 selected addresses and 256 total attempts; oversized plans fail
before any connections instead of silently dropping targets.

| Result | What was established |
| --- | --- |
| connected | A TCP connection completed from this machine at that time |
| refused | TCP connection was explicitly refused by the target or an intermediary |
| timed-out | An attempt did not complete before its deadline; cause is unknown |
| network-error | Another connection error, such as routing or permission failure |
| cancelled | An active attempt was interrupted before its result could be confirmed |
| not-attempted | Queued work was skipped after interruption or the total deadline |

Exit 0 means every planned connection succeeded; 2 means unsuccessful/incomplete
connections or a total deadline; 1 means input, DNS/target planning, or output
failure; 130 means interrupted. Valid-query DNS/planning failures still produce a
JSON report; invalid arguments produce an error before DNS or connections.

A timeout does not identify a firewall or establish a closed port. Connected
does not establish application health, service identity, authentication, TLS,
or vulnerability status. Remote owners/PIDs are not inferred. IPv4 and IPv6
results can differ; use --family or a numeric IP to narrow the question.
Existing discovery/audit commands stay passive and probe schedules no update checks.
See the [probe reference](https://github.com/Thuya-Myint/socklens/blob/main/docs/probe.md)
for the JSON schema, limits, cancellation behavior, and interpretation examples.

### watch

```sh
socklens watch
socklens watch --listening
socklens watch --port 8080 --count 5
socklens watch --protocol tcp
socklens watch --json
```

Takes the first snapshot immediately. Stop with Ctrl+C.

Plain `socklens watch` requires OS events, prints an initial baseline, then emits
only matching additions, removals, and updates. Unchanged notifications stay
quiet. Tracing failures are reported rather than switching to polling.

In an interactive terminal, missing prerequisites are handled during startup:

- Linux offers to install `bpftrace` if missing. It shows the package-manager
  commands and asks for confirmation before running them. Supported managers are
  apt-get, dnf, yum, pacman, zypper, and apk; unsupported systems receive guidance.
- Linux and macOS request administrator access through the system sudo prompt
  when tracing fails for lack of privileges, then restart the same watch.
- Windows requests access through UAC when ETW returns access denied. After
  approval the same watch opens in a new administrator window.

No helper is installed on Windows or macOS. Passwords are handled by the OS, not
SockLens. Declining setup or access stops the watch. JSON runs and redirected or
noninteractive streams never prompt, install, or elevate; prepare the prerequisites
and privileges before running those workflows. Go runtime dependencies remain
standard-library-only; no tracing helper is bundled in the release binaries.

| Platform | Event source | Requirements |
| --- | --- | --- |
| Linux | eBPF tracepoints | Installed `bpftrace` and kernel tracing privileges (usually root) |
| Windows amd64/arm64 | Native ETW Winsock-AFD and Kernel-Network providers | Administrator or sufficient ETW tracing permissions; no helper executable |
| macOS Intel/Apple Silicon | System DTrace TCP state and syscall probes | `/usr/sbin/dtrace`, tracing privileges (usually root), and probes permitted by SIP |

macOS SIP can restrict tracing even as root. SockLens reports those failures
without repeatedly requesting access; it never changes SIP.
Windows uses a uniquely named private ETW session and stops only its own session
on shutdown. ETW buffering can delay snapshots by about a second. Neither backend
periodically enumerates socket tables while waiting for notifications.

Use `--backend poll` to explicitly select polling, or `--backend auto` to opt into
polling fallback when tracing is unavailable. Backend messages go to stderr, preserving JSON stdout. A helper
failure after attachment stops the watch with an error rather than silently
leaving a stale display. A single snapshot (`--count 1`) does not start tracing.

| Option | Default | Meaning |
| --- | --- | --- |
| --backend events/auto/poll | events | Require OS events; auto explicitly permits polling fallback |
| --interval DURATION | 2s | Polling delay; minimum 100ms; unused with events |
| --count N | 0 | Stop after N snapshots (including quiet ones); 0 keeps watching |
| --changes | On | Emit a baseline, then only additions, removals, and updates; --changes=false prints full snapshots |

All shared discovery options are supported.

By default, the first output is the baseline. Later output uses ADDED, REMOVED,
and UPDATED BEFORE/AFTER; unchanged snapshots stay quiet. With --changes=false,
each refresh prints a timestamp and socket table. --count counts snapshots,
including quiet ones. An idle event watch can wait indefinitely before reaching
the count.

With --json, each emitted record is one JSON line containing timestamp, baseline,
added, removed, and updated. Updates include before/after socket details. With
--changes=false, records contain timestamp and sockets, as in earlier releases.

Polling can miss connections that open and close between refreshes.

Linux and macOS events trigger socket-table collection after TCP state transitions
and bind/connect/listen/accept/shutdown/close syscalls. Windows ETW notifications
include socket lifecycle and network activity. Notifications are coalesced.
Close notifications can also concern non-socket descriptors and cause unchanged
snapshots. This is event-triggered snapshot monitoring, not a lossless history:
short-lived sockets, dropped tracing notifications, ownership changes, and UDP
implicit binds or disconnects outside these hooks may be missed. Visibility is
still limited to accessible sockets (and the current network namespace on Linux). Use
polling when periodic reconciliation is needed.

### wait

Observe a local port until a condition matches or the timeout expires, whichever
happens first. The command stays quiet while waiting and stops at the first
matching snapshot.

| Condition | When the command succeeds |
| --- | --- |
| --until present | At least one socket matches the port and every supplied filter |
| --until absent | No socket matches the port and every supplied filter |

With --listening --protocol tcp, present means a TCP listener exists and absent
means no matching TCP listener remains. Think of these as "starts listening" and
"stops listening." The condition is checked immediately: if it already holds,
the command succeeds without waiting for a change.

Wait for a service to start listening:

```sh
socklens wait --until present --listening --protocol tcp --timeout 30s 8080
```

If port 8080 starts listening after 3 seconds, this succeeds after approximately
3 seconds; it does not continue waiting for the full 30 seconds. If no matching
listener is observed before the deadline, it times out.

Wait for a service to stop listening:

```sh
socklens wait --until absent --listening --protocol tcp --timeout 30s 8080
```

If there is already no matching listener, this succeeds immediately. Otherwise,
it keeps checking until the listener disappears or the deadline expires.

Additional examples:

```sh
socklens wait --listening --protocol tcp 8080
socklens wait --pid 12345 --interval 1s --json 8080
socklens wait --listening --timeout 0 8080
```

| Option | Default | Meaning |
| --- | --- | --- |
| --until present or --until absent | present | Succeed when matching sockets exist, or when none exist |
| --timeout DURATION | 30s | Maximum wait; 0 waits until interrupted |
| --interval DURATION | 500ms | Wait between polls; minimum 100ms |

Accepts --protocol, --pid, --listening, and --json. All filters combine. Put every
option before the final PORT argument; wait uses a positional port, not --port.

Success prints a confirmation and any matching sockets. With --json, success is
one object containing timestamp, condition, port, and sockets. Timeout and errors
go to standard error. Exit codes are 0 for success, 1 for an error, 2 for timeout,
and 130 for interruption. Press Ctrl+C to cancel.

wait reads local socket tables rather than connecting to the target port.
Without --listening, sockets in other states also count; for UDP, --listening
selects bound/unconnected sockets. Socket presence does not establish application
health. Absence does not reserve the port or guarantee a later bind will succeed.
Polling can miss brief changes between snapshots.

### terminate

Find the process first:

```sh
socklens list --listening
```

Replace 12345 below with an actual PID from the PIDS column.

Preview without signaling or prompting:

```sh
socklens terminate --pid 12345 --dry-run
```

On Linux or macOS:

```sh
socklens terminate --pid 12345
```

On Windows:

```powershell
socklens terminate --pid 12345 --force
```

| Option | Meaning |
| --- | --- |
| --pid PID | Required process ID |
| --dry-run | Display the target without sending a signal or asking for confirmation |
| --force | Windows only: explicitly permit immediate process termination |
| -h, --help | Show termination options |

Actual termination requires typing the displayed PID again and pressing Enter. A mismatched or incomplete confirmation cancels the operation.

Linux/macOS send SIGTERM. Windows termination is immediate and can interrupt unsaved work; --force does not bypass PID confirmation. It is rejected on Linux/macOS.

Protected system PIDs, SockLens itself, and processes owned by another user are refused. Identity checks prevent signaling a replacement process. If safe signaling is unavailable, termination is refused. A successful request does not guarantee that the process has finished exiting.

## JSON output

list and inspect produce a JSON array. Each socket includes Protocol, Local, Remote, State, Inode, PIDs, and Processes. Each process entry includes PID and Name. Inode is a platform-specific identity value and may be empty.

probe emits one TCP reachability report with schema_version, host, addresses, timing/limits, completion status, summary, optional error, results, and notes. watch emits newline-delimited records; wait emits one result on success. audit emits one report object with schema_version, timestamp, scope, external_reachability, summary, listeners, and notes. Since v0.7.0, audit adds an optional top-level firewall collection object and per-listener firewall context; --firewall=false omits them. These are additive schema version 1 fields. JSON strings escape control characters in process names. Existing discovery JSON schemas are unchanged.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Command succeeded or help was displayed |
| 1 | Invalid input, discovery/audit/update failure, probe DNS/target planning failure, termination refusal, settings error, or output error |
| 2 | wait timed out; probe had unsuccessful/incomplete connections or exhausted its total deadline |
| 130 | wait, audit, probe, or a foreground update was interrupted |

Stopping watch with Ctrl+C exits successfully.

## Planned development

The audit implementation and later security-analysis direction are described in
[ROADMAP.md](https://github.com/Thuya-Myint/socklens/blob/main/ROADMAP.md). Explicit TCP reachability checks are available in probe; protocol/vulnerability analysis remains planned work.
