# Firewall context in audit (v0.7.0)

SockLens explains locally collected rule candidates. It does not simulate a
complete firewall decision, capture packets, list every UDP peer, or test remote
reachability. It never changes policy, requests elevation, or downloads tools.

## Commands

```sh
socklens audit
socklens audit --port 5432 --peer 192.168.1.20
socklens audit --protocol udp --peer 2001:db8::20 --json
socklens audit --firewall-timeout 30s --rule-limit 0
socklens audit --firewall=false
```

--peer accepts one numeric unicast IP (mapped IPv4 is normalized); no hostname,
port, multicast, or unspecified address is accepted. A scoped IPv6 peer is
accepted, but interface/zone applicability is unresolved. --peer does not filter
out socket records; it filters their candidate rules and explains binding-family
compatibility. --peer cannot be combined with --firewall=false.

--firewall-timeout is a total collection deadline (default 20s, range 1s–2m).
--rule-limit is a text display limit per direction per listener (default 3, range
0–10000); 0 removes the selected text view's limit. Use --details to include
unverified candidates. Omitted related candidates are counted with instructions to view
all. JSON always includes every collected candidate. Rules that cannot be read
or represented are covered by collection/interpretation limitations, not omitted
under an implied complete result.

## Related rule evidence (v0.7.2)

Concise text starts with transport/binding observations, then shows conditional
rule evidence related to an explicit known local-port restriction or an observed
full executable-path match. Broad rules, missing path matches, and collected
package/service/user/security restrictions that remain unresolved are classified
unverified and retained in --details/JSON. Do not interpret their absence from the
concise view as absence from policy or proof that traffic is permitted.

Candidate relevance is endpoint, process, or unverified. It describes evidence
relation, never a final firewall decision. Windows identity_conditions preserve
unresolved package/service/user/security restrictions as structured evidence;
services associated with a PID do not bypass these conditions or establish which
shared-host service owns a socket. Other platform collectors retain their existing
coverage limits, including raw/unparsed restrictions and traversal uncertainty.
Windows IPv4 dotted subnet masks are normalized for peer comparisons; invalid
masks remain unresolved. A supplied --peer is a policy question, not observed
remote traffic. UDP peer information may be absent; TCP listener snapshots refer
to inspect for observed connection rows.

--rule-limit applies to the selected text view. --rule-limit 0 removes the default
view's related-rule limit; --details --rule-limit 0 displays all candidates.
JSON always keeps all candidates, including unverified ones, with relevance and
any available identity_conditions. Schema_version remains 1 with additive fields.

## Concise text and full details (v0.7.1)

Default text shows rule names/actions, conditional labels, protocol, local/remote
ports and IP restrictions, and a short application/profile/interface scope.
Program paths are abbreviated to basenames in the concise view, with instructions
to view the full path. Empty restriction arrays are displayed as unresolved.
Source/profile context and interpretation limits appear once per snapshot.

--details adds full evidence, collector/profile/interface metadata, rule IDs,
program paths, conditions, raw policy, and per-rule interpretation reasons.
It does not change collection or candidate matching. Combine --details with
--rule-limit 0 to show every candidate in full. JSON always retains all collected
candidate data, regardless of either display option.

## What to read

1. Collection coverage and source errors: partial means some context was read;
   unknown means no usable supported policy could be collected.
2. Interface addresses and network profiles/default settings, when available.
3. Inbound and outbound candidate rules for each socket, including allow/block
   action and unresolved restrictions. Block candidates are displayed first for
   review; this order does not implement OS rule precedence.
4. Evidence and recommendations from the existing listener audit.

All directional assessments remain unknown. Candidate match is conditional.
A matching allow rule does not prove access. An empty candidate list does not
prove allowed or blocked. Allow/block candidates may have different conditions;
coexistence is reported without claiming a confirmed effective conflict.

## Matching and interpretation

Supported protocol, local-port ranges, numeric addresses/CIDRs/address ranges,
and family restrictions can exclude rules that cannot match the endpoint/peer.
Dynamic keywords such as LocalSubnet and RPC remain unresolved candidates.
Missing fields are unknown, not an unrestricted-policy verdict. Negated/complex
expressions are preserved for review rather than guessed.

