# SockLens v0.5.0

Check for a newer stable release and install it directly from SockLens:

```sh
socklens update
```

All downloads are standalone binaries for macOS, Windows, and Linux on amd64 and
arm64. Go, a compiler, and a source checkout are not required.

## One initial upgrade from v0.4.0 or older

Older binaries do not contain the new command. Upgrade once using your existing
installer. With Homebrew:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
socklens version
```

The version should be v0.5.0. For future releases, run `socklens update`.

For Scoop bucket installations:

```powershell
scoop update
scoop update socklens
socklens version
```

Older direct-manifest Scoop installs need the one-time bucket migration described
in README.md. Linux/WSL2 users can rerun the binary installer:

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
SOCKLENS_VERSION=v0.5.0 sh /tmp/socklens-install.sh
```

## Update commands and options

```sh
socklens update
socklens update --dry-run
socklens update --timeout 10m
socklens update-check
socklens update-check --json
```

- `update` checks the latest stable release and upgrades only when newer.
- `--dry-run` shows the installation method without installing.
- `--timeout` limits the whole operation; default 5m, allowed 1s to 30m.
- `update-check` remains available for read-only checks and JSON output.
- Update notices now suggest `socklens update`. Daily notices remain opt-in;
  no updates are installed automatically.

Homebrew installations refresh Homebrew and upgrade the receipt's formula.
Scoop installations refresh Scoop and its buckets and upgrade SockLens. Both
paths check the installed version afterward. If the release's installer has not
been promoted yet, retry after publication completes.

Standalone installs download the matching release over HTTPS, verify SHA-256,
architecture, and executable version, then replace the executable. A failed
post-install check attempts to restore the previous executable. The install
directory must be writable. Checksums and archives share the same GitHub trust
boundary; checksums are not an independent publisher signature.

## Windows update progress

Windows starts a copied installer in the background so the initiating executable
can exit before replacement. The command prints a log location. Confirm the result:

```powershell
socklens update --status
socklens version
```

`--status` is used alone and does not contact GitHub. A successful initial exit
means the installer started; the status reports whether it completed or failed.
Other running SockLens instances can block replacement; close them if reported.

No administrator prompt, sudo, shell-profile change, or PowerShell execution-policy
override is added. Unknown package-manager locations are refused. Existing socket
commands, colors, JSON output, and safe process termination remain available.

## Validation and installers

The release workflow gates publication on Linux race/security checks and native
Windows, macOS, and ARM64 Linux tests. Integrity, invalid-version/architecture,
rollback, concurrent replacement, manager delegation, CLI, and Windows handoff
cases are covered. Isolated Linux and Windows copies also passed real downloads
and replacement before publication.

Fresh Homebrew installations and standalone Windows/Linux installers are tested
before Homebrew, the Scoop bucket, and the Linux installer are promoted. Six
platform archives, SHA256SUMS, a Homebrew formula, and a Scoop manifest are included.

Licensed under MIT.
