package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Minimal PE32+ fixture with a certificate-table entry, not a trusted signature.
// Actual cryptographic verification belongs to the Windows workflow gate.
func windowsFixture(machine uint16, signed bool) []byte {
	b := make([]byte, 544)
	copy(b, "MZ")
	binary.LittleEndian.PutUint32(b[60:], 128)
	copy(b[128:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(b[132:], machine)
	binary.LittleEndian.PutUint16(b[148:], 240)
	binary.LittleEndian.PutUint16(b[152:], 0x20b)
	binary.LittleEndian.PutUint32(b[260:], 16)
	if signed {
		binary.LittleEndian.PutUint32(b[296:], 512)
		binary.LittleEndian.PutUint32(b[300:], 32)
		copy(b[512:], "signed-input-bytes")
	}
	return b
}

func TestWindowsInputsPreservedInArchivesAndHashes(t *testing.T) {
	opts := testSettings(t)
	opts.windowsBinaries = t.TempDir()
	for arch, machine := range map[string]uint16{"amd64": 0x8664, "arm64": 0xaa64} {
		if err := os.WriteFile(filepath.Join(opts.windowsBinaries, "windows_"+arch+".exe"), windowsFixture(machine, true), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := packageRelease(opts, func(target buildTarget, path string) error {
		if target.OS == "windows" {
			t.Fatal("signed Windows input was rebuilt")
		}
		return fakeBuild(target, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(opts.out, opts.version, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		name := "socklens_" + opts.version + "_windows_" + arch
		path := filepath.Join(opts.out, opts.version, name+".zip")
		contents := readArchive(t, path)
		want, _ := os.ReadFile(filepath.Join(opts.windowsBinaries, "windows_"+arch+".exe"))
		if !bytes.Equal(contents[name+"/socklens.exe"], want) {
			t.Fatal("signed bytes changed")
		}
		archive, _ := os.ReadFile(path)
		if !strings.Contains(string(sums), fmt.Sprintf("%x  %s.zip", sha256.Sum256(archive), name)) {
			t.Fatal("manifest does not hash final signed archive")
		}
	}
}

func TestWindowsInputRejectsMissingUnsignedWrongArchitecture(t *testing.T) {
	for _, kind := range []string{"missing", "unsigned", "wrong-arch", "invalid-pe"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			data := windowsFixture(0x8664, true)
			switch kind {
			case "unsigned":
				data = windowsFixture(0x8664, false)
			case "wrong-arch":
				data = windowsFixture(0xaa64, true)
			case "invalid-pe":
				data = []byte("not a PE")
			}
			if kind != "missing" {
				if err := os.WriteFile(filepath.Join(directory, "windows_amd64.exe"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := copyWindowsBinary(directory, buildTarget{"windows", "amd64"}, filepath.Join(directory, "out.exe")); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}
