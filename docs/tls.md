# TLS certificate checks

`socklens tls --host HOST` explicitly connects to one host on TCP port 443 and
performs a TLS handshake with normal certificate verification enabled.

```sh
socklens tls --host example.com
socklens tls --host 192.168.1.20 --port 8443 --server-name service.example.com
socklens tls --host service.example.com --ca-file company-ca.pem --json
```

| Option | Default | Meaning |
| --- | --- | --- |
| --host HOST | required | One ASCII hostname or unicast IP; no URL, CIDR, or port list |
| --port PORT | 443 | One TCP port, 1–65535 |
| --server-name NAME | host | Name verified against the certificate; DNS names also set SNI |
| --family any/ipv4/ipv6 | any | Select resolved address family |
| --handshake-timeout DURATION | 3s | Per-address TCP connection plus TLS handshake; 100ms–30s |
| --timeout DURATION | 15s | Total deadline including DNS and all attempts; 1s–5m |
| --ca-file FILE | unset | Regular PEM file, at most 4 MiB, adding trust for this command only |
| --json | off | One versioned report, including partial results after interruption |

A hostname is resolved once, with at most 16 distinct selected addresses. Every
selected numeric address is attempted sequentially until the total deadline or
cancellation. More addresses reject the plan before connections start. DNS
answers do not silently change between attempts. The connection target and
certificate verification name are separate when --server-name is supplied.
An IP verification name requires a matching IP SAN; it does not send DNS SNI.

System trust remains enabled. --ca-file adds the supplied certificates without
changing the operating system's trust store or weakening verification. There is
no insecure/skip-verification mode. Certificates seen in verification errors are
reported as unverified evidence, never as trusted identity.

Results distinguish verified, certificate-error, tls-error, network-error,
timed-out, cancelled, and not-attempted. Certificate failures include stable
codes such as expired, not-yet-valid, hostname-mismatch, unknown-authority, and
certificate-invalid, with a next step. A verified leaf expiring within 30 days
gets expires-soon; this is a renewal warning, not a verification failure.
An expired intermediate may cause failure even if the reported leaf is current.
The verification error retains that detail.

Only successful handshakes report the negotiated TLS version and cipher suite.
TLS 1.2 is the minimum offered version; a failed handshake does not establish
which older versions the server supports. Certificate evidence includes subject,
issuer, DNS/IP SANs, validity timestamps, SHA-256 fingerprint, and verified status.
Text output escapes remote strings and adapts to terminal width.

JSON schema_version is 1. Reports contain timestamp, host, port, server_name,
family, timeout_ms, handshake_timeout_ms, duration_ms, status, optional error,
results, and notes. Each result includes endpoint, status, duration_ms, optional
tls_version/cipher_suite/certificate, error, warning, code, and next_step.
Not-attempted results remain in partial reports. Top-level completed means the
plan finished; inspect results or the exit code for verification failures.

Exit codes:

- 0: all selected endpoints verified, with no expiry warnings.
- 1: invalid input, CA loading, DNS/target planning, or output failure.
- 2: failed or incomplete checks, total timeout, or renewal warnings.
- 130: interrupted; partial report retained.

No HTTP/application request, STARTTLS negotiation, client certificate, redirect,
protocol/cipher enumeration, revocation check, CVE matching, or background update
check is performed. A verified handshake establishes certificate verification
and one negotiated connection at that time, not application health or a complete
security assessment. OS trust-store behavior can differ across platforms.
