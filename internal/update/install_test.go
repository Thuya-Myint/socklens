package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type installRunner struct {
	commands []Command
	run      func(Command) error
	output   func(Command) ([]byte, error)
}

func (r *installRunner) Run(_ context.Context, c Command, _ Streams) error {
	r.commands = append(r.commands, c)
	if r.run != nil {
		return r.run(c)
	}
	return nil
}
func (r *installRunner) Output(_ context.Context, c Command) ([]byte, error) { return r.output(c) }

func makeStandalone(t *testing.T) (string, []byte) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "socklens")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	old := []byte("old installed executable")
	if err := os.WriteFile(target, old, 0755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return resolved, old
}
func currentTestImage(t *testing.T) []byte {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func packageFixture(t *testing.T, image []byte, arch string, duplicate bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	folder := "socklens_v0.5.0_" + runtime.GOOS + "_" + arch
	name := "socklens"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	count := 1
	if duplicate {
		count = 2
	}
	if runtime.GOOS == "windows" {
		writer := zip.NewWriter(&buffer)
		for i := 0; i < count; i++ {
			header := &zip.FileHeader{Name: folder + "/" + name, Method: zip.Store}
			header.SetMode(0755)
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(image); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		compressed, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
		if err != nil {
			t.Fatal(err)
		}
		writer := tar.NewWriter(compressed)
		for i := 0; i < count; i++ {
			if err := writer.WriteHeader(&tar.Header{Name: folder + "/" + name, Mode: 0755, Size: int64(len(image)), Typeflag: tar.TypeReg}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(image); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return buffer.Bytes()
}
func installerFixture(t *testing.T, target string, body []byte, arch string, runner Runner, manifestOverride string, onArchive func()) Updater {
	t.Helper()
	extension := ".tar.gz"
	if runtime.GOOS == "windows" {
		extension = ".zip"
	}
	name := "socklens_v0.5.0_" + runtime.GOOS + "_" + arch + extension
	hash := sha256.Sum256(body)
	manifest := hex.EncodeToString(hash[:]) + "  " + name + "\n"
	if manifestOverride != "" {
		manifest = manifestOverride
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprint(w, `{"tag_name":"v0.5.0"}`)
		case "/v0.5.0/SHA256SUMS":
			fmt.Fprint(w, manifest)
		case "/v0.5.0/" + name:
			if onArchive != nil {
				onArchive()
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return Updater{Checker: Checker{Endpoint: server.URL + "/latest", Client: server.Client()}, Client: server.Client(), AssetBaseURL: server.URL, Executable: target, Arch: arch, Runner: runner, foreground: true}
}
func fakeBinaryRunner(target string, image []byte) *installRunner {
	return &installRunner{output: func(c Command) ([]byte, error) {
		data, err := os.ReadFile(c.Program)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(data, image) {
			return []byte("socklens v0.5.0\n"), nil
		}
		return []byte("socklens v0.3.0\n"), nil
	}}
}
func TestStandaloneVerifiedUpdate(t *testing.T) {
	target, _ := makeStandalone(t)
	image := currentTestImage(t)
	runner := fakeBinaryRunner(target, image)
	updater := installerFixture(t, target, packageFixture(t, image, runtime.GOARCH, false), runtime.GOARCH, runner, "", nil)
	plan, err := updater.Check(context.Background(), "v0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	result, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard})
	if err != nil || result.Status != "updated" || result.Version != "v0.5.0" {
		t.Fatalf("%+v %v", result, err)
	}
	installed, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(installed, image) {
		t.Fatal("verified executable was not installed", err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil || len(entries) != 1 {
		t.Fatal("update left temporary/lock files behind", err, entries)
	}
}
func TestStandaloneFailurePreservesOriginal(t *testing.T) {
	image := currentTestImage(t)
	for _, kind := range []string{"checksum", "duplicate", "version", "architecture", "rollback", "changed"} {
		t.Run(kind, func(t *testing.T) {
			target, old := makeStandalone(t)
			arch := runtime.GOARCH
			if kind == "architecture" {
				arch = "arm64"
				if runtime.GOARCH == "arm64" {
					arch = "amd64"
				}
			}
			body := packageFixture(t, image, arch, kind == "duplicate")
			runner := fakeBinaryRunner(target, image)
			originalOutput := runner.output
			if kind == "version" || kind == "rollback" {
				runner.output = func(c Command) ([]byte, error) {
					data, err := os.ReadFile(c.Program)
					if err != nil {
						return nil, err
					}
					if bytes.Equal(data, image) && (kind == "version" || c.Program == target) {
						return []byte("socklens v0.4.0\n"), nil
					}
					return originalOutput(c)
				}
			}
			override := ""
			if kind == "checksum" {
				extension := ".tar.gz"
				if runtime.GOOS == "windows" {
					extension = ".zip"
				}
				override = strings.Repeat("0", 64) + "  socklens_v0.5.0_" + runtime.GOOS + "_" + arch + extension + "\n"
			}
			var changed func()
			if kind == "changed" {
				changed = func() {
					if err := os.WriteFile(target, []byte("concurrent installer update"), 0755); err != nil {
						t.Error(err)
					}
				}
			}
			updater := installerFixture(t, target, body, arch, runner, override, changed)
			plan, err := updater.Check(context.Background(), "v0.3.0")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := updater.Apply(context.Background(), plan, Streams{Out: io.Discard, Err: io.Discard}); err == nil {
				t.Fatal("invalid/concurrent update succeeded")
			}
			data, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			expected := old
			if kind == "changed" {
				expected = []byte("concurrent installer update")
			}
			if !bytes.Equal(data, expected) {
				t.Fatalf("existing executable was damaged for %s", kind)
			}
		})
	}
}
func TestChecksumParserRejectsAmbiguity(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, text := range []string{"bad", digest + "  other.zip\n", digest + "  app.zip\n" + digest + "  app.zip\n", strings.Repeat("z", 64) + "  app.zip\n"} {
		if _, err := checksumFor([]byte(text), "app.zip"); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}
func TestAssetDownloadsRejectDowngradeRedirect(t *testing.T) {
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("insecure redirect was followed") }))
	defer insecure.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, insecure.URL, http.StatusFound) }))
	defer server.Close()
	updater := Updater{Client: server.Client()}
	if err := updater.download(context.Background(), server.URL, io.Discard, 1024); err == nil {
		t.Fatal("download followed a redirect to HTTP")
	}
}
func TestExtractionDoesNotFollowArchivePaths(t *testing.T) {
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	if err := writer.WriteHeader(&tar.Header{Name: "../../escape", Mode: 0755, Size: 1, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = writer.Write([]byte("x"))
	_ = writer.Close()
	_ = compressed.Close()
	root := t.TempDir()
	archive := filepath.Join(root, "release.tar.gz")
	_ = os.WriteFile(archive, buffer.Bytes(), 0600)
	if err := extractExecutable(archive, filepath.Join(root, "candidate"), "expected/socklens", false); err == nil {
		t.Fatal("missing executable accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape")); !os.IsNotExist(err) {
		t.Fatal("archive escaped its destination")
	}
}
