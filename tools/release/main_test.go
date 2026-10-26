package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

func fakeBuild(target buildTarget, destination string) error {
	return os.WriteFile(destination, []byte("binary-"+target.label()), 0755)
}

func readArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	if strings.HasSuffix(path, ".zip") {
		archive, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer archive.Close()
		result := map[string][]byte{}
		for _, entry := range archive.File {
			reader, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			result[entry.Name] = data
		}
		return result
	}
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
	if len(lines) != len(releaseTargets)+2 {
		t.Fatalf("wrong checksum count: %s", contents)
	}
	for i, target := range releaseTargets {
		fields := strings.Fields(lines[i])
		if len(fields) != 2 {
			t.Fatal(lines[i])
		}
		name := "socklens_" + opts.version + "_" + target.label()
		if fields[1] != name+target.extension() {
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
		binaryName := "socklens"
		if target.OS == "windows" {
			binaryName += ".exe"
		}
		if len(files) != 5 || string(files[name+"/"+binaryName]) != "binary-"+target.label() || len(files[name+"/README.md"]) == 0 || len(files[name+"/PHASES.md"]) == 0 || len(files[name+"/LICENSE"]) == 0 || len(files[name+"/CHANGELOG.md"]) == 0 {
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
	err := packageRelease(opts, func(target buildTarget, path string) error { called = true; return nil })
	data, _ := os.ReadFile(marker)
	if err == nil || called || string(data) != "original" {
		t.Fatalf("existing release changed: %v", err)
	}
}

func TestPackageFailureLeavesNoRelease(t *testing.T) {
	opts := testSettings(t)
	sentinel := errors.New("compiler failed")
	err := packageRelease(opts, func(target buildTarget, destination string) error {
		if target.Arch == "arm64" {
			return sentinel
		}
		return fakeBuild(target, destination)
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
	for _, name := range releaseFileNames(opts.version) {
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
	env := buildEnvironment([]string{"GOOS=windows", "GOARCH=386", "CGO_ENABLED=1", "OTHER=keep"}, buildTarget{"windows", "arm64"})
	joined := strings.Join(env, "\n")
	for _, want := range []string{"GOOS=windows", "GOARCH=arm64", "CGO_ENABLED=0", "OTHER=keep"} {
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

func releaseFileNames(version string) []string {
	names := []string{"SHA256SUMS", "socklens.rb", "socklens.json"}
	for _, target := range releaseTargets {
		names = append(names, "socklens_"+version+"_"+target.label()+target.extension())
	}
	return names
}

func TestPackageManagerMetadata(t *testing.T) {
	opts := testSettings(t)
	if err := packageRelease(opts, fakeBuild); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(opts.out, opts.version)
	data, err := os.ReadFile(filepath.Join(directory, "socklens.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version      string
		Bin          string
		Architecture map[string]struct {
			URL        string
			Hash       string
			ExtractDir string `json:"extract_dir"`
		}
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "0.1.0-test" || manifest.Bin != "socklens.exe" || len(manifest.Architecture) != 2 {
		t.Fatalf("bad Scoop manifest: %s", data)
	}
	for key, entry := range manifest.Architecture {
		arch := key
		if key == "64bit" {
			arch = "amd64"
		}
		name := "socklens_" + opts.version + "_windows_" + arch
		bytes, err := os.ReadFile(filepath.Join(directory, name+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(bytes)
		if entry.Hash != fmt.Sprintf("%x", hash) || entry.ExtractDir != name || !strings.HasSuffix(entry.URL, name+".zip") {
			t.Fatalf("Scoop entry does not match release: %+v", entry)
		}
	}
	formula, err := os.ReadFile(filepath.Join(directory, "socklens.rb"))
	if err != nil {
		t.Fatal(err)
	}
	for _, system := range []string{"darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			if !strings.Contains(string(formula), "socklens_"+opts.version+"_"+system+"_"+arch+".tar.gz") {
				t.Fatal("formula missing target", system, arch)
			}
		}
	}
}
