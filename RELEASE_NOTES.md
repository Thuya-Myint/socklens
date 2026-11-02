# SockLens v0.5.1

Clearer wait command help and documentation across Windows, macOS, and Linux.
The existing port discovery, monitoring, termination, and update behavior is unchanged.

## What changed

- Main help includes both wait --until present and wait --until absent examples.
- wait --help explains both conditions, immediate success, timeout, and cancellation.
- wait and inspect help no longer advertise --port; both require a final PORT argument.
- JSON descriptions explain each command's actual output instead of showing a watch-specific note everywhere.
- README documents wait start/stop examples, filters, polling, defaults, and exit codes.
- ROADMAP records the proposed local exposure audit and later remote/security work.
  Proposed commands such as audit are not implemented in this release.

## Wait examples

Wait for a TCP listener to appear:

```sh
socklens wait --until present --listening --protocol tcp --timeout 30s 8080
```

Wait for it to disappear:

```sh
socklens wait --until absent --listening --protocol tcp --timeout 30s 8080
```

Both check immediately and succeed if the condition already holds. Otherwise,
the command polls until the condition matches or the timeout expires. The default
condition is present, timeout is 30s, and polling interval is 500ms. Use
--timeout 0 to wait until interrupted. Put all options before the final port.

## Upgrade from v0.5.0

On macOS and Linux:

```sh
socklens update
socklens version
socklens wait --help
```

On Windows, wait for background installation to complete:

```powershell
socklens update
socklens update --status
socklens version
socklens wait --help
```

The installed version should be socklens v0.5.1. If status is still queued or
running, check it again before verifying the version. Homebrew and Scoop installs
use their installed package managers; standalone installs use verified downloads.

## Older installations

v0.4.0 and older need one initial package-manager upgrade because they do not
contain the update command. With Homebrew:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
```

With a Scoop bucket installation:

```powershell
scoop update
scoop update socklens
```

Older direct-manifest Scoop installs need the bucket migration in README.md.
Linux/WSL2 users can rerun the binary installer. Go and a source checkout are
not needed for any supported binary installation.

## Release validation

The release workflow runs Linux race/security checks, native Windows/macOS/Linux
checks, packaging, and checksum verification before publication. It tests fresh
Homebrew and standalone installations before promoting installer metadata.
Six prebuilt platform archives, SHA256SUMS, Homebrew, and Scoop metadata are included.

Licensed under MIT.
