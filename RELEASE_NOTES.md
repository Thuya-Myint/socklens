# SockLens v0.2.0

Native Linux, macOS, and Windows port inspection and monitoring. All downloads
are standalone executables; users do not need Go or a source checkout.

## Downloads

Choose amd64 for Intel/AMD 64-bit or arm64 for ARM64/Apple Silicon:

- Linux: socklens_v0.2.0_linux_ARCH.tar.gz
- macOS: socklens_v0.2.0_darwin_ARCH.tar.gz
- Windows: socklens_v0.2.0_windows_ARCH.zip

Verify SHA-256 using SHA256SUMS. Each archive contains a versioned folder with
the executable, license, and documentation.

## Install

Linux/macOS users can use the binary installer from the main repository with
SOCKLENS_VERSION=v0.2.0. Install through the Homebrew tap:

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
socklens version
```

Windows users can download and run install.ps1 from the main repository, or use
the release-hosted Scoop manifest:

```powershell
scoop install https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens.json
socklens version
```

## Included

Native TCP/UDP IPv4/IPv6 discovery and process names; list, inspect, watch,
change detection, and wait; JSON/text output; confirmed process termination and
dry-run previews. Windows termination is immediate and requires --force plus
typed PID confirmation. Linux/macOS use identity-checked SIGTERM and refuse
unsafe fallback. The action never escalates to SIGKILL automatically.

## Platform details

Windows UDP rows are BOUND because the system table does not expose connection
state. macOS process attribution is a best-effort last-recorded PID, not every
holder of a shared socket. Safe macOS signaling depends on kernel support for
audit-token/PID-version checks; unsupported/restricted systems refuse it.
Socket snapshots and polling can miss transient changes.

Native socket/process tests passed on Intel/Apple Silicon macOS, x64/ARM64
Windows, and ARM64 Linux. Linux race tests, vulnerability checks, package
validation, and Homebrew installation checks also passed. See the CI run for
release source commit 6f6d349.
Licensed under MIT.
