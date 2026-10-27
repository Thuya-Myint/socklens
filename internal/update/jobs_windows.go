//go:build windows

package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func queueInstall(ctx context.Context, u Updater, plan Plan) (InstallResult, error) {
	if err := ctx.Err(); err != nil {
		return InstallResult{}, err
	}
	root, err := installRoot()
	if err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return InstallResult{}, err
	}
	key := installKey(plan.Key)
	lock := filepath.Join(root, key+".lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		if os.IsExist(err) {
			job, statusErr := u.InstallStatus()
			if statusErr != nil {
				return InstallResult{}, statusErr
			}
			return InstallResult{Version: job.Latest, Method: job.Method, Status: "in-progress", LogPath: job.LogPath}, nil
		}
		return InstallResult{}, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		os.Remove(lock)
		return InstallResult{}, err
	}
	nonce := hex.EncodeToString(random[:])
	if err := os.WriteFile(filepath.Join(lock, "owner"), []byte(nonce), 0600); err != nil {
		os.Remove(lock)
		return InstallResult{}, err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			unlockInstall(root, key, nonce)
		}
	}()
	directory := filepath.Join(root, nonce)
	if err := os.Mkdir(directory, 0700); err != nil {
		return InstallResult{}, err
	}
	sourcePath, err := os.Executable()
	if err != nil {
		return InstallResult{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return InstallResult{}, err
	}
	defer source.Close()
	helperPath := filepath.Join(directory, "worker.exe")
	helper, err := os.OpenFile(helperPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return InstallResult{}, err
	}
	count, copyErr := io.Copy(helper, io.LimitReader(source, maxExecutable+1))
	closeErr := helper.Close()
	if copyErr != nil {
		return InstallResult{}, copyErr
	}
	if closeErr != nil {
		return InstallResult{}, closeErr
	}
	if count > maxExecutable {
		return InstallResult{}, fmt.Errorf("installer worker executable exceeds size limit")
	}
	deadline := time.Now().Add(DefaultInstallTimeout)
	if value, ok := ctx.Deadline(); ok {
		deadline = value
	}
	job := InstallJob{Nonce: nonce, Key: key, Target: plan.Target, Current: plan.Result.Current, Latest: plan.Result.Latest, Method: plan.Method, Status: "queued", ParentPID: os.Getpid(), Deadline: deadline, UpdatedAt: time.Now().UTC(), LogPath: filepath.Join(directory, "update.log")}
	path := filepath.Join(directory, "job.json")
	if err := saveInstallJSON(path, job); err != nil {
		return InstallResult{}, err
	}
	if err := saveInstallJSON(filepath.Join(root, key+".json"), struct {
		Nonce string `json:"nonce"`
	}{nonce}); err != nil {
		return InstallResult{}, err
	}
	cleanInstallJobs(root, key)
	log, err := os.OpenFile(job.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return InstallResult{}, err
	}
	defer log.Close()
	child := exec.Command(helperPath, "__install-update", nonce)
	child.Stdout, child.Stderr = log, log
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200}
	if err := child.Start(); err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.UpdatedAt = time.Now().UTC()
		_ = saveInstallJSON(path, job)
		return InstallResult{}, err
	}
	go func() { _ = child.Wait() }()
	handedOff = true
	return InstallResult{Version: job.Latest, Method: job.Method, Status: "started", LogPath: job.LogPath}, nil
}

func waitForInstallerParent(ctx context.Context, pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return fmt.Errorf("invalid installer parent process")
	}
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if err == syscall.Errno(87) {
		return nil
	}
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < timeout {
		timeout = time.Until(deadline)
	}
	if timeout <= 0 {
		return context.DeadlineExceeded
	}
	result, err := syscall.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}
	if result != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("installer parent did not exit before the deadline")
	}
	return nil
}
func RunInstallWorker(ctx context.Context, nonce string) error {
	return runInstallWorker(ctx, nonce, func(target string) Updater {
		u := DefaultUpdater()
		u.Executable = target
		u.foreground = true
		return u
	})
}
func runInstallWorker(ctx context.Context, nonce string, factory func(string) Updater) (resultErr error) {
	root, err := installRoot()
	if err != nil {
		return err
	}
	job, err := loadInstallJob(root, nonce)
	if err != nil {
		return err
	}
	directory := filepath.Join(root, nonce)
	if job.Status != "queued" || job.Deadline.IsZero() || time.Until(job.Deadline) > 30*time.Minute {
		return fmt.Errorf("installer job cannot be claimed")
	}
	if err := os.Mkdir(filepath.Join(directory, "claimed"), 0700); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(directory, "claimed"))
	defer unlockInstall(root, job.Key, nonce)
	path := filepath.Join(directory, "job.json")
	defer func() {
		job.UpdatedAt = time.Now().UTC()
		if resultErr != nil {
			job.Status = "failed"
			job.Error = resultErr.Error()
		} else {
			job.Status = "completed"
		}
		if err := saveInstallJSON(path, job); resultErr == nil {
			resultErr = err
		}
	}()
	owner, err := os.ReadFile(filepath.Join(root, job.Key+".lock", "owner"))
	if err != nil || string(owner) != nonce {
		return fmt.Errorf("installer lock is no longer owned by this job")
	}
	ctx, cancel := context.WithDeadline(ctx, job.Deadline)
	defer cancel()
	job.PID = os.Getpid()
	job.Status = "running"
	job.UpdatedAt = time.Now().UTC()
	if err := saveInstallJSON(path, job); err != nil {
		return err
	}
	if err := waitForInstallerParent(ctx, job.ParentPID); err != nil {
		return err
	}
	updater := factory(job.Target)
	updater.foreground = true
	plan, err := updater.Check(ctx, job.Current)
	if err != nil {
		return err
	}
	minimum := plan.Result.Current
	if plan.Result.UpdateAvailable {
		minimum = plan.Result.Latest
	}
	if comparison, err := Compare(minimum, job.Latest); err != nil || comparison < 0 {
		return fmt.Errorf("the requested release is no longer available; retry the update")
	}
	if plan.Result.UpdateAvailable && installKey(plan.Key) != job.Key {
		return fmt.Errorf("installation identity changed before update")
	}
	outcome, err := updater.Apply(ctx, plan, Streams{Out: os.Stdout, Err: os.Stderr})
	if err != nil {
		return err
	}
	job.Latest = outcome.Version
	return nil
}
