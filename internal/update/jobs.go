package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type InstallJob struct {
	Nonce     string    `json:"nonce"`
	Key       string    `json:"key"`
	Target    string    `json:"target"`
	Current   string    `json:"current"`
	Latest    string    `json:"latest"`
	Method    string    `json:"method"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	LogPath   string    `json:"log_path"`
	ParentPID int       `json:"parent_pid"`
	PID       int       `json:"pid,omitempty"`
	Deadline  time.Time `json:"deadline"`
	UpdatedAt time.Time `json:"updated_at"`
}

func installRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "socklens", "installer"), nil
}
func installKey(value string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(value))))
	return hex.EncodeToString(digest[:])
}
func saveInstallJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".install-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return replaceStateFile(name, path)
}
func loadInstallJob(root, nonce string) (InstallJob, error) {
	if !validNonce(nonce) {
		return InstallJob{}, fmt.Errorf("invalid installer worker nonce")
	}
	var job InstallJob
	if err := readInstallJSON(filepath.Join(root, nonce, "job.json"), &job); err != nil {
		return job, err
	}
	if job.Nonce != nonce || !validInstallKey(job.Key) || !filepath.IsAbs(job.Target) {
		return InstallJob{}, fmt.Errorf("invalid installer job")
	}
	return job, nil
}
func validInstallKey(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func (u Updater) InstallStatus() (InstallJob, error) {
	plan, err := u.Detect()
	if err != nil {
		return InstallJob{}, err
	}
	root, err := installRoot()
	if err != nil {
		return InstallJob{}, err
	}
	var index struct {
		Nonce string `json:"nonce"`
	}
	if err := readInstallJSON(filepath.Join(root, installKey(plan.Key)+".json"), &index); err != nil {
		if os.IsNotExist(err) {
			return InstallJob{Status: "not-started"}, nil
		}
		return InstallJob{}, err
	}
	return loadInstallJob(root, index.Nonce)
}
func unlockInstall(root, key, nonce string) {
	lock := filepath.Join(root, key+".lock")
	owner, err := os.ReadFile(filepath.Join(lock, "owner"))
	if err == nil && string(owner) == nonce {
		os.Remove(filepath.Join(lock, "owner"))
		os.Remove(lock)
	}
}

// Remove only files from old terminal jobs in this application's private cache.
// Unknown files, symlinks, active jobs, and recent workers are left alone.
func cleanInstallJobs(root, key string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validNonce(entry.Name()) {
			continue
		}
		job, err := loadInstallJob(root, entry.Name())
		if err != nil || job.Key != key || (job.Status != "completed" && job.Status != "failed") || time.Since(job.UpdatedAt) < time.Minute {
			continue
		}
		directory := filepath.Join(root, entry.Name())
		files, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		safe := true
		for _, file := range files {
			if file.Type()&os.ModeSymlink != 0 || (file.Name() != "job.json" && file.Name() != "update.log" && file.Name() != "worker.exe" && file.Name() != "claimed") {
				safe = false
				break
			}
			if file.IsDir() {
				children, err := os.ReadDir(filepath.Join(directory, file.Name()))
				if file.Name() != "claimed" || err != nil || len(children) != 0 {
					safe = false
					break
				}
			}
		}
		if !safe {
			continue
		}
		for _, file := range files {
			os.Remove(filepath.Join(directory, file.Name()))
		}
		os.Remove(directory)
	}
}
