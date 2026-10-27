# Security policy

The latest published stable release is supported.

Please report security-sensitive findings through GitHub's private vulnerability
reporting feature when it is enabled for this repository. If the Security tab
does not offer that option, contact the maintainer privately through the contact
methods on the GitHub profile; do not post secrets or exploit details publicly.
No response-time guarantee is made.

SockLens reads local proc data and can signal processes owned by the current
effective user after confirmation. Avoid unnecessary elevated privileges.
Release binaries should be checked against SHA256SUMS. Build with a currently
supported, patched Go toolchain.

Private vulnerability reporting must be enabled by the repository owner; this
document does not imply that the repository setting is already enabled.

Update checks read the public GitHub latest-release endpoint. Automatic checks
are disabled unless the user opts in; explicit update-check is user-initiated.
Requests include the installed version in User-Agent and do not include socket
or process data, credentials, or usage records. Update responses are bounded and
validated, and redirects are refused. Checks and notices do not download or execute
release code.

The explicit update command can run the installed Homebrew or Scoop manager, or
download and execute the selected standalone release after SHA-256 and binary
architecture checks. Downloads allow HTTPS redirects only to GitHub release-asset
hosts. Archive paths are not extracted arbitrarily. The staged executable must
report the expected version before installation; a post-install version check
attempts rollback on failure. The checksum manifest and archive share the same
GitHub trust boundary; checksums are not an independent publisher signature.
The updater does not request elevation or change PowerShell execution policy.
On Windows, a copied worker waits for the initiating process to exit, then
records completion or failure in the user's cache.
