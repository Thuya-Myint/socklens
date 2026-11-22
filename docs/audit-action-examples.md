# Actionable audit output

Development change: clearer default text for local listener audits. The following
are selected lines from representative fixtures, not observations of your machine.
Other process, finding and conditional-rule fields remain available.

The default view adds:

- **Exposure:** what the binding itself establishes, with its limits.
- **Unknown:** missing owner metadata, absent policy collection, or an unresolved
  traffic decision and the observed policy coverage.
- **Check:** at most one passive, copyable next command for that port/protocol.

## 1. Loopback listener

Previously, the binding and general recommendation explained that the listener
used loopback, but offered no focused command for checking it:

```text
Binding: loopback / IPv4 | LISTEN
Next: Keep loopback for local clients; local processes can still connect.
```

Selected lines in the improved default view:

```text
INFO TCP 127.0.0.1:8080
  Exposure: Loopback-bound; direct socket access is local. Proxies or forwarding are not evaluated.
  Unknown: firewall not collected
  Check: socklens inspect --protocol tcp 8080
```

Inspect the observed process and connection rows. A loopback binding does not
certify application security or rule out exposure through a separate proxy or
forwarder. Here, firewall collection was explicitly disabled for the fixture.

## 2. Wildcard listener

Previously:

```text
Binding: all local IPv4 interfaces | LISTEN
Next: Verify service identity, intended clients, authentication and encryption. Confirm access beyond localhost is intended; review binding and network controls.
```

Selected lines in the improved view for a known process on port 5432:

```text
REVIEW TCP 0.0.0.0:5432
  Exposure: All interfaces in this address family; remote reachability is not established.
  Unknown: firewall not collected
  Check: socklens inspect --protocol tcp 5432
```

Confirm the actual owner and intended clients, then review that application's
binding/configuration. The PostgreSQL convention for port 5432 is an unverified
hint, not service identification. Neither a wildcard nor a private address proves
that another machine can connect. IPv6 wildcard acceptance of IPv4 is not inferred.

## 3. Missing process name

Previously, the ownership warning could be followed by a general binding
recommendation that did not prioritize the missing evidence:

```text
Ownership: partial; process details are incomplete
Next: Keep loopback for local clients; local processes can still connect.
```

Selected lines in the improved view:

```text
INFO UDP [::1]:5353
  Unknown: owner metadata incomplete; firewall not collected
  Check: socklens inspect --protocol udp 5353
```

`Next` now first asks you to identify the owner and explains that names may need
additional OS permissions. The command can still return incomplete metadata;
permissions and process races are not fixed by re-running an inspection. Missing
metadata does not increase severity or turn the listener into a vulnerability.

## 4. Incomplete firewall evidence

Previously, the summary stated that a decision was not established and offered
general configuration advice. It did not select a concrete policy-evidence check.

Selected lines in the improved view when the process is known:

```text
REVIEW TCP 0.0.0.0:8443
  Exposure: All interfaces in this address family; remote reachability is not established.
  Unknown: traffic decision not established (partial firewall coverage)
  Check: socklens audit --protocol tcp --port 8443 --details
```

`Next` first asks you to review policy evidence and source restrictions. Details
show the collected source status, rule conditions and raw evidence. They do not
repair missing access or resolve unsupported firewall layers. A conditional ALLOW
candidate is not an effective allow verdict, and no candidates does not mean
allowed or blocked. Even a hypothetical complete collection remains distinct
from a proved traffic decision.

## How a check is chosen

1. If owner metadata is incomplete, inspect that port first.
2. Otherwise, if collected firewall coverage is incomplete/unknown, request the
   detailed audit for that port.
3. Otherwise, offer the ordinary port inspection as a follow-up.

Checks contain only a validated TCP/UDP protocol and numeric port. They do not
embed process names, addresses, paths or peer input as shell text. Invalid or
zero-port observations do not produce a command. Scope is **port/protocol**:
the check may show other bindings on the same port and does not repeat the
original PID, local-address, or peer filters. Reapply those manually when needed.

Checks are never run by the audit. Both inspection and detailed audit are passive
local reads; the latter may query policy sources again. No probe, elevation,
process termination or firewall change is performed automatically. When firewall
collection was disabled, it is not silently re-enabled by rendering a suggestion.

## Try the workflow

Use the executable containing this development change:

```sh
socklens audit --port 8080 --firewall=false
socklens inspect --protocol tcp 8080
socklens audit --port 8080 --details
socklens audit --port 8080 --json
```

Replace the protocol/port with those actually shown in your audit. The last two
commands expose detailed or structured evidence; `--details` itself does not
make collection more complete. Network reachability requires separate,
user-selected validation, not a conclusion from this passive report.

## Compatibility and layout

This is a default-text presentation improvement. Audit JSON remains schema 1;
there are no new JSON fields or changed finding levels, reading priorities, or
exit codes. `--details` keeps the existing full-evidence presentation. Added text
uses the same terminal escaping, colors and narrow-width wrapping as the rest
of the report. On very narrow terminals, a command may wrap; use a wider view
before copying it as one line.
