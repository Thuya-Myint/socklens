package installtest

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fixture struct {
	script, tools, downloads, destination string
	env                                   []string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("POSIX Linux installer integration")
	}
	script, err := filepath.Abs("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	f := fixture{script: script, tools: filepath.Join(root, "tools"), downloads: filepath.Join(root, "downloads"), destination: filepath.Join(root, "installed")}
	for _, dir := range []string{f.tools, f.downloads, f.destination} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"tar", "gzip", "mktemp", "chmod", "mv", "mkdir", "cat", "rm", "sha256sum"} {
		actual, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(actual, filepath.Join(f.tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	cp, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	uname := `#!/bin/sh
case "$1" in -s) printf '%s\n' "$TEST_OS";; -m) printf '%s\n' "$TEST_ARCH";; esac
`
	curl := `#!/bin/sh
destination=
url=
while [ "$#" -gt 0 ]; do
  case "$1" in -o) shift; destination=$1 ;; https://*) url=$1 ;; esac
  shift
done
exec "$TEST_CP" "$TEST_DOWNLOADS/${url##*/}" "$destination"
`
	for name, body := range map[string]string{"uname": uname, "curl": curl} {
		if err := os.WriteFile(filepath.Join(f.tools, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	name := "socklens_v0.1.0_linux_amd64.tar.gz"
	file, err := os.Create(filepath.Join(f.downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	zip := gzip.NewWriter(file)
	archive := tar.NewWriter(zip)
	body := []byte("#!/bin/sh\nprintf '%s\\n' 'socklens fixture'\n")
	if err := archive.WriteHeader(&tar.Header{Name: "socklens_v0.1.0_linux_amd64/socklens", Mode: 0755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(body); err != nil {
		t.Fatal(err)
	}
	for _, close := range []func() error{archive.Close, zip.Close, file.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(f.downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(filepath.Join(f.downloads, "SHA256SUMS"), []byte(fmt.Sprintf("%x  %s\n", sum, name)), 0644); err != nil {
		t.Fatal(err)
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "PATH" && key != "SOCKLENS_VERSION" && key != "SOCKLENS_INSTALL_DIR" {
			f.env = append(f.env, entry)
		}
	}
	f.env = append(f.env, "PATH="+f.tools, "TEST_OS=Linux", "TEST_ARCH=x86_64", "TEST_CP="+cp, "TEST_DOWNLOADS="+f.downloads, "SOCKLENS_INSTALL_DIR="+f.destination, "SOCKLENS_VERSION=v0.1.0")
	return f
}

func (f fixture) run(extra ...string) (string, error) {
	cmd := exec.Command("/bin/sh", f.script)
	cmd.Env = append(append([]string{}, f.env...), extra...)
	data, err := cmd.CombinedOutput()
	return string(data), err
}

func TestInstallWithoutGoOrGit(t *testing.T) {
	f := newFixture(t)
	out, err := f.run()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	installed := filepath.Join(f.destination, "socklens")
	stat, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0755 {
		t.Fatal("installed executable permissions", stat.Mode())
	}
	cmd := exec.Command(installed, "version")
	cmd.Env = f.env
	data, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(data), "socklens fixture") {
		t.Fatalf("%s %v", data, err)
	}
	// PATH contains only installer prerequisites, with no Go, Git, or compiler.
	if _, err := os.Stat(filepath.Join(f.tools, "go")); !os.IsNotExist(err) {
		t.Fatal("unexpected Go executable")
	}
}

func TestBadChecksumPreservesExistingInstall(t *testing.T) {
	f := newFixture(t)
	target := filepath.Join(f.destination, "socklens")
	os.WriteFile(target, []byte("keep existing"), 0755)
	os.WriteFile(filepath.Join(f.downloads, "socklens_v0.1.0_linux_amd64.tar.gz"), []byte("tampered"), 0644)
	out, err := f.run()
	data, _ := os.ReadFile(target)
	if err == nil || !strings.Contains(out, "checksum mismatch") || string(data) != "keep existing" {
		t.Fatalf("%v %s", err, out)
	}
}

func TestUnsupportedAndInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ env, message string }{
		{"TEST_OS=Darwin", "macOS binaries are not released"},
		{"TEST_ARCH=mips", "unsupported CPU"},
		{"SOCKLENS_VERSION=../../escape", "invalid version"},
	} {
		f := newFixture(t)
		out, err := f.run(tc.env)
		if err == nil || !strings.Contains(out, tc.message) {
			t.Fatalf("%s: %v %s", tc.env, err, out)
		}
		if _, err := os.Stat(filepath.Join(f.destination, "socklens")); !os.IsNotExist(err) {
			t.Fatal("installed despite rejected input")
		}
	}
}

func TestInstallerRefusesSymlinkDestination(t *testing.T) {
	f := newFixture(t)
	target := filepath.Join(t.TempDir(), "important")
	os.WriteFile(target, []byte("keep"), 0644)
	if err := os.Symlink(target, filepath.Join(f.destination, "socklens")); err != nil {
		t.Fatal(err)
	}
	out, err := f.run()
	data, _ := os.ReadFile(target)
	if err == nil || !strings.Contains(out, "symlink") || string(data) != "keep" {
		t.Fatalf("%v %s", err, out)
	}
}
