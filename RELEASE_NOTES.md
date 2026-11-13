# SockLens v0.11.0

Event monitoring reconciliation, diagnostics, and authenticated standalone releases
on Windows, macOS, and Linux.

## Included

- Event-mode watch reconciles persistent state after missed notifications with a
  default 20-second idle wait and --reconcile (minimum 1 second).
- watch --stats reports the selected backend, collection count and cumulative
  collection time to stderr on exit. Single snapshots are labelled snapshot.
- doctor [--json] reports discovery results, latency and platform event-backend
  prerequisites without requesting privileges or attaching a tracer.
- Linux owner lookup skips owner-map and process-name work for irrelevant inodes.
- SHA256SUMS.sigstore.json authenticates the manifest using GitHub Actions Sigstore
  signing. Standalone installers and updates require independently installed
  Cosign, verify the exact workflow/tag identity and issuer, and refuse unsigned
  releases before installing or executing candidate code.
- Formatter failures fail make check; doctor output errors propagate.

## Migration and limits

Install Cosign independently before standalone installation or updates. The shell
and PowerShell installers accept SOCKLENS_COSIGN as an absolute verifier path;
the SockLens updater finds cosign on PATH. Use the signed installer for the first
upgrade from an older checksum-only SockLens. Homebrew and Scoop retain their
package-manager metadata trust policies.

Reconciliation cannot reconstruct sockets that open and close between snapshots.
Collection/output time adds to its wait. Event-loss counters, high-load rate
limiting and measured Linux benchmark improvements are not included. Doctor does
not certify tracing attachment. macOS SIP and tracing permissions still apply.
Manifest signing is not native code signing/notarization or SLSA provenance.
