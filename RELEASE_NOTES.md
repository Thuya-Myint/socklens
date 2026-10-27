# SockLens v0.4.0

Default terminal colors and responsive socket output on Windows, macOS, and Linux.
All downloads are standalone binaries; Go and a source checkout are not required.

## Clearer terminal output

- Grouped help, labeled success/error/warning messages, and readable empty results.
- Aligned socket tables with socket, listening/bound, and connected counts.
- Readable process names with safe escaping for unusual or control characters.
- Timestamped watch events with counted added, removed, and updated sections.
- Clearer termination confirmation, dry-run output, and missing-process guidance.

## Responsive layouts and colors

Wide terminals show a table. Narrow terminals show stacked socket records with
wrapped values. Full endpoints, PIDs, and process names remain visible. Watch
checks the width again for each output and adapts after a terminal resize.

Colors are on by default in supported terminals:

```sh
socklens list --listening
socklens --color auto inspect 8080
socklens list --color never
socklens list --color always
```

The global --color option accepts auto (default), always, and never. It may appear
before the command or among its options, before a final PORT argument. NO_COLOR
and TERM=dumb disable automatic colors; --color always overrides those settings.
Windows enables native virtual-terminal output when supported.

Redirected text stays plain by default. JSON output and JSON diagnostics never
contain colors, even with --color always. Version output stays plain. Existing
JSON schemas, exit codes, and safe process-termination protections are unchanged.

## Upgrade on macOS / Linux with Homebrew

Once the v0.4.0 tap formula is promoted:

```sh
brew update
brew upgrade Thuya-Myint/socklens/socklens
socklens version
socklens update-check
```

New installations can tap and install the ready-to-run binary:

```sh
brew tap Thuya-Myint/socklens https://github.com/Thuya-Myint/socklens
brew install Thuya-Myint/socklens/socklens
```

## Windows

New Scoop installation:

```powershell
scoop install https://github.com/Thuya-Myint/socklens/releases/download/v0.4.0/socklens.json
```

The direct Scoop manifest is version-pinned. To replace an older Scoop release,
run `scoop uninstall socklens` before the installation command above. Users of the
standalone PowerShell installer can rerun install.ps1 from the repository to get
the latest published release with checksum verification.

## Linux / WSL2 binary installer

```sh
curl -fsSL https://raw.githubusercontent.com/Thuya-Myint/socklens/main/install.sh -o /tmp/socklens-install.sh
SOCKLENS_VERSION=v0.4.0 sh /tmp/socklens-install.sh
export PATH="$HOME/.local/bin:$PATH"
socklens version
```

## Update checks and automatic notices

```sh
socklens update-check
socklens update-check --timeout 10s
socklens updates --enable
socklens updates --status
```

Update checks compare against published stable releases. Pushing source alone
does not create a new downloadable version. Automatic notices are opt-in and
check at most once every 24 hours during eligible interactive commands. The
background result is cached; a notice can appear on a later command invocation.
Manual update-check requests a fresh result immediately. Notices do not install
updates; use your package manager or the binary installer to upgrade.

## Downloads and validation

Six archives cover Windows, macOS, and Linux on amd64 and arm64. Windows uses ZIP;
macOS and Linux use tar.gz. SHA256SUMS, a Homebrew formula, and a Scoop manifest
are included. See README.md for every CLI command and option.

Native tests passed on Intel/Apple Silicon macOS, x64/ARM64 Windows, and ARM64
Linux, including native Mac terminal resize/color checks. Linux race tests,
vulnerability checks, installation tests, and archive/checksum checks passed.
Runtime source CI: https://github.com/Thuya-Myint/socklens/actions/runs/40453682724

Licensed under MIT.
