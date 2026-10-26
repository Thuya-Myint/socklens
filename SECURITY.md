# Security policy

The latest v0.2.x release is the supported release line.

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
