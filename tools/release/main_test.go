package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSettings(t *testing.T) settings {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"README.md", "PHASES.md", "LICENSE", "CHANGELOG.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+" content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return settings{version: "v0.1.0-test", commit: "abcdef123456", stamp: time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), root: root, out: filepath.Join(root, "dist")}
}

func fakeBuild(arch, destination string) error {
	return os.WriteFile(destination, []byte("binary-"+arch), 0755)
}

func readArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	files := map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg || strings.Contains(header.Name, "..") || strings.HasPrefix(header.Name, "/") {
			t.Fatalf("unsafe entry: %+v", header)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = data
		if strings.HasSuffix(header.Name, "/socklens") && header.Mode != 0755 {
			t.Fatal("binary is not executable")
		}
	}
	return files
}

func TestPackageReleaseAndChecksums(t *testing.T) {
	opts := testSettings(t)
	if err := packageRelease(opts, fakeBuild); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(opts.out, opts.version)
	contents, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrong checksum count: %s", contents)
	}
	for i, arch := range []string{"amd64", "arm64"} {
		fields := strings.Fields(lines[i])
		if len(fields) != 2 {
			t.Fatal(lines[i])
		}
		name := "socklens_" + opts.version + "_linux_" + arch
		if fields[1] != name+".tar.gz" {
			t.Fatal(fields[1])
		}
		archive := filepath.Join(directory, fields[1])
		data, err := os.ReadFile(archive)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if fields[0] != hex.EncodeToString(sum[:]) {
			t.Fatal("checksum mismatch")
		}
		files := readArchive(t, archive)
		if len(files) != 5 || string(files[name+"/socklens"]) != "binary-"+arch || len(files[name+"/README.md"]) == 0 || len(files[name+"/PHASES.md"]) == 0 || len(files[name+"/LICENSE"]) == 0 || len(files[name+"/CHANGELOG.md"]) == 0 {
			t.Fatalf("wrong archive contents: %+v", files)
		}
	}
	entries, err := os.ReadDir(opts.out)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging left behind: %v %v", entries, err)
	}
}

func TestPackageNeverOverwrites(t *testing.T) {
	opts := testSettings(t)
	destination := filepath.Join(opts.out, opts.version)
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(destination, "keep")
	os.WriteFile(marker, []byte("original"), 0644)
	called := false
	err := packageRelease(opts, func(arch, path string) error { called = true; return nil })
	data, _ := os.ReadFile(marker)
	if err == nil || called || string(data) != "original" {
		t.Fatalf("existing release changed: %v", err)
	}
}

func TestPackageFailureLeavesNoRelease(t *testing.T) {
	opts := testSettings(t)
	sentinel := errors.New("compiler failed")
	err := packageRelease(opts, func(arch, destination string) error {
		if arch == "arm64" {
			return sentinel
		}
		return fakeBuild(arch, destination)
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("build error lost: %v", err)
	}
	entries, readErr := os.ReadDir(opts.out)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("partial release left: %v %v", entries, readErr)
	}
}

func TestArchiveReproducible(t *testing.T) {
	opts := testSettings(t)
	if err := packageRelease(opts, fakeBuild); err != nil {
		t.Fatal(err)
	}
	second := opts
	second.out = filepath.Join(opts.root, "dist2")
	if err := packageRelease(second, fakeBuild); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SHA256SUMS", "socklens_" + opts.version + "_linux_amd64.tar.gz", "socklens_" + opts.version + "_linux_arm64.tar.gz"} {
		a, _ := os.ReadFile(filepath.Join(opts.out, opts.version, name))
		b, _ := os.ReadFile(filepath.Join(second.out, second.version, name))
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs for identical inputs", name)
		}
	}
}

func TestLabelsAndArguments(t *testing.T) {
	for _, label := range []string{"../escape", "/tmp/out", "x/y", "x y", ".", strings.Repeat("v", 65), "x;touch"} {
		if err := validateLabels(label, "unknown"); err == nil {
			t.Fatalf("accepted unsafe version %q", label)
		}
	}
	if err := validateLabels("v1.2.3-rc.1+local", "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := validateLabels("dev", "bad commit"); err == nil {
		t.Fatal("accepted invalid commit")
	}
	for _, args := range [][]string{{"--version", "../x"}, {"--date", "invalid"}, {"extra"}} {
		if err := run(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestBuildEnvironment(t *testing.T) {
	env := buildEnvironment([]string{"GOOS=windows", "GOARCH=386", "CGO_ENABLED=1", "OTHER=keep"}, "arm64")
	joined := strings.Join(env, "\n")
	for _, want := range []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0", "OTHER=keep"} {
		if !strings.Contains(joined, want) {
			t.Fatal("missing", want)
		}
	}
	for _, key := range []string{"GOOS=", "GOARCH=", "CGO_ENABLED="} {
		count := 0
		for _, entry := range env {
			if strings.HasPrefix(entry, key) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("duplicate %s: %v", key, env)
		}
	}
}
