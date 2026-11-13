# Releasing SockLens

Follow [.github/RELEASING.md](.github/RELEASING.md) for the automated release flow.
Update CHANGELOG.md and RELEASE_NOTES.md, validate the release branch in CI,
then merge the reviewed source into main and push a new version tag after owner
authorization. Never move a published tag or overwrite published assets.

The release workflow signs SHA256SUMS with GitHub Actions Sigstore identity and
uploads SHA256SUMS.sigstore.json. Its publish job alone receives id-token: write.
It verifies the exact workflow/tag identity and issuer, rejects a tampered
manifest and incorrect identity, then publishes the tested archives. Installer
promotion waits for signed-release installation checks on all supported release
architectures. Standalone installers require independently installed Cosign,
which the workflow bootstraps using pinned verifier checksums.

Manual dispatch must select the existing version tag as workflow ref and input.
Branch commits run CI without signing or publishing. Package managers retain
their metadata trust policy. Manifest signing does not imply SLSA provenance,
Authenticode signing or macOS notarization.
