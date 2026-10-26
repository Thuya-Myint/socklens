// Release builds local Linux packages. It never pushes or publishes.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type settings struct {
	version, commit string
	stamp           time.Time
	root, out       string
}

type archiveFile struct {
	name, path string
	mode       int64
}

var safeLabel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

func validateLabels(version, commit string) error {
	if !safeLabel.MatchString(version) {
		return fmt.Errorf("invalid version: use 1-64 letters, digits, dots, underscores, plus or minus; begin with a letter or digit")
	}
	if !safeLabel.MatchString(commit) {
		return fmt.Errorf("invalid commit label")
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	version := fs.String("version", "dev", "package version")
	commit := fs.String("commit", "unknown", "commit label")
	date := fs.String("date", time.Now().UTC().Format(time.RFC3339), "RFC3339 build date")
	directory := fs.String("out", "dist", "output parent directory")
	goBin := fs.String("go", "go", "Go executable")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if err := validateLabels(*version, *commit); err != nil {
		return err
	}
	stamp, err := time.Parse(time.RFC3339, *date)
	if err != nil {
		return fmt.Errorf("invalid build date: %w", err)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("run from the SockLens project root: %w", err)
	}
	opts := settings{version: *version, commit: *commit, stamp: stamp.UTC(), root: root, out: *directory}
	build := func(arch, destination string) error {
		flags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.buildDate=%s", opts.version, opts.commit, opts.stamp.Format(time.RFC3339))
		cmd := exec.CommandContext(context.Background(), *goBin, "build", "-trimpath", "-ldflags", flags, "-o", destination, "./cmd/socklens")
		cmd.Dir = root
		cmd.Env = buildEnvironment(os.Environ(), arch)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build linux/%s: %w\n%s", arch, err, output)
		}
		return nil
	}
	if err := packageRelease(opts, build); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Created local packages in", filepath.Join(opts.out, opts.version))
	return err
}

func buildEnvironment(base []string, arch string) []string {
	env := make([]string, 0, len(base)+3)
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if key != "GOOS" && key != "GOARCH" && key != "CGO_ENABLED" {
			env = append(env, entry)
		}
	}
	return append(env, "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
}

func packageRelease(opts settings, build func(string, string) error) error {
	if err := validateLabels(opts.version, opts.commit); err != nil {
		return err
	}
	destination := filepath.Join(opts.out, opts.version)
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("output already exists: %s; choose a new version or output directory", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(opts.out, 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(opts.out, ".socklens-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	var sums bytes.Buffer
	for _, arch := range []string{"amd64", "arm64"} {
		work, err := os.MkdirTemp("", "socklens-build-")
		if err != nil {
			return err
		}
		binary := filepath.Join(work, "socklens")
		err = build(arch, binary)
		if err != nil {
			os.RemoveAll(work)
			return err
		}
		name := fmt.Sprintf("socklens_%s_linux_%s", opts.version, arch)
		archive := filepath.Join(staging, name+".tar.gz")
		files := []archiveFile{
			{"README.md", filepath.Join(opts.root, "README.md"), 0644},
			{"LICENSE", filepath.Join(opts.root, "LICENSE"), 0644},
			{"CHANGELOG.md", filepath.Join(opts.root, "CHANGELOG.md"), 0644},
			{"PHASES.md", filepath.Join(opts.root, "PHASES.md"), 0644},
			{"socklens", binary, 0755},
		}
		err = writeArchive(archive, name, opts.stamp, files)
		os.RemoveAll(work)
		if err != nil {
			return err
		}
		file, err := os.Open(archive)
		if err != nil {
			return err
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(digest.Sum(nil)), name+".tar.gz")
	}
	if err := os.WriteFile(filepath.Join(staging, "SHA256SUMS"), sums.Bytes(), 0644); err != nil {
		return err
	}
	// An exclusive lock prevents concurrent packagers from replacing a release.
	// Recheck the final path under that lock before the single directory rename.
	lockPath := destination + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(lockPath)
	if err := lock.Close(); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("output already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staging, destination); err != nil {
		return err
	}

	return nil
}

func writeArchive(destination, prefix string, stamp time.Time, files []archiveFile) (err error) {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		if e := output.Close(); err == nil {
			err = e
		}
	}()
	compressed := gzip.NewWriter(output)
	compressed.Header.ModTime = stamp
	defer func() {
		if e := compressed.Close(); err == nil {
			err = e
		}
	}()
	archive := tar.NewWriter(compressed)
	defer func() {
		if e := archive.Close(); err == nil {
			err = e
		}
	}()
	for _, file := range files {
		info, err := os.Stat(file.path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive input is not a regular file: %s", file.path)
		}
		header := &tar.Header{Name: prefix + "/" + file.name, Mode: file.mode, Size: info.Size(), ModTime: stamp, Typeflag: tar.TypeReg, Format: tar.FormatPAX}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		input, err := os.Open(file.path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
