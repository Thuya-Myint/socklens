# SockLens v0.6.1

Read the audit findings that need attention first, rather than scanning an
alphabetical list of socket addresses.

## Audit reading order

1. Non-loopback listeners on a hinted database/administration service port.
2. Other review items, currently wildcard bindings.
3. Other non-loopback specific-address bindings.
4. Loopback bindings.

Wildcard bindings lead within the first group. Ties use numeric port, protocol,
and numeric address, followed by deterministic socket/owner metadata. Port 80
therefore appears before 443. Review findings also precede information within a
record. Both text and JSON use the same order; the report schema remains version 1.

The text overview and command help explain the order. This is a reading priority,
not a vulnerability score. Port hints remain unverified; a wildcard binding does
not establish remote reachability, and a loopback binding is not a safety verdict.
Finding levels, summary counts, filters, and exit codes are unchanged.

## Try it

```sh
socklens audit
socklens audit --protocol udp
socklens audit --json
socklens audit --help
```

The v0.6.0 passive audit and Windows TCP discovery correction, and v0.5.2 Scoop
junction update fix, remain included. Firewall rule collection, peer-specific
policy analysis, and remote reachability tests are future work.

## Upgrade

On macOS and Linux, run socklens update, then socklens version.
On Windows v0.5.2 or newer:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then run socklens version. Expected version: v0.6.1.
Windows Scoop users still on v0.5.0/v0.5.1 need scoop update followed by
scoop update socklens once to bypass their installed updater's junction bug.
v0.4.0 and older need one initial package-manager or binary-installer upgrade.

## Platforms and validation

All macOS, Windows, and Linux amd64/arm64 binaries are included, with Homebrew,
Scoop, and Linux installer promotion after successful checks. No Go installation
or source checkout is needed.

Mixed-protocol/binding ordering tests cover numeric ports and addresses,
loopback hints, input-order independence, and review-before-information findings.
CLI regressions check text and JSON order. Full native platform tests,
Linux race/security checks, packaging, and installer smoke tests gate release.

Licensed under MIT.
