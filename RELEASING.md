# Releasing SockLens

1. Review CHANGELOG.md and RELEASE_NOTES.md; choose an unused version tag.
2. Run make check, make test, and the pinned govulncheck command from CI.
3. Check supported platforms and kernel limitations in the notes.
4. Commit the reviewed source locally; never include bin/ or dist/.
5. Build with VERSION, COMMIT, and BUILD_DATE set to the intended commit metadata.
6. Verify SHA256SUMS, archive license/docs, and the extracted amd64 binary.
   Record runtime results per architecture; compile-only targets must be identified.
7. Obtain owner approval before pushing the main branch or version tag.
8. After approval, push main, wait for all CI jobs to pass, then push the reviewed
   annotated tag. Do not force-push a release tag.
9. The manually dispatched Prepare release draft workflow accepts an existing
   tag and creates a draft with its archives. Alternatively, upload the already
   verified local packages through gh release create --draft.
10. Inspect the draft and its checksums before publishing it. Do not publish if
    CI failed or the tag/package commit differs from the reviewed commit.

Publishing a release is separate from local installation. The latest v0.1.x
line is supported. Enable GitHub private vulnerability reporting in repository
settings if desired; the source files do not configure that setting.

## Consumer distribution updates

Keep Formula/socklens.rb URLs/checksums and install.sh's default version aligned
with the intended published release. Verify the Homebrew formula declares no Go
build/runtime dependency and installs bin/socklens from an archive. Keep the
README's binary installation paths before contributor build instructions.
The installation CI job tests Homebrew and the shell installer. Never replace
previously published archives merely to update installation instructions.

## Native release promotion

Native CI must pass before publishing v0.2.0. Review skips for unavailable
identity-checked macOS signaling; do not describe skipped checks as passed.
Publish archives plus SHA256SUMS, socklens.rb, and socklens.json. Once those
URLs exist, promote the generated socklens.rb to Formula/socklens.rb and update
the shell installer's default version. Never point the public formula at
unpublished assets. Scoop can use the release-hosted socklens.json directly.
