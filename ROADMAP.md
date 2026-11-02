# SockLens roadmap

Status: proposal recorded on 2026-10-27. The v0.5.x line is the baseline; v0.5.1 clarifies help and documentation.
This document describes future work, not completed commands or a release promise.
The owner wants SockLens to grow from local port inspection into service exposure
and security analysis, eventually including explicitly selected remote hosts.

## Current baseline: v0.5.x

Local TCP/UDP IPv4/IPv6 discovery, PID/process information where available,
combined filters, inspect/watch/wait, confirmed same-user process termination,
JSON, adaptive colored output, direct updates, opt-in daily notices, six binary
platforms, consumer installers, and release CI are implemented. Native platform,
security, packaging, and fresh-install checks passed for the published release.

wait observes filtered local socket tables. present means at least one matching
socket exists; absent means none exists. It stops when the condition matches or
the timeout expires, and succeeds immediately if the initial snapshot matches.
The README contains full examples, options, and exit codes.

## Proposed v0.6.0: local exposure analysis and update reliability

The primary user-facing addition would be a passive local audit. Existing list,
inspect, watch, and wait behavior should remain compatible.

### Proposed audit command

These examples illustrate the intended interface; it is not implemented:

```sh
socklens audit
socklens audit --port 8080
socklens audit --protocol tcp --json
```

The audit would reuse local discovery and filters, then classify listener binding
scope: loopback, wildcard, private-network, link-local, or another address scope.
IPv4 and IPv6 require explicit tests. An IPv6 wildcard must not automatically be
reported as also accepting IPv4.

Initial review rules could flag all-interface listeners and non-loopback
listeners associated with potentially sensitive services. Port-based service
names are hints and must be labeled as unverified. A broad binding can be
intentional; its presence alone does not establish a vulnerability.

Each result should include the endpoint/protocol, observed PID/process where
available, rule identifier, direct evidence, a separate confidence label,
why the configuration deserves review, and a practical recommendation.
Findings should distinguish informational observations from review items;
severity must have a stated basis rather than treating every open port as unsafe.
Unknown process ownership should remain unknown rather than implying safety.

Example interpretation: an IPv4 listener at 0.0.0.0:5432 is bound to all local
IPv4 interfaces. 5432 can suggest a service to investigate; it does not prove
which service is running, its authentication settings, or a vulnerability.
The recommendation could be to confirm whether broad binding is intended and
review the application's binding and network-access controls.

A local bind address does not prove reachability from the Internet. Firewalls,
routing, NAT, containers, and IPv6 policy affect reachability. Private addresses
and loopback binding are scope observations, not complete security verdicts.
The first audit should work from local observations without initiating remote
probes. It should never change firewall rules or terminate services automatically.

### Update reliability work

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

## Possible v0.7.0: explicitly selected remote TCP checks

A later command could connect to user-selected hosts and TCP ports, reporting
what is reachable from the machine running SockLens. The first version should
accept explicit targets/ports and provide bounded concurrency, connection and
total deadlines, cancellation, and JSON. Output must retain the actual resolved
address used; DNS results and IPv4/IPv6 scope need explicit handling.

A successful connection supports "reachable from this client." A timeout alone
does not establish "closed" or identify a firewall. Remote process IDs and owners
are not available from an ordinary TCP connection. Existing local commands should
keep their current meaning; active network work belongs to an explicit command.
Remote checks are intended for systems the user owns or has permission to assess.

## Later: service and protocol security checks

Add explicit, limited protocol checks once target handling is reliable. TLS
checks could inspect certificate expiry, hostname/chain validation, and negotiated
protocol details. A single successful handshake does not enumerate every supported
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
