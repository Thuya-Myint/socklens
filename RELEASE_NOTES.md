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

## Install

Download the Linux archive matching your architecture and SHA256SUMS from
this release, then verify and extract it:

```sh
sha256sum -c SHA256SUMS --ignore-missing
tar -xzf socklens_v0.1.0_linux_amd64.tar.gz
install -Dm755 socklens_v0.1.0_linux_amd64/socklens "$HOME/.local/bin/socklens"
export PATH="$HOME/.local/bin:$PATH"
socklens version
socklens check
```

For ARM64, replace amd64 with arm64. No Go installation is needed to run the
downloaded binary.

## Supported scope

Linux/WSL2 only. Actual termination requires Linux 5.3+ on amd64/arm64 with
pidfd syscalls permitted. Owners are best-effort; sockets reflect the current
network namespace. Polling can miss short-lived sockets. SIGTERM delivery does
not guarantee exit. ARM64 is cross-compiled and not runtime-tested on this
development machine.

Licensed under MIT.
