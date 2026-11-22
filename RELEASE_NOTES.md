# SockLens v0.12.0

Stable socket JSON, clearer monitoring and audit output, saved snapshots, advanced
filters, and basic shell completions on Windows, macOS, and Linux.

## Included

- `list` and `inspect` accept `--format json-v1`: a versioned envelope, structured
  endpoints, explicit counts and consistent empty arrays. Existing `--json` output
  stays compatible. JSON v1 disables colors and automatic update notices/checks.
- `watch --event-interval` bounds burst-triggered collections (default 100ms),
  while idle reconciliation remains enabled. `--stats` adds collection reasons,
  runtime errors, selected source and delivered/coalesced notification counts.
  Statistics stay on stderr; monitoring JSON records are unchanged.
- bpftrace/DTrace attachment allows a bounded 30-second startup and reports
  bounded helper diagnostics on timeout. Cancellation remains interruptible.
  Early backend deadline errors are distinguished from reconciliation timeouts;
  failed partial sources are closed before explicit auto fallback.
- Default `audit` text adds Exposure, Unknown and one focused, passive Check
  command. Missing owner metadata and incomplete firewall evidence stay explicit.
  Audit JSON, detailed evidence, finding levels/order and exit codes are unchanged.
- `snapshot save` and `snapshot diff` persist versioned local observations and
  compare added, removed and updated socket records, with bounded input handling.
- Shared socket filters add process-name substrings, state, local port ranges and
  local IP addresses. Completion generation supports Bash, Zsh, Fish and PowerShell.
- Added compatibility, real-listener, burst/reconciliation, concurrency, timeout,
  escaping, narrow-layout and passive-command regression coverage, plus detailed
  JSON, watch and before/after audit documentation.

## Examples

```sh
socklens inspect --format json-v1 8080
socklens watch --port 8080 --event-interval 250ms --reconcile 5s --stats
socklens audit --port 8080
socklens snapshot save --protocol tcp before.json
socklens snapshot save --protocol tcp after.json
socklens snapshot diff --json before.json after.json
socklens list --process node --port-range 8000-9000
socklens completion powershell
```

Flags precede positional ports and snapshot file paths. `--format json-v1` is
for list/inspect only; watch still uses `--json` newline-delimited records.

## Upgrade and limits

Use your package manager or `socklens update` after installer promotion completes.
Standalone installers/updaters require independently installed Cosign to verify
the signed checksum manifest against the exact workflow/tag identity and issuer.
Homebrew and Scoop retain their package-manager metadata trust policies.

Manifest signing is **not Windows Authenticode signing or macOS notarization**.
Native Windows signing remains separate unfinished work; Windows application
trust policies may still block the executable. No security-policy bypass is
included. This release does not submit or certify a Microsoft Store package.

Monitoring is best-effort event-triggered snapshot collection, not lossless
socket history. Coalescing is not kernel event loss; backend drop counts remain
unavailable. Reconciliation cannot reconstruct short-lived sockets. Tracing
permissions and macOS SIP restrictions still apply. A passive audit does not
prove external reachability or classify a listener as safe/vulnerable.

Snapshot files, watch JSON lines and JSON v1 envelopes are distinct formats.
Use fresh snapshot destinations. Completion scripts offer basic suggestions;
complete command/subcommand/filter coverage remains follow-up work. No GUI,
remote agent, automatic firewall modification or CVE matching is included.
