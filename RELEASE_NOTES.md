# SockLens v0.5.2

Fix Windows Scoop installations failing with "The system cannot find the path specified"
when running update or update --status. All six macOS, Windows, and Linux binaries
are included; users do not need Go or a source checkout.

## Windows Scoop fix

Scoop's current directory is a junction. The previous updater could run the
application through this junction but failed when resolving its installation
path. v0.5.2 resolves the path through a native Windows file handle, including
junctions in custom Scoop roots. Path-resolution failures now identify the
installed executable instead of showing only a bare system error.

A native Windows regression creates real current/root junctions, checks executable
identity, exercises manager delegation, and verifies status lookup. It reproduces
the released failure before the fix and passes afterward. The fixed detector also
passed a read-only check against the affected real Scoop installation.

## Upgrade once through Scoop

The installed v0.5.0/v0.5.1 updater contains the bug, so use Scoop once to receive
the corrected executable:

```powershell
scoop update
scoop update socklens
socklens version
```

The version should be socklens v0.5.2. Future releases can use:

```powershell
socklens update
socklens update --status
```

Windows installs run in the background; wait for completed status before checking
the installed version. Daily notices are independent of installation. Enabling
notices does not repair the old path resolver.

Older direct-manifest Scoop installations need the bucket migration in README.md.
If you have not installed the bucket, follow the README Scoop installation guide.

## macOS and Linux

```sh
socklens update
socklens version
```

Homebrew-managed installs use Homebrew. Standalone binaries use verified release
downloads. v0.4.0 and older require one initial upgrade through their installer to
receive the update command. Linux/WSL2 users can rerun the binary installer.

The native path-resolution change applies to Windows. macOS/Linux retain their
existing path-resolution behavior and are checked in the full release matrix.
Local socket discovery, filtering, watch/wait, confirmed termination, help,
JSON, colors, and update preferences remain available.

## Publication and validation

Required checks cover native Intel/Apple Silicon macOS, x64/ARM64 Windows,
ARM64 Linux, Linux race/security checks, and packaging/checksum verification.
Fresh Homebrew and standalone installer tests gate installer promotion. Every
published change ships all supported OS/CPU binaries and installer updates.

The local exposure audit and remote/security-analysis commands remain proposed
in ROADMAP.md; they are not part of v0.5.2.

Licensed under MIT.
