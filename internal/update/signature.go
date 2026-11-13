package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const releaseIssuer = "https://token.actions.githubusercontent.com"

// Verify the manifest before its hashes authorize an archive or executable.
// Cosign must be installed independently; never run a verifier from the release.
func (u Updater) verifyReleaseManifest(ctx context.Context, version, work string, manifest, bundle []byte) error {
	program, err := u.lookup("cosign")
	if err != nil {
		return fmt.Errorf("install Sigstore cosign to verify standalone releases: %w", err)
	}
	manifestPath := filepath.Join(work, "SHA256SUMS")
	bundlePath := filepath.Join(work, "SHA256SUMS.sigstore.json")
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(bundlePath, bundle, 0600); err != nil {
		return err
	}
	identity := "https://github.com/Thuya-Myint/socklens/.github/workflows/release.yml@refs/tags/" + version
	_, err = u.runner().Output(ctx, Command{Program: program, Args: []string{"verify-blob", "--bundle", bundlePath, "--certificate-identity", identity, "--certificate-oidc-issuer", releaseIssuer, manifestPath}})
	if err != nil {
		return fmt.Errorf("release signature verification failed; installation unchanged: %w", err)
	}
	return nil
}
