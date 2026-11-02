# SockLens v0.6.0

Explain local listener exposure with the new passive audit command:

```sh
socklens audit
socklens audit --port 8080
socklens audit --protocol tcp --json
```

All supported macOS, Windows, and Linux amd64/arm64 binaries are included.
Go, a compiler, and a source checkout are not required.

## Local listener audit

Audit reads one snapshot of TCP listeners and bound/unconnected UDP sockets using
the existing native discovery backends. Filter by --port, --protocol, and --pid;
all supplied filters combine. Audit always selects these listener/bound rows,
so there is no --listening option or positional port argument.

It classifies loopback, wildcard, private, link-local, multicast, and other
specific-address bindings, including IPv4, IPv6, and IPv4-mapped IPv6. Each record
contains available process details, evidence, confidence, and a suggested next step.
Missing process details stay explicit. IPv6 wildcard observations do not imply
that IPv4 connections are also accepted.

Wildcard bindings produce review items. A small set of common database and
administration port conventions provides unverified service hints; non-loopback
matches add a heuristic review finding. A port convention is not a confirmed
service identity, authentication weakness, encryption finding, or vulnerability.

Review means to check whether a configuration is intended. Information is not a
safety verdict. External reachability is not tested. Firewalls, routing, NAT,
containers, and network namespaces can change actual reachability. Windows UDP
tables do not expose connection state; UDP observations do not prove readiness.

Audit opens no connections, probes no services, performs no DNS or CVE lookup,
and starts no background update checks even when daily notices are enabled.
It does not change services or firewall rules. The runtime remains standard-library-only.

## Output and scripting

```sh
socklens audit --pid 12345
socklens audit --color never
socklens audit --json
socklens audit --help
```

Replace 12345 with an actual observed PID. Text records adapt to terminal width,
use normal color controls, and escape untrusted process names. JSON and its
diagnostics remain color-free.

JSON returns one report with schema_version 1, timestamp, local scope, external
reachability marked not-tested, summary, listeners, and notes. Every finding has
a rule ID, information/review level, observed/heuristic confidence, evidence, and
recommendation. Completed audits exit 0 even when review items exist; errors exit
1, interruption exits 130. Scripts can inspect summary.review_listeners.
Existing discovery JSON shapes remain unchanged.

See README.md and the audit reference linked there for all options and rule details.

## Windows TCP discovery correction

Windows now queries TCP listeners and connections separately. The listener-only
query establishes LISTEN membership on systems that return state zero in the
combined table. Connection rows retain their own state. This also improves TCP
listener filtering in list, inspect, watch, and wait. Regression tests cover the
zero-state case and real listeners/connections for both IP families.

The v0.5.2 Scoop junction update fix remains included.

## Upgrade

With v0.5.2 or newer on Windows:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then verify:

```powershell
socklens version
socklens audit --help
```

Windows Scoop installations on v0.5.0/v0.5.1 must use Scoop once because their installed updater
contains the junction bug:

```powershell
scoop update
scoop update socklens
```

On macOS and Linux, use socklens update, then socklens version. v0.4.0 and older
need one initial upgrade through the existing installer to receive update support.
Linux/WSL2 users can rerun the binary installer. The expected version is v0.6.0.

## Validation and scope

Rule tests cover scope boundaries, mapped/zone addresses, confidence, missing
metadata, deterministic reports, and snapshot ownership. CLI tests cover combined
filters, errors, cancellation, output failures, escaping, narrow layouts, JSON,
and live local TCP/UDP sockets. Full native macOS/Windows/Linux, Linux race/security,
packaging, and fresh installer checks gate publication and installer promotion.

Remote targets, TLS/authentication/CVE analysis, shell completion, and additional
interrupted-update recovery remain planned work in ROADMAP.md.

Licensed under MIT.
