# Contributing

Discuss substantial changes in an issue before implementation.

Use a supported Go toolchain (Go 1.26 or newer), GNU Make, and a C compiler for
race tests. Run make check and make test before opening a pull request.
Add tests for changed behavior and update user-facing documentation.

Keep the CLI and release packager standard-library-only. OS-specific code
belongs behind build tags in internal/platform. Never add implicit termination,
PID-based signal fallback, or automatic SIGKILL escalation.

Contributions are submitted under the project's MIT license.