A wildcard socket lists observed same-family addresses on up interfaces; a
rule restricted to a particular local address may only apply on that interface.
An IPv4 peer does not establish IPv4 acceptance by an IPv6 wildcard. Non-loopback
peers against loopback bindings are explained without a firewall verdict.

Application rules are compared with best-effort owning executable paths, never
basenames. Missing paths retain conditional candidates. Process/socket snapshots
are not atomic; PID reuse and process changes remain possible. macOS application
bundle paths may cover executable paths inside that bundle.

Remote ports are not supplied by --peer and remain unresolved. Inbound concerns
traffic to this local endpoint; outbound concerns traffic from this local
endpoint, including potential replies. This is not a list of all destinations
the owning process can contact from other local ports.

State tracking, actual active interface/profile applicability, service/package
identity, authentication/IPsec restrictions, precedence, rule traversal and other
filtering layers are not fully evaluated. Rules retain these conditions for
review. No new assessment changes existing review levels or reading priority.

## Platform coverage

| Platform | Read-only sources | Important limits |
| --- | --- | --- |
| Windows | System Windows PowerShell NetSecurity ActiveStore rules plus associated port/address/application/service/interface/security filters; HNetCfg.FwPolicy2 COM fallback | Bulk filters can require privileges; fallback exposes available enabled rules, current profile bitmask, firewall enabled state and default actions. AppContainer, Hyper-V, service hardening, other WFP providers and stateful reply rules are not fully covered. Profile bitmask Domain=1, Private=2, Public=4. NotConfigured stays unresolved. |
| Linux/WSL | nft -j list ruleset; if unavailable/unreadable, iptables-save/ip6tables-save -t filter | Optional installed tools and policy-read permissions are needed. Only straightforward direct input/output match fields are normalized. Custom-chain traversal, sets/expressions not understood, legacy rules alongside nftables, other hooks/tables, eBPF, container and host policy remain unresolved. WSL does not expose the Windows host firewall. |
| macOS | socketfilterfw status/block-all/signed-app settings/application rules; pfctl status and recursive anchor rules | Application Firewall is inbound/application focused. PF rules retain raw syntax rather than normalized port/IP matching, so they may appear as broad conditional candidates. Privileged PF queries may be denied. Signed-app exceptions, PF traversal, Network Extensions and other providers are not simulated. |

Successful collection is always partial. Missing tools, permissions, parse
failures, deadlines, or unsupported systems are explicitly reported. No source
failure implies that the firewall is disabled. Queries are local and bounded to
8 MiB per output stream. Source error text and rule strings are escaped in text;
JSON is color-free. Cancellation stops queries and exits 130. Policy-query
failure/timeout still returns a socket report with coverage limits and exit 0;
invalid options and socket discovery failures exit 1.

## JSON extension

The report keeps schema_version 1 with additive optional fields:

- report.firewall: coverage, sources (name/status/detail), profiles, interfaces,
  and notes. Internal raw rule lists and PID-to-path caches are not serialized.
- listeners[].firewall: coverage, optional peer, peer_compatibility,
  local_addresses, inbound, and outbound.
- Each direction: assessment=unknown, summary, rules (an array, empty when none).
- Each candidate: match=conditional, relevance (endpoint/process/unverified), reasons, and rule.
- A rule: id, name, source, direction, action, family, protocols, local_ports,
  remote_ports, local_addresses, remote_addresses, optional program/profiles/
  interfaces/raw, and conditions. Empty restriction arrays mean unknown or not
  normalized; do not interpret them as a confirmed unrestricted policy.

Use summary and structured fields for scripts. Human text wraps to terminal
width. The complete socket report remains available with --firewall=false,
which omits the optional firewall fields and invokes no policy commands.

## Sources

- [Microsoft firewall rules and associated filters](https://learn.microsoft.com/en-us/powershell/module/netsecurity/get-netfirewallrule)
- [Microsoft port filters](https://learn.microsoft.com/en-us/powershell/module/netsecurity/get-netfirewallportfilter)
- [Microsoft INetFwPolicy2](https://learn.microsoft.com/en-us/windows/win32/api/netfw/nn-netfw-inetfwpolicy2)
- [nftables manual](https://netfilter.org/projects/nftables/manpage.html)
- [Apple firewall security](https://support.apple.com/guide/security/seca0e83763f/web)
- [Apple: Packet Filter is not API](https://developer.apple.com/documentation/technotes/tn3165-packet-filter-is-not-api)
