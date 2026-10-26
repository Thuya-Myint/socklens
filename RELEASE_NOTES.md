# SockLens v0.1.0

SockLens is a standard-library-only Go CLI for inspecting local ports on Linux
and WSL2.

## Included

- List and inspect TCP/UDP sockets, including IPv4/IPv6 and process owners.
- Filter by local port, protocol, PID, or listener state.
- Watch snapshots or changes in text or newline-delimited JSON.
- Wait for matching sockets to appear or disappear.
- Preview termination or send confirmed, same-user SIGTERM through a stable
  process handle.

## Install — no Go required

SockLens installs as a ready-to-run binary. You do not need Go, a compiler,
or a source checkout.

### Homebrew on Linux / WSL2

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
```

### Binary installer on Linux / WSL2

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
socklens version
```

The installer selects amd64/arm64 and verifies SHA-256 before installing.

### Manual download

Download the archive matching your Linux architecture and SHA256SUMS:

```sh
sha256sum -c SHA256SUMS --ignore-missing
tar -xzf socklens_v0.1.0_linux_amd64.tar.gz
install -Dm755 socklens_v0.1.0_linux_amd64/socklens "$HOME/.local/bin/socklens"
```

For ARM64, replace amd64 with arm64. macOS/native Windows packages are not yet
released; Linux binaries cannot run directly on those operating systems.

## Supported scope

Linux/WSL2 only. Actual termination requires Linux 5.3+ on amd64/arm64 with
pidfd syscalls permitted. Owners are best-effort; sockets reflect the current
network namespace. Polling can miss short-lived sockets. SIGTERM delivery does
not guarantee exit. ARM64 is cross-compiled and not runtime-tested on this
development machine.

Licensed under MIT.
