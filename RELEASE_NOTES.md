# SockLens v0.7.1

Audit output is easier to scan, with concise text by default and full detail
available on demand.

```sh
socklens audit
socklens audit --port 8080
socklens audit --port 8080 --details
socklens audit --details --rule-limit 0
socklens audit --port 8080 --json
```

## Included

- A short overview with source coverage, available network profiles, and default
  policy configuration. Source failures remain visible, including fallback use.
- Combined process/PID details and readable bindings, findings and next steps.
- Conditional inbound/outbound rule counts and short rule summaries showing
  protocol, ports, IP restrictions and application/profile/interface scope.
- --details for full evidence, source/profile/interface metadata, rule IDs,
  restrictions, raw policy, complete program paths and interpretation notes.
- Consolidated explanations instead of repeating them for every candidate.
- Unresolved restrictions stay explicit. Allow/block candidates remain conditional;
  no candidates does not imply permission or denial. Profile values are observed
  configuration, not a complete effective policy or remote reachability result.

The text rule limit still defaults to 3 per direction per listener. Use
--details --rule-limit 0 to display all candidates in full. JSON always retains
all collected candidate data and is unaffected by either text display option.
No JSON schema, collector, candidate matching, finding level, reading priority,
update preference or process termination behavior changes are introduced.

Both text views wrap to terminal width, follow normal color settings, and escape
untrusted strings. The Go implementation remains standard-library-only.

## Upgrade

On macOS/Linux, run socklens update, then socklens version.
On Windows v0.5.2 or newer:

```powershell
socklens update
socklens update --status
```

Wait for completed status, then run socklens version. Expected: v0.7.1.
Windows Scoop v0.5.0/v0.5.1 needs scoop update followed by scoop update socklens
once to bypass its installed updater's junction bug. v0.4.0 and older need one
initial package-manager/binary-installer upgrade.

All macOS, Windows, and Linux amd64/arm64 binaries are included. Homebrew, Scoop,
and Linux installer references update after required checks. No Go installation
or source checkout is needed.

## Validation

Regressions cover concise versus detailed evidence visibility, unchanged JSON,
conditional labels, unknown restrictions, program-path summaries, rule limits,
output errors, escaping and narrow terminal layouts. Native platform tests,
Linux race/security checks, packaging and installer smoke tests gate release.
A real Windows listener preview also verified both views and local policy context.

Firewall interpretation remains partial; macOS PF fields are preserved as raw
policy, Linux queries depend on available tools/permissions, and remote
reachability and service/TLS/vulnerability testing remain future work.

Licensed under MIT.
