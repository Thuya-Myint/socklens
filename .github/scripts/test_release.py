import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import release as flow

REPO = "Thuya-Myint/socklens"
VERSION = "v0.4.0"
COMMIT = "a" * 40


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.make_assets()

    def make_assets(self):
        for target in flow.TARGETS:
            (self.assets / flow.archive_name(VERSION, target)).write_bytes(target.encode())
        hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in self.assets.iterdir()}
        base = "https://github.com/" + REPO + "/releases/download/" + VERSION + "/"
        formula = 'class Socklens < Formula\n  version "0.4.0"\n'
        for target in flow.TARGETS:
            if not target.startswith("windows_"):
                name = flow.archive_name(VERSION, target)
                formula += '  url "' + base + name + '"\n  sha256 "' + hashes[name] + '"\n'
        (self.assets / "socklens.rb").write_text(formula + "end\n")
        architecture = {}
        for arch, key in (("amd64", "64bit"), ("arm64", "arm64")):
            name = flow.archive_name(VERSION, "windows_" + arch)
            architecture[key] = {"url": base + name, "hash": hashes[name], "extract_dir": name[:-4]}
        (self.assets / "socklens.json").write_text(json.dumps({"version": "0.4.0", "architecture": architecture, "bin": "socklens.exe"}))
        self.refresh_checksums()

    def refresh_checksums(self):
        lines = []
        for path in sorted(self.assets.iterdir()):
            if path.name != "SHA256SUMS":
                lines.append(hashlib.sha256(path.read_bytes()).hexdigest() + "  " + path.name)
        (self.assets / "SHA256SUMS").write_text("\n".join(lines) + "\n")

    def info(self, draft=False):
        hashes = flow.artifact_hashes(self.assets, VERSION, REPO)
        return {"tag_name": VERSION, "target_commitish": COMMIT, "draft": draft, "prerelease": False, "assets": [{"name": name, "digest": "sha256:" + digest, "size": (self.assets / name).stat().st_size, "state": "uploaded"} for name, digest in hashes.items()]}

    def test_stable_versions_and_numeric_order(self):
        self.assertGreater(flow.version_key("v0.10.0"), flow.version_key("v0.9.0"))
        for value in ("v01.0.0", "v0.4.0-beta", "v1.2.3;echo bad", "main", "v1.2"):
            with self.assertRaises(ValueError):
                flow.version_key(value)

    def test_valid_metadata_and_remote_digests(self):
        self.assertEqual(len(flow.artifact_hashes(self.assets, VERSION, REPO)), 9)
        info = self.info()
        flow.verify_remote(info, self.assets, VERSION, REPO)
        info["assets"][0]["digest"] = "sha256:" + "0" * 64
        with self.assertRaises(ValueError):
            flow.verify_remote(info, self.assets, VERSION, REPO)

    def test_modified_download_fails_closed(self):
        (self.assets / flow.archive_name(VERSION, "darwin_arm64")).write_bytes(b"changed")
        with self.assertRaises(ValueError):
            flow.artifact_hashes(self.assets, VERSION, REPO)

    def test_duplicate_or_unexpected_checksum_fails(self):
        path = self.assets / "SHA256SUMS"
        text = path.read_text()
        for extra in (text.splitlines()[0], "0" * 64 + "  ../../secret"):
            path.write_text(text + extra + "\n")
            with self.assertRaises(ValueError):
                flow.artifact_hashes(self.assets, VERSION, REPO)

    def test_metadata_cannot_point_to_another_release(self):
        path = self.assets / "socklens.json"
        content = json.loads(path.read_text())
        content["architecture"]["64bit"]["url"] = "https://example.com/other.zip"
        path.write_text(json.dumps(content))
        self.refresh_checksums()
        with self.assertRaises(ValueError):
            flow.artifact_hashes(self.assets, VERSION, REPO)

    def test_unverified_draft_is_never_published(self):
        bad = self.info(draft=True)
        bad["assets"][0]["digest"] = "sha256:" + "0" * 64
        with mock.patch.object(flow, "tag_commit", return_value=COMMIT), mock.patch.object(flow, "release", side_effect=[self.info(draft=True), bad]), mock.patch.object(flow, "command") as commands:
            with self.assertRaises(ValueError):
                flow.publish(REPO, VERSION, COMMIT, self.assets)
            self.assertFalse(any("edit" in call.args for call in commands.call_args_list))

    def test_moved_tag_is_never_published(self):
        with mock.patch.object(flow, "tag_commit", return_value="b" * 40), mock.patch.object(flow, "command") as commands:
            with self.assertRaises(ValueError):
                flow.publish(REPO, VERSION, COMMIT, self.assets)
            commands.assert_not_called()

    @mock.patch("builtins.print")
    def test_published_assets_are_not_replaced_on_rerun(self, _print):
        with mock.patch.object(flow, "tag_commit", return_value=COMMIT), mock.patch.object(flow, "release", return_value=self.info()), mock.patch.object(flow, "download") as download, mock.patch.object(flow, "command") as commands:
            flow.publish(REPO, VERSION, COMMIT, self.assets)
            download.assert_called_once()
            commands.assert_not_called()

    @mock.patch("builtins.print")
    def test_older_release_does_not_become_latest(self, _print):
        with mock.patch.object(flow, "tag_commit", return_value=COMMIT), mock.patch.object(flow, "release", side_effect=[None, self.info(draft=True), self.info()]), mock.patch.object(flow, "latest", return_value={"tag_name": "v0.5.0"}), mock.patch.object(flow, "command") as commands:
            flow.publish(REPO, VERSION, COMMIT, self.assets)
            edit = next(call.args for call in commands.call_args_list if "edit" in call.args)
            self.assertIn("--latest=false", edit)

    def make_installer_root(self, version="0.3.0"):
        root = self.root / "checkout"
        (root / "Formula").mkdir(parents=True)
        (root / "Formula/socklens.rb").write_text('class Socklens < Formula\n version "' + version + '"\nend\n')
        (root / "install.sh").write_text('version=${SOCKLENS_VERSION:-v' + version + '}\n')
        (root / "README.md").write_text('scoop install https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens.json\nSOCKLENS_VERSION=v0.3.0 sh /tmp/install.sh\nUpdate checks require v0.3.0.\n')
        return root

    def test_promotion_updates_all_installers_and_keeps_compatibility_docs(self):
        root = self.make_installer_root()
        updates = flow.installer_updates(root, self.assets, VERSION, REPO)
        self.assertEqual(set(updates), {"Formula/socklens.rb", "bucket/socklens.json", "install.sh", "README.md"})
        flow.apply_updates(root, updates)
        self.assertEqual((root / "Formula/socklens.rb").read_bytes(), (self.assets / "socklens.rb").read_bytes())
        self.assertEqual(json.loads((root / "bucket/socklens.json").read_text())["version"], "0.4.0")
        self.assertIn("SOCKLENS_VERSION:-v0.4.0", (root / "install.sh").read_text())
        self.assertIn("/v0.4.0/socklens.json", (root / "README.md").read_text())
        self.assertIn("Update checks require v0.3.0.", (root / "README.md").read_text())

    def test_promotion_cannot_downgrade_installers(self):
        root = self.make_installer_root("0.5.0")
        self.assertEqual(flow.installer_updates(root, self.assets, VERSION, REPO), {})


if __name__ == "__main__":
    unittest.main()
