# Publishing SockLens

Release notes and CHANGELOG.md must name the intended stable version. Merge the
release source into main, then create and push its vMAJOR.MINOR.PATCH tag. Pushing
ordinary branch commits runs CI but does not publish a release.

The Publish release and installers workflow:

1. Resolves the tag to an immutable main-branch commit and validates release notes.
2. Calls the full CI workflow: supported Go versions, race/security tests, native
   Windows/macOS/Linux tests, current installer tests, packaging and checksums.
3. Builds versioned binaries, uploads a draft, verifies every uploaded digest, and
   publishes the stable release. Older maintenance releases do not replace Latest.
4. Tests the new public Homebrew formula on Linux and both Mac architectures, and
   standalone installers on x64/ARM64 Windows and Linux without Go on PATH.
5. Updates Formula/socklens.rb, bucket/socklens.json, install.sh and installation
   version references on main using an installer-only GitHub Actions bot commit.

The workflow uses scoped GITHUB_TOKEN permissions and needs no personal token.
Installer promotion never force-pushes, retries concurrent main updates, and skips
stale releases. GitHub intentionally does not trigger another push CI run for bot
commits made with GITHUB_TOKEN; the exact installer files are smoke-tested before
promotion, and this avoids a release/CI loop.

To recover a transient failure, rerun failed jobs or manually dispatch the release
workflow with the existing version tag. Published assets are never overwritten;
a rerun downloads and verifies them. Draft uploads can be resumed. A failed test
blocks its dependent publishing/promotion step. All new source tests must pass
before the public release is created; install smoke tests must pass before the
Homebrew tap and other installer references are updated.

If a published binary needs a code fix, create a new patch version rather than
moving the tag or replacing published assets.
