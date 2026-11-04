# SockLens v0.8.0

Test TCP reachability to an explicit host and selected ports with the new probe
command, on Windows, macOS and Linux/WSL.

```sh
socklens probe --host 127.0.0.1 --ports 8080
socklens probe --host 192.168.1.20 --ports 80,443
socklens probe --host localhost --ports 8080 --family ipv4
socklens probe --host ::1 --ports 8080 --json
```

## Included

- Required --host and --ports, optional --family any/ipv4/ipv6.
- One-time hostname resolution and numeric TCP4/TCP6 attempts per selected IP/port;
  actual attempted addresses and available local/remote socket endpoints retained.
- --connect-timeout (3s default, 100ms–30s), --timeout (15s total, 1s–5m), and
  --concurrency (4 default, 1–32). Total timeout includes DNS and queued work.
- Bounds of 64 distinct ports, 16 selected addresses and 256 total attempts;
  oversized/invalid requests fail before connections, without silent truncation.
- Connected, refused, timed-out, network-error, cancelled and not-attempted results.
- Immediate connection close and no application payloads, banners, TLS, UDP,
  automatic retries, remote PID guesses or background update checks.
- Partial reports after cancellation or total deadline; responsive/color-controlled
  text and one complete JSON report with schema version 1.
- Exit 0 all connected; 2 unsuccessful/incomplete connections or total deadline;
  1 input/DNS/planning/output failure; 130 interrupted.

A successful TCP connection establishes reachability from this client at that
time. Application health, service identity and security are not established.
Refusal may originate at target or intermediary. Timeouts do not identify a
firewall or establish a closed port. Existing local/audit commands stay passive.

All supported macOS, Windows and Linux amd64/arm64 binaries are included, with
installer promotion after required checks. No Go installation or source checkout
is needed and no external Go modules were added.

## Upgrade

On macOS/Linux run socklens update, then socklens version.
On Windows v0.5.2 or newer:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then verify socklens version reports v0.8.0 and run
socklens probe --help. Windows Scoop v0.5.0/v0.5.1 needs scoop update followed by
scoop update socklens once. v0.4.0 and older need an initial installer upgrade.

See README and docs/probe.md for every option, JSON field, limit and example.

## Validation

Tests cover input/target limits, one-time DNS/numeric pinning, mixed families,
concurrency, refusal/error classification, cancellation/queued work, deadlines,
connection closure and no payloads, output failures, escaping/layout/colors,
JSON and exit codes. Real local IPv4/IPv6 listeners and refused ports are included
in native Windows/macOS/Linux CI. Linux race/security, packaging and fresh
installer checks gate publication and promotion.

Licensed under MIT.
