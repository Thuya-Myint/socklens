# SockLens v0.10.0

Check TLS certificates and negotiated connection details for one explicitly
selected host and port on Windows, macOS, and Linux.

```sh
socklens tls --host example.com
socklens tls --host 192.168.1.20 --port 8443 --server-name service.example.com
socklens tls --host service.example.com --ca-file company-ca.pem --json
```

## Included

- Required --host and optional --port (443 by default), --server-name, and
  --family any/ipv4/ipv6. Hostnames are resolved once; numeric endpoints are pinned
  for every selected address, with a maximum of 16 distinct addresses.
- Normal certificate hostname and chain verification, TLS 1.2 or newer, validity
  dates, subject/issuer, DNS/IP SANs, and SHA-256 certificate fingerprints.
- Stable certificate findings: expired, not-yet-valid, hostname-mismatch,
  unknown-authority, certificate-invalid, and expires-soon, with next steps.
- A renewal warning for verified leaf certificates expiring within 30 days.
- Negotiated TLS version and cipher suite after successful verification.
- Explicit --ca-file trust additions for private PKI, scoped to this command.
  System trust remains enabled, with no operating-system trust-store changes.
- Per-address TCP/TLS deadline of 3 seconds and total deadline of 15 seconds,
  configurable with --handshake-timeout and --timeout.
- Responsive escaped text, JSON schema_version 1, cancellation/partial reports,
  and connection cleanup. No admin permissions or additional helper are needed.

Verification is never disabled. Certificates attached to verification failures
are labelled as unverified evidence. --server-name separates the connection
address from the certificate verification name and DNS SNI.

## Scope and exit codes

This is one verified TLS handshake per selected address. It sends no HTTP or
other application request, follows no redirects, and does not perform STARTTLS,
client-certificate authentication, revocation checks, protocol/cipher enumeration,
or CVE matching. A successful handshake does not establish application health or
a complete security assessment. OS trust behavior can differ across platforms.

Exit 0 means every selected endpoint verified without warnings. Exit 1 indicates
input, CA loading, DNS/target planning, or output failure. Exit 2 covers failed or
incomplete checks, total timeout, and expiry warnings. Exit 130 means interrupted;
the report retains partial results and not-attempted targets.

Existing discovery commands stay passive. The TLS command does not schedule
background update checks. Watch behavior remains as shipped in v0.9.1.

## Distribution and upgrade

All six Linux, Windows, and macOS amd64/arm64 archives are provided, with
SHA256SUMS, a Homebrew formula, and Scoop metadata. Fresh installer checks gate
promotion. No Go installation is needed.

Run socklens update, then verify socklens version reports v0.10.0. On Windows,
use socklens update --status to wait for completion. Homebrew/Scoop users can
also use their package manager's upgrade command.

See docs/tls.md for options, JSON fields, finding semantics, and trust behavior.

## Validation

Owned loopback fixtures cover TLS 1.2/1.3, valid/expired/not-yet-valid certificates,
hostname mismatch, unknown trust, private CA additions, expiry warnings, DNS
pinning and target limits, plaintext/legacy TLS rejection, deadlines, cancellation,
connection cleanup, no application requests, output errors, and escaping.
All-platform CI gates publication, including Linux race/security checks, native
Windows/macOS/Linux tests, verified packaging, and fresh installation checks.

Licensed under MIT.
