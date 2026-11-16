# Windows release trust and blocked launches

`Shim: Could not create process ... socklens.exe update` means Scoop's launcher
could not start its target. The text alone does not identify the cause: check the
target exists, its architecture, signature, and Windows security events. When
Windows also reports a blocked app, changing the updater cannot repair a launch
that Windows refuses before application code runs.

Keep Defender, Smart App Control and organizational application-control policies
enabled. Do not remove download-origin metadata, add exclusions, install a
self-signed trust root, or change execution policy to work around this error.
PowerShell execution policy is distinct from native executable application control.

## Read-only diagnosis

```powershell
Get-Content "$env:USERPROFILE\scoop\shims\socklens.shim"
Get-Item "$env:USERPROFILE\scoop\apps\socklens\current" | Select-Object FullName, Target
Get-AuthenticodeSignature "$env:USERPROFILE\scoop\apps\socklens\current\socklens.exe" |
    Format-List Path, Status, SignatureType, SignerCertificate, TimeStamperCertificate
Get-FileHash "$env:USERPROFILE\scoop\apps\socklens\current\socklens.exe" -Algorithm SHA256
```

Review Windows Security Protection History and Event Viewer:
Microsoft-Windows-CodeIntegrity/Operational (including enforcement events 3077
and 3033, with related 3089 signature details), and AppLocker EXE and DLL logs.
An administrator may need to export these logs. Do not infer a malware detection
from an unsigned signature alone. A valid signature also does not guarantee that
every enterprise policy permits an application.

After a trusted release is available, Scoop can update a blocked package without
starting the old SockLens executable: `scoop update`, then `scoop update socklens`.
This does not confer trust on an unsigned replacement. Confirm its signature and
publisher before attempting to start it. Do not repeatedly reinstall v0.11.0 to
resolve a native signature failure.

## Investigation: v0.11.0, 2026-11-16

Both official ZIP hashes match their published GitHub asset digests and the Scoop
manifest. ZIP roots match `extract_dir`; the shim target uses the normal Scoop
`current` junction. PE machines are 0x8664 (amd64) and 0xaa64 (arm64).
Both executables report `NotSigned`. The release workflow signs SHA256SUMS using
Sigstore, which authenticates downloads but is not embedded Authenticode signing.
PR #2 explicitly documented that limitation; no earlier Windows block issue was
found among the repository's two issues/PRs at investigation time.

The inspected PC has Smart App Control enabled (VerifiedAndReputablePolicyState=1).
Its Scoop junction points to the 0.8.0 directory, whose executable is also unsigned;
that directory name alone does not prove the executable's version after updates.
The Code Integrity log was inaccessible without additional privileges and Defender
history could not be queried in the available PowerShell session. Thus a trust
block is strongly supported, but the exact per-file enforcement event and any
Defender verdict remain unconfirmed. No downloaded SockLens binary was executed.

## Maintainer signing setup

The release workflow now builds both Windows binaries from the tested immutable
commit, signs them with Azure Artifact Signing, verifies embedded Authenticode
trust, the exact configured publisher subject, and timestamps on Windows, then
transfers the verified files within the same workflow run. The packager preserves
their bytes, checks PE architecture and a nonempty certificate table, and generates
ZIPs, checksums and Scoop hashes afterward. PE structure checks on Linux do not
replace Windows cryptographic verification. CI development packages remain unsigned.

Before releasing, provision an identity-validated **Public Trust** certificate
profile (a paid external service; this PR does not provision or purchase it).
Create a protected GitHub environment named `windows-signing`, restrict it to
release tags, and bind Azure's OIDC federation to that repository/environment.
Give the identity only the Certificate Profile Signer role at the required profile
scope. Configure these environment variables:

- `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`
- `SIGNING_ENDPOINT`, `SIGNING_ACCOUNT`, `SIGNING_PROFILE`
- `SIGNING_SUBJECT`: the full exact certificate subject expected for the publisher

No private key or long-lived Azure client secret is stored in this workflow.
Missing configuration or a failed signature check prevents publication and package
promotion; there is no unsigned production fallback. Keep the PR in draft until
the signing identity is provisioned and a release rehearsal validates actual
signing, timestamps, both architectures and Scoop installation on protected Windows.
Publish a new patch tag; do not replace the v0.11.0 ZIPs or reuse their checksums.

References: [Microsoft signing guidance](https://learn.microsoft.com/en-us/windows/apps/develop/smart-app-control/code-signing-for-smart-app-control),
[Artifact Signing action](https://github.com/Azure/artifact-signing-action),
[OIDC setup](https://github.com/Azure/artifact-signing-action/blob/main/docs/OIDC.md).
