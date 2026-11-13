package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSignatureFailureStopsBeforeArchiveAndExecution(t *testing.T) {
	for _, missing := range []bool{false, true} {
		target, old := makeStandalone(t)
		image := currentTestImage(t)
		runner := fakeBinaryRunner(target, image)
		output := runner.output
		verified := false
		runner.output = func(c Command) ([]byte, error) {
			if c.Program == "fixture-cosign" {
				verified = true
				return nil, errors.New("signature rejected")
			}
			if c.Program != target {
				t.Fatal("executed release before verifying signature")
			}
			return output(c)
		}
		updater := installerFixture(t, target, nil, runtime.GOARCH, runner, "", func() { t.Error("downloaded archive before verifying signature") })
		if missing {
			updater.Lookup = func(string) (string, error) { return "", errors.New("not found") }
		}
		plan, err := updater.Check(context.Background(), "v0.3.0")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := updater.Apply(context.Background(), plan, Streams{}); err == nil {
			t.Fatal("unsigned update accepted")
		}
		data, err := os.ReadFile(target)
		if err != nil || string(data) != string(old) || verified == missing {
			t.Fatalf("original changed or verifier bypassed: %v", err)
		}
	}
}

func TestSignaturePolicyPinsExactIdentityAndArtifact(t *testing.T) {
	work := t.TempDir()
	runner := &installRunner{output: func(c Command) ([]byte, error) {
		want := Command{Program: "trusted-cosign", Args: []string{"verify-blob", "--bundle", filepath.Join(work, "SHA256SUMS.sigstore.json"), "--certificate-identity", "https://github.com/Thuya-Myint/socklens/.github/workflows/release.yml@refs/tags/v0.11.0", "--certificate-oidc-issuer", releaseIssuer, filepath.Join(work, "SHA256SUMS")}}
		if !reflect.DeepEqual(c, want) {
			t.Fatalf("unexpected verification policy: %+v", c)
		}
		for name, want := range map[string]string{"SHA256SUMS": "manifest", "SHA256SUMS.sigstore.json": "bundle"} {
			data, err := os.ReadFile(filepath.Join(work, name))
			if err != nil || string(data) != want {
				t.Fatalf("wrong artifact: %s %v", data, err)
			}
		}
		return nil, nil
	}}
	u := Updater{Runner: runner, Lookup: func(string) (string, error) { return "trusted-cosign", nil }}
	if err := u.verifyReleaseManifest(context.Background(), "v0.11.0", work, []byte("manifest"), []byte("bundle")); err != nil {
		t.Fatal(err)
	}
}
