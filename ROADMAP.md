# SockLens roadmap

Status: the v0.10.0 phase adds explicit TLS certificate checks, implemented and
verified by all-platform CI. v0.9.1 provides event-triggered watch and guided
setup. The v0.5.x line provides discovery, process controls, installers,
and updates. Sections distinguish implemented work from proposals; this is not a
release promise.
The owner wants SockLens to grow from local port inspection into service exposure
and security analysis, eventually including explicitly selected remote hosts.

## v0.5.x foundation

Local TCP/UDP IPv4/IPv6 discovery, PID/process information where available,
combined filters, inspect/watch/wait, confirmed same-user process termination,
JSON, adaptive colored output, direct updates, opt-in daily notices, six binary
platforms, consumer installers, and release CI are implemented. Native platform,
security, packaging, and fresh-install checks passed for the published release.

wait observes filtered local socket tables. present means at least one matching
socket exists; absent means none exists. It stops when the condition matches or
the timeout expires, and succeeds immediately if the initial snapshot matches.
The README contains full examples, options, and exit codes.

## v0.6.0: local exposure audit

Implemented commands:

```sh
socklens audit
socklens audit --port 8080
socklens audit --protocol tcp --json
socklens audit --pid 12345
```

Audit reuses native local socket discovery and shared port/protocol/PID filtering.
It classifies loopback, wildcard, private, link-local, multicast, and specific
address bindings for IPv4, IPv6, and mapped IPv4. Every listener has structured
findings with rule IDs, evidence, observed/heuristic confidence, and next steps.
A small common-port hint set is explicitly unverified. Reports distinguish
information from review; neither is a complete security verdict.

The implementation is passive and offline. External reachability, IPv4 acceptance
by IPv6 wildcard sockets, authentication, TLS, and CVEs are not inferred from
open ports. No background update checks run during audit. Missing process details
remain explicit. Windows uses separate listener/connection tables so zero states
in the combined table do not hide listeners.

A pure shared analysis package consumes socket observations independently of
CLI rendering. JSON has schema_version 1; text uses responsive records and normal
color controls. The audit reference linked from README documents every option,
rule, JSON field, and limitation. Existing discovery JSON schemas are preserved.

Tests cover address boundaries, confidence and hint semantics, missing metadata,
combined filters, output errors, cancellation, escaping, narrow layouts, and
live TCP/UDP listeners with connected sockets excluded. All-platform CI and
fresh installation checks gate publication and installer promotion.

### Follow-up update reliability work (not included in the initial audit)

- Test existing installs upgrading through real isolated Homebrew taps and Scoop
  buckets in CI, in addition to the current fresh-install and delegation tests.
- Exercise interruption during download, staging, and replacement; preserve the
  installed executable or provide a verified recovery path and retained backup.
- Recognize abandoned jobs and locks only when it is safe; do not unlock an
  installation while its updater is still active.
- Make offline, permission, locked-file, and stale-catalog outcomes actionable.
- Ensure Windows status distinguishes active work, completion, failure, and an
  unexpectedly stopped worker. Preserve daily-notice preferences across upgrades.

### Architecture and completion criteria

Keep runtime Go dependencies in the standard library. Platform-specific discovery
remains behind internal/platform; shared pure classification/rules should consume
normalized observations and produce structured findings, independently of CLI
formatting. A future active-probe layer should be separate from passive discovery.

Test rules with IPv4/IPv6 fixtures and real local TCP/UDP listeners on Windows,
macOS, and Linux. Verify JSON structure, missing metadata, combined filters,
clear scope labels, offline operation of passive analysis, and no implied external
reachability. Existing termination and discovery guarantees must still hold.
Publish only after required CI passes and the owner authorizes the release.

Shell completion for Zsh, Bash, Fish, and PowerShell remains a usability candidate.
It can follow this release if including it would delay the exposure/reliability
work. Its proposed interface is socklens completion SHELL.

## v0.8.0: explicitly selected TCP reachability checks

Implemented: probe --host HOST --ports LIST, one-time DNS and numeric address
pinning, IPv4/IPv6 family selection, bounded concurrency, per-attempt/total
deadlines, cancellation/partial reports, JSON and explicit result classifications.
See docs/probe.md. Tests use controlled dependencies and owned loopback fixtures.

Connected establishes TCP reachability from this client at that time. Refusal,
timeout and other errors retain their actual limits; no service health, remote
process identity, authentication/TLS or vulnerability verdict is inferred.
Existing local commands keep their meaning and do not start network probes.

## Current phase: explicit TLS certificate checks

Implemented: `socklens tls --host HOST [--port PORT]`, with one-time DNS,
numeric address pinning, per-address TCP/TLS deadlines, a total deadline,
hostname/chain verification, validity dates, negotiated TLS version/cipher,
SHA-256 evidence, expiry warnings, text/JSON output, and explicit private CA trust
additions. Verification is never disabled. See docs/tls.md for scope and limits.

Loopback tests cover valid TLS 1.2/1.3, certificate failures, warning semantics,
connection cleanup, timeout/cancellation, and no application requests. Native
publication checks remain required before release.

## Later: broader service and protocol security checks

Expand explicit, limited protocol checks incrementally. The initial TLS check
inspects certificate expiry, hostname/chain validation, and negotiated protocol
details. A single successful handshake does not enumerate every supported
protocol or cipher; each finding must describe exactly what was tested.

Service identification requires evidence beyond a port number. Future known-
vulnerability matching needs dependable product/version information, relevant
configuration/patch context, data freshness and provenance, and an uncertainty
model. A banner or port number alone must not produce a confirmed CVE finding.
Remote UDP discovery would need its own semantics; silence must remain unknown.

Every security finding should explain the evidence, confidence, and remediation.
The project should grow its checks incrementally rather than promise a complete
assessment of every service from its open ports alone.

## References

- [NIST SP 800-115, Technical Guide to Information Security Testing and Assessment](https://csrc.nist.gov/pubs/sp/800/115/final): distinguishes port/service discovery and vulnerability assessment and discusses identification limitations.
- [OWASP WSTG: Weak Transport Layer Security](https://owasp.github.io/www-project-web-security-testing-guide/latest/4-Web_Application_Security_Testing/09-Weak_Cryptography/01-Weak_Transport_Layer_Security): protocol and certificate checks are separate technical observations.

## v0.7.0: initial read-only firewall context

Implemented: available Windows/macOS/Linux policy sources, conditional inbound/
outbound candidates, numeric peer filtering, coverage/permission evidence,
profile/default settings, local addresses, query deadlines and optional socket-only
mode. See docs/firewall.md. This is partial policy context; complete precedence,
stateful traffic, custom-chain/PF traversal, service/package identity and other
filtering providers remain unresolved. Explicit TCP reachability checks are now available in probe.
