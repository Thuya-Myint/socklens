# SockLens v0.7.0

Audit now adds read-only local firewall context and peer-specific candidate rule
filtering across Windows, macOS, and Linux/WSL.

```sh
socklens audit
socklens audit --port 5432 --peer 192.168.1.20
socklens audit --protocol udp --json
socklens audit --rule-limit 0
socklens audit --firewall=false
```

## Included

- Available firewall source status, profile/default settings, interface addresses,
  and conditional inbound/outbound allow/block rule candidates.
- Numeric peer IP filtering, protocol/port/address ranges, executable-path
  restrictions, and explicit unknown conditions and binding-family compatibility.
- Windows NetSecurity ActiveStore query and ordinary-user COM fallback.
- Linux nftables JSON with iptables filter-table fallback; missing utilities and
  permissions are reported. WSL coverage is for its own Linux namespace.
- macOS Application Firewall settings/app rules and PF status/raw anchor rules.
- No probes, elevation, firewall changes, or background update checks.
- Partial/unknown coverage even when a source succeeds; all assessments remain
  unknown and candidates conditional. No matching rules is not an allow/block
  verdict. Service hints and the existing audit reading order remain unchanged.

PF expressions, rule traversal/precedence, stateful replies, actual profile and
interface applicability, service/package identity, third-party filters and other
layers are not fully evaluated. This release is policy context, not a complete
packet-filter simulator or remote reachability test.

## Options and output

--peer accepts one numeric unicast IP and requires firewall collection.
--firewall=false retains socket-only audit and omits optional policy JSON fields.
--firewall-timeout defaults to 20s (1s–2m). --rule-limit defaults to 3 candidates
per direction per listener in text; 0 shows all. JSON always contains all collected
candidates. Policy failures/deadlines retain the socket snapshot with a reason;
Ctrl+C cancels the command. Text remains responsive and safely escaped.

The report keeps schema_version 1 with additive optional firewall collection and
per-listener fields. Existing discovery JSON formats remain unchanged. See the
README and docs/firewall.md for every condition, option and interpretation limit.

## Upgrade

On macOS/Linux, run socklens update, then socklens version.
On Windows v0.5.2 or newer:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then run socklens version. Expected: v0.7.0.
Windows Scoop v0.5.0/v0.5.1 needs scoop update followed by scoop update socklens
once to bypass its installed updater's junction bug. v0.4.0 and older need one
initial package-manager/binary-installer upgrade.

All macOS, Windows, and Linux amd64/arm64 binaries are included; Homebrew, Scoop,
and Linux installer references update after required checks. No Go installation
or source checkout is needed. Optional firewall enrichment uses local OS policy
utilities; no additional Go modules or third-party SDKs were added.

## Validation

Peer CIDR/ranges, protocol/port direction, IPv4/IPv6/mapped compatibility,
application identity uncertainty, overlapping allow/block candidates, parser
failures, source permissions, deadlines, cancellation, JSON and narrow/escaped
text are covered by regressions. Native read-only collectors, all-platform tests,
Linux race/security checks, packaging and installer smoke tests gate release.

Remote reachability, service/TLS/authentication/CVE analysis and complete firewall
policy evaluation remain future work. Licensed under MIT.
