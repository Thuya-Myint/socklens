#!/usr/bin/env python3
"""Release/installer automation; uses only Python's standard library and gh/git."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

VERSION = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\Z")
TARGETS = ("linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64", "windows_arm64")


def version_key(value):
    match = VERSION.fullmatch(value)
    if not match or len(value) > 64:
        raise ValueError("expected a stable vMAJOR.MINOR.PATCH version")
    return tuple(int(part) for part in match.groups())


def command(*args, cwd=None, check=True):
    result = subprocess.run(args, cwd=cwd, text=True, encoding="utf-8", stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and result.returncode:
        raise RuntimeError("command failed: " + " ".join(args[:3]) + "\n" + result.stderr)
    return result


def api(repo, endpoint):
    return json.loads(command("gh", "api", "repos/" + repo + "/" + endpoint).stdout)


def release(repo, version):
    result = command("gh", "release", "view", version, "--repo", repo, "--json", "apiUrl", check=False)
    if result.returncode:
        if "release not found" in result.stderr.lower() or "HTTP 404" in result.stderr:
            return None
        raise RuntimeError(result.stderr)
    return json.loads(command("gh", "api", json.loads(result.stdout)["apiUrl"]).stdout)


def latest(repo):
    result = command("gh", "api", "repos/" + repo + "/releases/latest", check=False)
    if result.returncode:
        if "HTTP 404" in result.stderr:
            return None
        raise RuntimeError(result.stderr)
    return json.loads(result.stdout)


def tag_commit(repo, version):
    obj = api(repo, "git/ref/tags/" + version)["object"]
    for _ in range(8):
        if obj["type"] == "commit":
            return obj["sha"]
        if obj["type"] != "tag":
            break
        obj = api(repo, "git/tags/" + obj["sha"])["object"]
    raise ValueError("release tag does not resolve to a commit")


def archive_name(version, target):
    return "socklens_" + version + "_" + target + (".zip" if target.startswith("windows_") else ".tar.gz")


def artifact_hashes(directory, version, repo):
    version_key(version)
    directory = Path(directory)
    archives = {archive_name(version, target) for target in TARGETS}
    expected = archives | {"socklens.rb", "socklens.json"}
    sums = {}
    for line in (directory / "SHA256SUMS").read_text(encoding="utf-8").splitlines():
        fields = line.split()
        if len(fields) != 2 or not re.fullmatch(r"[0-9a-f]{64}", fields[0]) or fields[1] in sums:
            raise ValueError("invalid or duplicate checksum")
        sums[fields[1]] = fields[0]
    if set(sums) != expected:
        raise ValueError("unexpected or missing release checksum entries")
    if {path.name for path in directory.iterdir()} != expected | {"SHA256SUMS"}:
        raise ValueError("unexpected or missing release files")
    for name, digest in sums.items():
        path = directory / name
        if path.is_symlink() or not path.is_file() or hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise ValueError("release checksum mismatch: " + name)
    manifest = directory / "SHA256SUMS"
    if manifest.is_symlink() or not manifest.is_file():
        raise ValueError("invalid release manifest")
    sums["SHA256SUMS"] = hashlib.sha256(manifest.read_bytes()).hexdigest()
    base = "https://github.com/" + repo + "/releases/download/" + version + "/"
    formula = (directory / "socklens.rb").read_text(encoding="utf-8")
    if 'version "' + version[1:] + '"' not in formula:
        raise ValueError("Homebrew version mismatch")
    pairs = re.findall(r'url "([^"\n]+)"\s+sha256 "([0-9a-f]{64})"', formula)
    wanted = {(base + archive_name(version, target), sums[archive_name(version, target)]) for target in TARGETS if not target.startswith("windows_")}
    if len(pairs) != 4 or set(pairs) != wanted:
        raise ValueError("Homebrew URLs/checksums mismatch")
    scoop = json.loads((directory / "socklens.json").read_text(encoding="utf-8"))
    if scoop.get("version") != version[1:] or scoop.get("bin") != "socklens.exe" or set(scoop.get("architecture", {})) != {"64bit", "arm64"}:
        raise ValueError("Scoop metadata mismatch")
    for arch, key in (("amd64", "64bit"), ("arm64", "arm64")):
        name = archive_name(version, "windows_" + arch)
        info = scoop["architecture"][key]
        if info.get("url") != base + name or info.get("hash") != sums[name] or info.get("extract_dir") != name[:-4]:
            raise ValueError("Scoop URL/checksum mismatch")
    return sums


def verify_remote(info, directory, version, repo):
    hashes = artifact_hashes(directory, version, repo)
    if info["tag_name"] != version or info["prerelease"]:
        raise ValueError("release identity mismatch")
    assets = info["assets"]
    if len(assets) != len(hashes) or {asset["name"] for asset in assets} != set(hashes):
        raise ValueError("uploaded assets differ from release files")
    for asset in assets:
        name = asset["name"]
        if asset.get("state") != "uploaded" or asset.get("digest") != "sha256:" + hashes[name] or asset["size"] != (Path(directory) / name).stat().st_size:
            raise ValueError("uploaded asset hash/size mismatch: " + name)


def download(repo, version, directory):
    version_key(version)
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    info = release(repo, version)
    if not info or info["draft"]:
        raise ValueError("published release is required")
    command("gh", "release", "download", version, "--repo", repo, "--dir", str(directory))
    verify_remote(info, directory, version, repo)
    return info


def publish(repo, version, commit, directory):
    version_key(version)
    if not re.fullmatch(r"[0-9a-f]{40}", commit) or tag_commit(repo, version) != commit:
        raise ValueError("release tag moved or does not match tested source")
    artifact_hashes(directory, version, repo)
    info = release(repo, version)
    if info and info.get("target_commitish") != commit:
        raise ValueError("existing release is not bound to the tested commit")
    if info and not info["draft"]:
        # Published assets are immutable. A rerun validates the public files,
        # rather than replacing them with a potentially different toolchain build.
        with tempfile.TemporaryDirectory() as temporary:
            download(repo, version, temporary)
        print("Release already published; verified without replacing assets.")
        return
    files = [str(path) for path in sorted(Path(directory).iterdir())]
    if info:
        command("gh", "release", "upload", version, "--repo", repo, "--clobber", *files)
    else:
        command("gh", "release", "create", version, "--repo", repo, "--verify-tag", "--target", commit, "--draft", "--title", "SockLens " + version, "--notes-file", "RELEASE_NOTES.md", *files)
    verify_remote(release(repo, version), directory, version, repo)
    previous = latest(repo)
    make_latest = previous is None or version_key(version) >= version_key(previous["tag_name"])
    command("gh", "release", "edit", version, "--repo", repo, "--draft=false", "--prerelease=false", "--latest=" + str(make_latest).lower(), "--title", "SockLens " + version, "--notes-file", "RELEASE_NOTES.md")
    info = release(repo, version)
    if info["draft"]:
        raise ValueError("release did not become public")
    verify_remote(info, directory, version, repo)
    print("Published " + version + " with verified assets.")


def installer_updates(root, directory, version, repo):
    root, directory = Path(root), Path(directory)
    artifact_hashes(directory, version, repo)
    current = re.search(r'^\s*version "([0-9]+\.[0-9]+\.[0-9]+)"', (root / "Formula/socklens.rb").read_text(encoding="utf-8"), re.MULTILINE)
    if not current:
        raise ValueError("cannot determine current Homebrew version")
    if version_key("v" + current[1]) > version_key(version):
        return {}
    installer, replacements = re.subn(r'(?<=SOCKLENS_VERSION:-)v[0-9]+\.[0-9]+\.[0-9]+', version, (root / "install.sh").read_text(encoding="utf-8"))
    if replacements != 1:
        raise ValueError("expected one Linux installer version")
    readme = (root / "README.md").read_text(encoding="utf-8")
    readme = re.sub(r'(?<=/releases/download/)v[0-9]+\.[0-9]+\.[0-9]+(?=/socklens\.json)', version, readme)
    readme = re.sub(r'(?<=SOCKLENS_VERSION=)v[0-9]+\.[0-9]+\.[0-9]+(?= sh)', version, readme)
    return {"Formula/socklens.rb": (directory / "socklens.rb").read_bytes(), "bucket/socklens.json": (directory / "socklens.json").read_bytes(), "install.sh": installer.encode(), "README.md": readme.encode()}


def apply_updates(root, updates):
    for name, data in updates.items():
        path = Path(root) / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)


def promote(repo, version, commit, directory):
    version_key(version)
    if tag_commit(repo, version) != commit:
        raise ValueError("release tag differs from tested source")
    published = release(repo, version)
    if not published or published["draft"] or published.get("target_commitish") != commit:
        raise ValueError("cannot promote a release that is unpublished or not bound to the tested commit")
    for attempt in range(3):
        # Rebuild the small installer-only commit on current main on each retry.
        # This preserves concurrent human changes and never force-pushes.
        command("git", "fetch", "origin", "main")
        command("git", "checkout", "-B", "installer-promotion", "origin/main")
        command("git", "merge-base", "--is-ancestor", commit, "HEAD")
        if latest(repo)["tag_name"] != version:
            print("A different release is latest; skipping stale installer promotion.")
            return
        updates = installer_updates(Path.cwd(), directory, version, repo)
        if not updates:
            print("Installers already target a newer version.")
            return
        apply_updates(Path.cwd(), updates)
        paths = list(updates)
        command("git", "add", "--", *paths)
        if command("git", "diff", "--cached", "--quiet", check=False).returncode == 0:
            print("Installers already match this release.")
            return
        command("git", "config", "user.name", "github-actions[bot]")
        command("git", "config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com")
        command("git", "commit", "-m", "Update installers for " + version)
        result = command("git", "push", "origin", "HEAD:main", check=False)
        if result.returncode == 0:
            print("Promoted Homebrew, Scoop, and Linux installers for " + version)
            return
        if attempt == 2:
            raise RuntimeError("installer promotion push failed: " + result.stderr)
        time.sleep(2)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("publish", "download", "stage", "promote"))
    parser.add_argument("--version", required=True)
    parser.add_argument("--directory", required=True)
    parser.add_argument("--commit", default="")
    parser.add_argument("--root", default=".")
    args = parser.parse_args()
    repo = os.environ.get("GITHUB_REPOSITORY", "Thuya-Myint/socklens")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise ValueError("invalid repository")
    directory = Path(args.directory).resolve()
    if args.action == "publish":
        publish(repo, args.version, args.commit, directory)
    elif args.action == "download":
        download(repo, args.version, directory)
    elif args.action == "stage":
        apply_updates(args.root, installer_updates(args.root, directory, args.version, repo))
    else:
        promote(repo, args.version, args.commit, directory)


if __name__ == "__main__":
    main()
