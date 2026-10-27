package update

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var managerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var tapName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func readInstallJSON(path string, value any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return fmt.Errorf("invalid installation metadata: %s", path)
	}
	file, err := openStateFile(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, (1<<20)+1))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("invalid trailing installation metadata")
	}
	return nil
}

func (u Updater) Detect() (Plan, error) {
	target := u.Executable
	var err error
	if target == "" {
		target, err = os.Executable()
		if err != nil {
			return Plan{}, err
		}
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return Plan{}, err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return Plan{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return Plan{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return Plan{}, fmt.Errorf("refusing to update a non-regular or privileged executable")
	}
	plan := Plan{Target: target, VerifyPath: target, Key: target, original: info}
	directory := filepath.Dir(target)
	if filepath.Base(directory) == "bin" {
		root := filepath.Dir(directory)
		receipt := filepath.Join(root, "INSTALL_RECEIPT.json")
		if _, err := os.Lstat(receipt); err == nil {
			var metadata struct {
				Source struct {
					Tap string `json:"tap"`
				} `json:"source"`
			}
			if err := readInstallJSON(receipt, &metadata); err != nil {
				return Plan{}, err
			}
			if filepath.Base(filepath.Dir(root)) != "socklens" {
				return Plan{}, fmt.Errorf("unsupported Homebrew formula location")
			}
			plan.Method, plan.Package = "homebrew", "socklens"
			if metadata.Source.Tap != "" {
				if !tapName.MatchString(metadata.Source.Tap) {
					return Plan{}, fmt.Errorf("invalid Homebrew tap in receipt")
				}
				plan.Package = metadata.Source.Tap + "/socklens"
			}
			plan.Program, err = u.lookup("brew")
			if err != nil {
				return Plan{}, fmt.Errorf("Homebrew owns this installation, but brew is unavailable: %w", err)
			}
			return plan, nil
		} else if !os.IsNotExist(err) {
			return Plan{}, err
		}
	}
	if u.system() == "windows" {
		appRoot := filepath.Dir(directory)
		appsRoot := filepath.Dir(appRoot)
		if strings.EqualFold(filepath.Base(appRoot), "socklens") && strings.EqualFold(filepath.Base(appsRoot), "apps") {
			metadataPath := ""
			for _, name := range []string{"scoop-install.json", "install.json"} {
				candidate := filepath.Join(directory, name)
				if _, err := os.Lstat(candidate); err == nil {
					metadataPath = candidate
					break
				} else if !os.IsNotExist(err) {
					return Plan{}, err
				}
			}
			if metadataPath == "" {
				return Plan{}, fmt.Errorf("Scoop-style installation has no receipt; refusing direct replacement")
			}
			var metadata struct {
				Bucket string `json:"bucket"`
			}
			if err := readInstallJSON(metadataPath, &metadata); err != nil {
				return Plan{}, err
			}
			if !managerName.MatchString(metadata.Bucket) {
				return Plan{}, fmt.Errorf("this Scoop installation uses a pinned manifest; migrate once to the SockLens bucket before using socklens update")
			}
			scoopRoot := os.Getenv("SCOOP")
			if scoopRoot == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return Plan{}, err
				}
				scoopRoot = filepath.Join(home, "scoop")
			}
			script := filepath.Join(scoopRoot, "apps", "scoop", "current", "bin", "scoop.ps1")
			if _, err := os.Stat(script); err != nil {
				return Plan{}, fmt.Errorf("Scoop owns this installation, but its command script is unavailable: %w", err)
			}
			host, err := u.lookup("pwsh")
			if err != nil {
				host, err = u.lookup("powershell.exe")
			}
			if err != nil {
				return Plan{}, fmt.Errorf("PowerShell is required to update this Scoop installation")
			}
			plan.Method, plan.Program, plan.Script, plan.Package = "scoop", host, script, metadata.Bucket+"/socklens"
			plan.VerifyPath = filepath.Join(appRoot, "current", filepath.Base(target))
			plan.Key = plan.VerifyPath
			physicalRoot := scoopRoot
			if resolved, e := filepath.EvalSymlinks(scoopRoot); e == nil {
				physicalRoot = resolved
			}
			actualRoot := filepath.Dir(appsRoot)
			if !strings.EqualFold(filepath.Clean(actualRoot), filepath.Clean(physicalRoot)) {
				globalRoot := os.Getenv("SCOOP_GLOBAL")
				if globalRoot == "" {
					globalRoot = filepath.Join(os.Getenv("PROGRAMDATA"), "scoop")
				}
				if resolved, e := filepath.EvalSymlinks(globalRoot); e == nil {
					globalRoot = resolved
				}
				if !strings.EqualFold(filepath.Clean(actualRoot), filepath.Clean(globalRoot)) {
					return Plan{}, fmt.Errorf("unknown Scoop installation root; set SCOOP to its manager directory")
				}
				plan.Global = true
			}
			for _, base := range []string{filepath.Join(scoopRoot, "buckets", metadata.Bucket, "bucket"), filepath.Join(scoopRoot, "buckets", metadata.Bucket)} {
				candidate := filepath.Join(base, "socklens.json")
				if _, err := os.Stat(candidate); err == nil {
					plan.BucketPath = candidate
					break
				}
			}
			if plan.BucketPath == "" {
				return Plan{}, fmt.Errorf("cannot locate the installed Scoop bucket; run scoop update and retry")
			}
			return plan, nil
		}
	}
	normalized := filepath.ToSlash(target)
	for _, part := range []string{"/Cellar/socklens/", "/nix/store/", "/snap/", "/Microsoft/WinGet/Packages/"} {
		haystack, needle := normalized, part
		if u.system() == "windows" {
			haystack, needle = strings.ToLower(haystack), strings.ToLower(needle)
		}
		if strings.Contains(haystack, needle) {
			return Plan{}, fmt.Errorf("this path is managed by another installer; refusing direct replacement")
		}
	}
	if u.system() != "windows" {
		for _, base := range []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"} {
			if filepath.Dir(target) == base {
				return Plan{}, fmt.Errorf("system-managed executable path; use its package manager")
			}
		}
	}
	if (u.system() != "linux" && u.system() != "darwin" && u.system() != "windows") || (u.arch() != "amd64" && u.arch() != "arm64") {
		return Plan{}, fmt.Errorf("no published binary for %s/%s", u.system(), u.arch())
	}
	plan.Method = "standalone"
	return plan, nil
}
