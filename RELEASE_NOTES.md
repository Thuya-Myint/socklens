# SockLens v0.7.2

Audit now identifies more Windows socket owners, shows hosted service names when
available, and explains communication scope in plain language on every platform.

```sh
socklens list --port 60817
socklens audit --port 60817
socklens audit --port 60817 --firewall=false
socklens audit --port 60817 --details
socklens audit --port 60817 --json
```

Use a currently observed port. For a Windows service endpoint, output can show
AppleMobileDeviceService.exe as the process and Apple Mobile Device Service as
the separately observed hosted service, instead of just an unavailable name.

## Included

- Native Windows Toolhelp executable-name fallback when preferred full-image
  access fails. Discovery JSON fields remain the same; missing names may fill.
- Read-only native Windows SCM service status queries for audit: service name,
  display name, state, PID, own/shared-host type and source. Shared-host service
  metadata does not identify which service owns a particular socket. Permissions
  can omit services; no administrator prompt or service changes are attempted.
- Shared macOS/Windows/Linux inbound TCP/UDP binding scope and outbound visibility.
  Loopback describes local binding scope, not a firewall grant. UDP peers are not
  invented; TCP listener output points to inspect for accepted connection rows.
- Concise rule evidence related to an explicit known local port or observed full
  process path. Broad/identity-unresolved rules stay in --details and JSON.
- Collected Windows package/service/user/security restrictions remain unverified;
  a process basename is never substituted for a full executable-path match.
- Windows dotted subnet masks recognized for numeric peer filtering.
- Additive schema 1 service_lookup/listener services and candidate relevance/
  identity_conditions fields. All candidate data remains available in JSON.

All firewall assessments remain conditional/unknown where the full decision
cannot be established. Profiles, identity, precedence, other layers and external
reachability are not fully evaluated. Related evidence is not permission.
--rule-limit 0 removes the selected view's display limit; use --details
--rule-limit 0 for every candidate. A supplied --peer is labeled requested,
never an observed remote connection. No network probes or automatic elevation.

## Platforms and upgrade

All macOS, Windows and Linux amd64/arm64 binaries and installers are included.
Windows gets OS-specific owner/service enrichment; macOS/Linux retain their
native process discovery and share the new communication/output behavior.
No external Go modules or new runtime utilities were added for owner/service lookup.

On macOS/Linux run socklens update, then socklens version.
On Windows v0.5.2 or newer:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then verify socklens version reports v0.7.2.
Windows Scoop v0.5.0/v0.5.1 needs scoop update followed by scoop update socklens
once. v0.4.0 and older need an initial package-manager/binary-installer upgrade.

## Validation

Tests cover missing-name fallback, native Windows process/service APIs, 32/64-bit
service table bounds and UTF-16 strings, service-to-PID association, shared hosts,
cancellation, escaping, protocol/binding summaries, dotted masks, candidate
relevance and complete detail/JSON visibility. All-platform native/race/security,
packaging and installer smoke checks gate release and promotion. The real Windows
PID 8100 endpoint was checked for the recovered executable and service names.

Termination identity/permissions remain unchanged. Service metadata is a
best-effort observation, not verified executable identity or a vulnerability
finding. Remote reachability and full policy evaluation remain future work.

Licensed under MIT.
