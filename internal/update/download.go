package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxDownload = 64 << 20
const maxExecutable = 64 << 20

func (u Updater) assetURL(version, name string) (string, error) {
	base := u.AssetBaseURL
	if base == "" {
		base = ReleasesURL + "/download"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("invalid release download endpoint")
	}
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(version) + "/" + name, nil
}
func (u Updater) download(ctx context.Context, address string, out io.Writer, limit int64) error {
	origin, err := url.Parse(address)
	if err != nil || origin.Scheme != "https" {
		return fmt.Errorf("release downloads require HTTPS")
	}
	client := u.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	safe := *client
	safe.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 || request.URL.Scheme != "https" || request.URL.User != nil {
			return fmt.Errorf("unsafe release download redirect")
		}
		host := request.URL.Hostname()
		if request.URL.Host != origin.Host && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com" && host != "github-releases.githubusercontent.com" {
			return fmt.Errorf("unexpected release download host")
		}
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "socklens-updater")
	response, err := safe.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("release download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > limit {
		return fmt.Errorf("release download exceeds size limit")
	}
	count, err := io.Copy(out, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if count > limit {
		return fmt.Errorf("release download exceeds size limit")
	}
	return nil
}
func checksumFor(data []byte, name string) (string, error) {
	digest := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 {
			return "", fmt.Errorf("invalid release checksum manifest")
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return "", fmt.Errorf("invalid release checksum digest")
		}
		if fields[1] == name {
			if digest != "" {
				return "", fmt.Errorf("duplicate release checksum entry")
			}
			digest = strings.ToLower(fields[0])
		}
	}
	if digest == "" {
		return "", fmt.Errorf("release checksum for this platform is missing")
	}
	return digest, nil
}
func extractExecutable(archive, candidate, member string, windows bool) error {
	output, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	copyFile := func(input io.Reader, size int64) error {
		if size <= 0 || size > maxExecutable {
			return fmt.Errorf("release executable has invalid size")
		}
		count, err := io.Copy(output, io.LimitReader(input, maxExecutable+1))
		if err != nil {
			return err
		}
		if count != size {
			return fmt.Errorf("release executable is truncated or oversized")
		}
		return nil
	}
	found := 0
	if windows {
		var reader *zip.ReadCloser
		reader, err = zip.OpenReader(archive)
		if err == nil {
			defer reader.Close()
			if len(reader.File) > 64 {
				err = fmt.Errorf("release archive has too many entries")
			}
			for _, entry := range reader.File {
				if err != nil {
					break
				}
				if entry.Name != member {
					continue
				}
				found++
				if found > 1 || !entry.Mode().IsRegular() || entry.UncompressedSize64 > maxExecutable {
					err = fmt.Errorf("invalid or duplicate release executable")
					break
				}
				input, openErr := entry.Open()
				if openErr != nil {
					err = openErr
					break
				}
				err = copyFile(input, int64(entry.UncompressedSize64))
				closeErr := input.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
	} else {
		var input *os.File
		input, err = os.Open(archive)
		if err == nil {
			defer input.Close()
			var compressed *gzip.Reader
			compressed, err = gzip.NewReader(input)
			if err == nil {
				defer compressed.Close()
				reader := tar.NewReader(io.LimitReader(compressed, 128<<20))
				for count := 0; count < 65; count++ {
					var header *tar.Header
					header, err = reader.Next()
					if err == io.EOF {
						err = nil
						break
					}
					if err != nil {
						break
					}
					if count == 64 || header.Size < 0 || header.Size > maxExecutable {
						err = fmt.Errorf("release archive exceeds limits")
						break
					}
					if header.Name != member {
						continue
					}
					found++
					if found > 1 || header.Typeflag != tar.TypeReg {
						err = fmt.Errorf("invalid or duplicate release executable")
						break
					}
					err = copyFile(reader, header.Size)
					if err != nil {
						break
					}
				}
			}
		}
	}
	if err == nil && found != 1 {
		err = fmt.Errorf("release archive has no matching executable")
	}
	if err == nil {
		err = output.Sync()
	}
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
func validateImage(path, system, arch string) error {
	switch system {
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		machine := elf.EM_X86_64
		if arch == "arm64" {
			machine = elf.EM_AARCH64
		}
		if file.Class != elf.ELFCLASS64 || file.Machine != machine {
			return fmt.Errorf("release executable targets another architecture")
		}
	case "darwin":
		file, err := macho.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		cpu := macho.CpuAmd64
		if arch == "arm64" {
			cpu = macho.CpuArm64
		}
		if file.Cpu != cpu || file.Type != macho.TypeExec {
			return fmt.Errorf("release executable targets another architecture")
		}
	case "windows":
		file, err := pe.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
		if arch == "arm64" {
			machine = pe.IMAGE_FILE_MACHINE_ARM64
		}
		if file.Machine != machine {
			return fmt.Errorf("release executable targets another architecture")
		}
	default:
		return fmt.Errorf("unsupported executable format")
	}
	return nil
}
func (u Updater) installStandalone(ctx context.Context, plan Plan) (InstallResult, error) {
	lock := plan.Target + ".update-lock"
	if err := os.Mkdir(lock, 0700); err != nil {
		return InstallResult{}, fmt.Errorf("installation is not writable or another update is active (%s): %w", lock, err)
	}
	defer os.Remove(lock)
	work, err := os.MkdirTemp(filepath.Dir(plan.Target), ".socklens-update-")
	if err != nil {
		return InstallResult{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(work)
		}
	}()
	extension, binaryName := ".tar.gz", "socklens"
	if u.system() == "windows" {
		extension, binaryName = ".zip", "socklens.exe"
	}
	folder := "socklens_" + plan.Result.Latest + "_" + u.system() + "_" + u.arch()
	name := folder + extension
	manifestURL, err := u.assetURL(plan.Result.Latest, "SHA256SUMS")
	if err != nil {
		return InstallResult{}, err
	}
	var manifest boundedBuffer
	manifest.limit = 64 << 10
	if err := u.download(ctx, manifestURL, &manifest, 64<<10); err != nil {
		return InstallResult{}, err
	}
	expected, err := checksumFor(manifest.Bytes(), name)
	if err != nil {
		return InstallResult{}, err
	}
	archivePath := filepath.Join(work, "release"+extension)
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return InstallResult{}, err
	}
	address, err := u.assetURL(plan.Result.Latest, name)
	digest := sha256.New()
	if err == nil {
		err = u.download(ctx, address, io.MultiWriter(archive, digest), maxDownload)
	}
	closeErr := archive.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return InstallResult{}, err
	}
	if hex.EncodeToString(digest.Sum(nil)) != expected {
		return InstallResult{}, fmt.Errorf("release checksum mismatch; installation unchanged")
	}
	candidate := filepath.Join(work, binaryName)
	if err := extractExecutable(archivePath, candidate, folder+"/"+binaryName, u.system() == "windows"); err != nil {
		return InstallResult{}, err
	}
	if err := validateImage(candidate, u.system(), u.arch()); err != nil {
		return InstallResult{}, err
	}
	version, err := u.probe(ctx, candidate)
	if err != nil {
		return InstallResult{}, fmt.Errorf("new executable verification failed: %w", err)
	}
	if comparison, err := Compare(version, plan.Result.Latest); err != nil || comparison != 0 {
		return InstallResult{}, fmt.Errorf("downloaded executable version does not match release")
	}
	if err := os.Chmod(candidate, plan.original.Mode().Perm()); err != nil {
		return InstallResult{}, err
	}
	current, err := os.Lstat(plan.Target)
	if err != nil || !sameExecutable(current, plan.original) {
		return InstallResult{}, fmt.Errorf("installation changed during download; retry update")
	}
	previousDigest, err := executableDigest(plan.Target)
	if err != nil || plan.originalHash == "" || previousDigest != plan.originalHash {
		return InstallResult{}, fmt.Errorf("installation contents changed during download; retry update")
	}
	if err := ctx.Err(); err != nil {
		return InstallResult{}, err
	}
	backup := filepath.Join(work, "previous"+filepath.Ext(binaryName))
	rollback := func(cause error) (InstallResult, error) {
		if _, err := os.Stat(backup); err == nil {
			if err := restoreExecutable(plan.Target, backup); err != nil {
				cleanup = false
				return InstallResult{}, fmt.Errorf("%w; rollback failed: %v; previous executable saved at %s", cause, err, backup)
			}
		}
		return InstallResult{}, cause
	}
	if err := replaceExecutable(plan.Target, candidate, backup); err != nil {
		return rollback(err)
	}
	installed, err := u.verifyInstalled(ctx, plan, plan.Target)
	if err != nil {
		return rollback(err)
	}
	return installed, nil
}

func sameExecutable(left, right os.FileInfo) bool {
	return left != nil && right != nil && os.SameFile(left, right) && left.Size() == right.Size() && left.ModTime().Equal(right.ModTime()) && left.Mode() == right.Mode()
}
func executableDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, maxExecutable+1))
	if err != nil {
		return "", err
	}
	if count > maxExecutable {
		return "", fmt.Errorf("installed executable exceeds size limit")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
