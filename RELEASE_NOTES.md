# SockLens v0.3.0

Check for newer releases directly from SockLens, with optional daily update notices.
Prebuilt binaries support Linux, macOS, and Windows on amd64 and arm64. Go and a
source checkout are not required.

## New commands

```sh
socklens update-check
socklens update-check --json
socklens update-check --timeout 5s
socklens updates --enable
socklens updates --status
socklens updates --disable
```

`update-check` checks the latest stable GitHub release and reports a release link.
It does not install updates. `--timeout` accepts 100ms to 30s; the default is 3s.
An available update returns exit code 0; network and rate-limit failures return 1.

Daily notices are disabled by default. When enabled, eligible interactive commands
can schedule a background check at most once per 24 hours. Cached notices use
standard error and stay out of JSON output. Socket and process information is
never sent. Users can also subscribe to Releases through GitHub's Watch settings.

`socklens version` now prints only the version number, without commit/build details.
Windows settings updates support concurrent readers and atomic file replacement.

## Install or upgrade

### Homebrew — macOS and Linux

For a new installation:

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
```

For an existing installation, after the tap has been updated for this release:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
```

### Scoop — Windows

```powershell
scoop install https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens.json
```

This is a version-pinned manifest. If an older SockLens is already installed with
Scoop, uninstall it with `scoop uninstall socklens`, then run the installation
command above. Other Windows users can run `install.ps1` from the repository;
it installs the latest release and verifies its checksum.

### Linux / WSL2 — binary installer

Run these commands to install or replace an existing binary:

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
SOCKLENS_VERSION=v0.3.0 sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
```

Verify the upgrade:

```sh
socklens version
socklens update-check
```

## Downloads and verification

Choose amd64 for Intel/AMD 64-bit or arm64 for ARM64/Apple Silicon. Linux and macOS
use tar.gz archives; Windows uses ZIP. Verify archive hashes with SHA256SUMS.
The release also includes a binary-only Homebrew formula and Scoop manifest.

All existing list, inspect, watch, wait, check, help, and confirmed termination
commands remain available. See README.md for every command and option.

Validation includes native x64/ARM64 Windows, Intel/Apple Silicon macOS, ARM64 Linux,
Linux race tests, vulnerability checks, package checks, and installer checks.
Licensed under MIT.
