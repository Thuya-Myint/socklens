package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

const ReleasesURL = "https://github.com/Thuya-Myint/socklens/releases"
const LatestAPI = "https://api.github.com/repos/Thuya-Myint/socklens/releases/latest"

type Result struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url"`
	Status          string `json:"status"`
}
type Checker struct {
	Endpoint string
	Client   *http.Client
}

func DefaultChecker() Checker {
	return Checker{Endpoint: LatestAPI, Client: &http.Client{Timeout: 3 * time.Second}}
}
func (c Checker) Check(ctx context.Context, current string) (Result, error) {
	if _, err := parseVersion(current); err != nil {
		return Result{}, err
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = LatestAPI
	}
	client := c.Client
	if client == nil {
		client = DefaultChecker().Client
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return fmt.Errorf("unexpected update API redirect") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "socklens/"+current)
	response, err := copyClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("could not check GitHub releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		case 403, 429:
			return Result{}, fmt.Errorf("GitHub update checks are rate-limited or unavailable; try again later")
		case 404:
			return Result{}, fmt.Errorf("no published release found")
		default:
			return Result{}, fmt.Errorf("GitHub release check returned HTTP %d", response.StatusCode)
		}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > 1<<20 {
		return Result{}, fmt.Errorf("release response is too large")
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return Result{}, fmt.Errorf("invalid release response: %w", err)
	}
	parsed, err := parseVersion(release.Tag)
	if err != nil {
		return Result{}, fmt.Errorf("invalid release tag: %w", err)
	}
	if release.Draft || release.Prerelease || len(parsed.pre) > 0 {
		return Result{}, fmt.Errorf("latest release is not a published stable release")
	}
	comparison, err := Compare(current, release.Tag)
	if err != nil {
		return Result{}, err
	}
	status := "up-to-date"
	if comparison < 0 {
		status = "update-available"
	} else if comparison > 0 {
		status = "ahead"
	}
	return Result{Current: current, Latest: release.Tag, UpdateAvailable: comparison < 0, ReleaseURL: ReleasesURL + "/tag/" + url.PathEscape(release.Tag), Status: status}, nil
}
func Notice(result Result) string {
	message := fmt.Sprintf("A newer SockLens version is available: %s (installed: %s).\n", result.Latest, result.Current)
	if runtime.GOOS == "windows" {
		message += "Download/update instructions: " + result.ReleaseURL + "\n"
	} else {
		message += "Homebrew: brew update && brew upgrade Thuya-Myint/socklens/socklens\n"
		message += "Other installations: " + result.ReleaseURL + "\n"
	}
	return message
}
func validNonce(value string) bool {
	if len(value) != 32 {
		return false
	}
	return strings.Trim(value, "0123456789abcdef") == ""
}
