#!/bin/sh
# Install a published SockLens binary; never compile or require Go.
set -eu

fail() { printf '%s\n' "socklens installer: $*" >&2; exit 1; }

version=${SOCKLENS_VERSION:-v0.1.0}
case "$version" in v[0-9]*) ;; *) fail "invalid version: $version" ;; esac
case "$version" in *[!A-Za-z0-9._+-]*) fail "invalid version: $version" ;; esac
case "$(uname -s)" in
  Linux) system=linux ;;
  Darwin) fail "macOS binaries are not released yet. This release supports Linux and WSL2." ;;
  *) fail "native support for this operating system is not released yet. Windows users can use WSL2." ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) fail "unsupported CPU architecture" ;;
esac
for tool in curl tar gzip mktemp chmod mv mkdir cat rm; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool not found: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then
  hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  hash_tool=shasum
else
  fail "sha256sum or shasum is required to verify the download"
fi

install_dir=${SOCKLENS_INSTALL_DIR:-${HOME:?HOME must be set}/.local/bin}
[ -n "$install_dir" ] || fail "installation directory must not be empty"
destination="$install_dir/socklens"
[ ! -d "$destination" ] && [ ! -L "$destination" ] || fail "refusing to replace a directory or symlink: $destination"

work=$(mktemp -d "${TMPDIR:-/tmp}/socklens-install.XXXXXXXX")
staged=
cleanup() {
  if [ -n "$staged" ]; then rm -f "$staged"; fi
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
archive="socklens_${version}_${system}_${arch}.tar.gz"
folder="socklens_${version}_${system}_${arch}"
base="https://github.com/Thuya-Myint/socklens/releases/download/$version"
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' "$base/$archive" -o "$work/$archive"
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' "$base/SHA256SUMS" -o "$work/SHA256SUMS"

expected=
while read -r digest name extra; do
  if [ "$name" = "$archive" ]; then
    [ -z "$expected" ] || fail "duplicate checksum entry"
    expected=$digest
  fi
done < "$work/SHA256SUMS"
[ "${#expected}" -eq 64 ] || fail "release checksum is missing or invalid"
case "$expected" in *[!0123456789abcdefABCDEF]*) fail "invalid release checksum" ;; esac
if [ "$hash_tool" = sha256sum ]; then
  actual=$(sha256sum "$work/$archive")
else
  actual=$(shasum -a 256 "$work/$archive")
fi
actual=${actual%% *}
[ "$actual" = "$expected" ] || fail "checksum mismatch; nothing installed"

# Extract only the named executable to a new regular file, not archive paths.
tar -xOzf "$work/$archive" "$folder/socklens" > "$work/socklens"
[ -s "$work/socklens" ] || fail "archive does not contain an executable"
mkdir -p "$install_dir"
staged=$(mktemp "$install_dir/.socklens.XXXXXXXX")
cat "$work/socklens" > "$staged"
chmod 755 "$staged"
mv -f "$staged" "$destination"
staged=
printf '%s\n' "Installed SockLens $version to $destination"
printf 'Run: "%s" version\n' "$destination"
case ":${PATH:-}:" in
  *:"$install_dir":*) ;;
  *) printf '%s\n' "Add $install_dir to your PATH to run socklens from any folder." ;;
esac
