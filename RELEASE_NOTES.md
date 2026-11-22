# SockLens v0.12.1

Safer snapshot saves and contextual shell completion on Windows, macOS and Linux.

## Included

- Snapshot saves publish the complete file atomically without overwriting an
  existing file, a concurrent writer's destination, or a dangling symlink.
  Filesystems without hard-link support fail safely; use NTFS, ext4, APFS or
  another filesystem with hard links. The 32 MiB limit includes the final newline.
- Bash, Zsh, Fish and PowerShell 7 completion scripts suggest commands,
  snapshot save/diff subcommands, command-specific options, filter flags,
  selected option values, and snapshot/CA filenames. Options follow command help.
- Added a practical before/after snapshot guide and shell setup instructions.
  Regenerate completion scripts after upgrading to get the new behavior.
- Regression checks cover concurrent saves, destination preservation, malformed
  and oversized files, combined filters, and all four shell adapters.
- Fixed a background-worker test race without changing runtime worker behavior.

## Examples

```sh
socklens snapshot save --protocol tcp --port 8080 before.json
# Start your service in another terminal.
socklens snapshot save --protocol tcp --port 8080 after.json
socklens snapshot diff --json before.json after.json
socklens completion bash
```

Put flags before filenames and use a new destination for every save. Snapshot
files, legacy --json output and list/inspect --format json-v1 are distinct formats.
See [snapshot and completion setup](docs/snapshots-and-completion.md).

## Upgrade and distribution

Use your package manager or `socklens update` after installer promotion completes.
Standalone installation/update requires independently installed Cosign to verify
checksum manifests against the exact release workflow/tag identity and issuer.
Homebrew and Scoop retain their package-manager metadata trust policies.

The checksum manifest is signed with Sigstore. Native Windows Authenticode
signing remains unfinished: Windows application-trust policies may still block
standalone executables. This release does not submit or certify a Microsoft Store
package. Existing security protections remain in place.

Monitoring remains best-effort event-triggered observation, and audit output does
not establish external reachability. This patch preserves the existing JSON formats.
