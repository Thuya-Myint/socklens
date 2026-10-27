package update

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultInstallTimeout = 5 * time.Minute

type Command struct {
	Program string
	Args    []string
}
type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}
type Runner interface {
	Run(context.Context, Command, Streams) error
	Output(context.Context, Command) ([]byte, error)
}
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command Command, streams Streams) error {
	child := exec.CommandContext(ctx, command.Program, command.Args...)
	child.Stdin, child.Stdout, child.Stderr = streams.In, streams.Out, streams.Err
	return child.Run()
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, fmt.Errorf("command output exceeded %d bytes", b.limit)
	}
	return b.Buffer.Write(data)
}
func (ExecRunner) Output(ctx context.Context, command Command) ([]byte, error) {
	child := exec.CommandContext(ctx, command.Program, command.Args...)
	out, stderr := &boundedBuffer{limit: 4096}, &boundedBuffer{limit: 4096}
	child.Stdout, child.Stderr = out, stderr
	if err := child.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w (%q)", filepath.Base(command.Program), err, stderr.String())
	}
	return out.Bytes(), nil
}

type Updater struct {
	Checker      Checker
	Client       *http.Client
	Executable   string
	OS, Arch     string
	Lookup       func(string) (string, error)
	Runner       Runner
	AssetBaseURL string // internal test injection; production always uses GitHub
	foreground   bool   // only the copied Windows worker bypasses the handoff
}

func DefaultUpdater() Updater {
	checker := DefaultChecker()
	checker.Client = &http.Client{Timeout: 10 * time.Second}
	return Updater{Checker: checker}
}
func (u Updater) system() string {
	if u.OS != "" {
		return u.OS
	}
	return runtime.GOOS
}
func (u Updater) arch() string {
	if u.Arch != "" {
		return u.Arch
	}
	return runtime.GOARCH
}
func (u Updater) runner() Runner {
	if u.Runner != nil {
		return u.Runner
	}
	return ExecRunner{}
}
func (u Updater) lookup(name string) (string, error) {
	if u.Lookup != nil {
		return u.Lookup(name)
	}
	return exec.LookPath(name)
}

type Plan struct {
	Result                                                                Result
	Method, Target, Package, Program, Script, VerifyPath, BucketPath, Key string
	Global                                                                bool
	original                                                              os.FileInfo
	originalHash                                                          string
}
type InstallResult struct{ Version, Method, Status, LogPath string }

func (u Updater) Check(ctx context.Context, current string) (Plan, error) {
	checker := u.Checker
	if checker.Endpoint == "" && checker.Client == nil {
		checker = DefaultUpdater().Checker
	}
	result, err := checker.Check(ctx, current)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Result: result}
	if !result.UpdateAvailable {
		return plan, nil
	}
	location, err := u.Detect()
	if err != nil {
		return Plan{}, err
	}
	location.Result = result
	if location.Method == "standalone" {
		before, err := executableDigest(location.Target)
		if err != nil {
			return Plan{}, err
		}
		installed, err := u.probe(ctx, location.Target)
		if err != nil {
			return Plan{}, fmt.Errorf("cannot verify the installed executable: %w", err)
		}
		after, err := executableDigest(location.Target)
		if err != nil {
			return Plan{}, err
		}
		info, err := os.Lstat(location.Target)
		if err != nil || before != after || !sameExecutable(info, location.original) {
			return Plan{}, fmt.Errorf("installation changed while its version was checked")
		}
		location.originalHash = after
		comparison, err := Compare(installed, result.Latest)
		if err != nil {
			return Plan{}, err
		}
		if comparison >= 0 {
			location.Result.Current = installed
			location.Result.UpdateAvailable = false
			location.Result.Status = "up-to-date"
		}
	}
	return location, nil
}

