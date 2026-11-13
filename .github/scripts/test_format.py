from pathlib import Path
import shutil
import subprocess
import unittest


class FormatCheckTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("make") and shutil.which("sh"), "requires POSIX make/sh")
    def test_formatter_failure_fails_check_even_without_output(self):
        root = Path(__file__).resolve().parents[2]
        result = subprocess.run(["make", "check", "GOFMT=false", "GO=true"], cwd=root, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