func (u Updater) Apply(ctx context.Context, plan Plan, streams Streams) (InstallResult, error) {
	if !plan.Result.UpdateAvailable {
		return InstallResult{Version: plan.Result.Current, Method: plan.Method, Status: plan.Result.Status}, nil
	}
	if u.system() == "windows" && !u.foreground {
		return queueInstall(ctx, u, plan)
	}
	switch plan.Method {
	case "homebrew":
		for _, args := range [][]string{{"update"}, {"upgrade", "--formula", plan.Package}} {
			if err := u.runner().Run(ctx, Command{plan.Program, args}, streams); err != nil {
				return InstallResult{}, fmt.Errorf("Homebrew update failed: %w", err)
			}
		}
		data, err := u.runner().Output(ctx, Command{plan.Program, []string{"--prefix", plan.Package}})
		if err != nil {
			return InstallResult{}, err
		}
		prefix := strings.TrimSpace(string(data))
		if !filepath.IsAbs(prefix) || strings.ContainsAny(prefix, "\r\n\x00") {
			return InstallResult{}, fmt.Errorf("Homebrew returned an invalid installation path")
		}
		return u.verifyInstalled(ctx, plan, filepath.Join(prefix, "bin", "socklens"))
	case "scoop":
		if err := u.runScoop(ctx, plan, []string{"update"}, streams); err != nil {
			return InstallResult{}, err
		}
		installed, err := u.probe(ctx, plan.VerifyPath)
		if err != nil {
			return InstallResult{}, err
		}
		if comparison, _ := Compare(installed, plan.Result.Latest); comparison >= 0 {
			return InstallResult{Version: installed, Method: plan.Method, Status: "up-to-date"}, nil
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if err := readInstallJSON(plan.BucketPath, &manifest); err != nil {
			return InstallResult{}, fmt.Errorf("cannot read the Scoop bucket manifest: %w", err)
		}
		if comparison, err := Compare(manifest.Version, plan.Result.Latest); err != nil || comparison < 0 {
			return InstallResult{}, fmt.Errorf("the Scoop bucket has not published %s yet; retry later", plan.Result.Latest)
		}
		if comparison, err := Compare(manifest.Version, installed); err != nil || comparison < 0 {
			return InstallResult{}, fmt.Errorf("refusing a Scoop downgrade")
		}
		args := []string{"update", "socklens"}
		if plan.Global {
			args = append(args, "--global")
		}
		if err := u.runScoop(ctx, plan, args, streams); err != nil {
			return InstallResult{}, err
		}
		return u.verifyInstalled(ctx, plan, plan.VerifyPath)
	case "standalone":
		return u.installStandalone(ctx, plan)
	default:
		return InstallResult{}, fmt.Errorf("unsupported installation method")
	}
}
func (u Updater) runScoop(ctx context.Context, plan Plan, args []string, streams Streams) error {
	command := Command{plan.Program, append([]string{"-NoLogo", "-NoProfile", "-NonInteractive", "-File", plan.Script}, args...)}
	if err := u.runner().Run(ctx, command, streams); err != nil {
		return fmt.Errorf("Scoop update failed: %w", err)
	}
	return nil
}
func (u Updater) probe(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := u.runner().Output(ctx, Command{path, []string{"version"}})
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 || !strings.EqualFold(fields[0], "socklens") {
		return "", fmt.Errorf("unexpected version response %q", string(data))
	}
	if _, err := parseVersion(fields[1]); err != nil {
		return "", err
	}
	return fields[1], nil
}
func (u Updater) verifyInstalled(ctx context.Context, plan Plan, path string) (InstallResult, error) {
	version, err := u.probe(ctx, path)
	if err != nil {
		return InstallResult{}, fmt.Errorf("could not verify the updated installation: %w", err)
	}
	comparison, err := Compare(version, plan.Result.Latest)
	if err != nil || comparison < 0 {
		return InstallResult{}, fmt.Errorf("%s still provides %s, expected %s or newer; the installer may not be published yet", plan.Method, version, plan.Result.Latest)
	}
	if comparison, err := Compare(version, plan.Result.Current); err != nil || comparison < 0 {
		return InstallResult{}, fmt.Errorf("refusing an installed-version downgrade")
	}
	return InstallResult{Version: version, Method: plan.Method, Status: "updated"}, nil
}
