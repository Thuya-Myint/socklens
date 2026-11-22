package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

const snapshotVersion = 1
const maxSnapshotBytes = 32 << 20

type savedSnapshot struct {
	Version   int               `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Sockets   []platform.Socket `json:"sockets"`
}

func snapshotCommand(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return fail(errOut, fmt.Errorf("usage: socklens snapshot save [filters] FILE | snapshot diff [--json] BEFORE AFTER"))
	}
	switch args[0] {
	case "save":
		return snapshotSave(args[1:], out, errOut, platform.List)
	case "diff":
		return snapshotDiff(args[1:], out, errOut)
	default:
		return fail(errOut, fmt.Errorf("unknown snapshot subcommand %q; use save or diff", args[0]))
	}
}

func snapshotSave(args []string, out, errOut io.Writer, list func() ([]platform.Socket, error)) int {
	fs := newCommandFlags("snapshot-save", errOut)
	opts := addSocketFlags(fs)
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 1 {
		return fail(errOut, fmt.Errorf("usage: socklens snapshot save [filters] FILE (flags before FILE)"))
	}
	if err := opts.validateFiltersOnly(fs); err != nil {
		return fail(errOut, err)
	}
	if opts.json {
		return fail(errOut, fmt.Errorf("--json is not supported for snapshot save"))
	}
	rows, err := list()
	if err != nil {
		return fail(errOut, err)
	}
	rows, err = filterSockets(rows, opts.filter)
	if err != nil {
		return fail(errOut, err)
	}
	record := savedSnapshot{Version: snapshotVersion, Timestamp: time.Now().UTC(), Sockets: cloneSockets(rows)}
	if err := saveSnapshotFile(fs.Arg(0), record); err != nil {
		return fail(errOut, err)
	}
	_, err = fmt.Fprintf(out, "Saved %d sockets to %s\n", len(rows), fs.Arg(0))
	if err != nil {
		return fail(errOut, err)
	}
	return 0
}

func (opts *socketOptions) validateFiltersOnly(fs *flag.FlagSet) error {
	// Reuse the standard filter validation without passing the output path as a positional argument.
	copyFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	copyFlags.SetOutput(io.Discard)
	copyOpts := addSocketFlags(copyFlags)
	*copyOpts = *opts
	fs.Visit(func(f *flag.Flag) {
		if target := copyFlags.Lookup(f.Name); target != nil {
			_ = target.Value.Set(f.Value.String())
		}
	})
	return copyOpts.validate(copyFlags)
}

func saveSnapshotFile(path string, record savedSnapshot) error {
	if path == "" {
		return fmt.Errorf("snapshot path cannot be empty")
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > maxSnapshotBytes {
		return fmt.Errorf("snapshot exceeds 32 MiB limit")
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".socklens-snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Link publishes the complete file atomically and refuses any existing
	// destination, including dangling symlinks and concurrent creators. Rename
	// would replace an existing file on Unix. Fail safely on filesystems that
	// do not support hard links rather than falling back to an unsafe rename.
	if err := os.Link(file.Name(), path); err != nil {
		return fmt.Errorf("publish snapshot without overwriting (requires hard-link support): %w", err)
	}
	return nil
}

func readSnapshotFile(path string) (savedSnapshot, error) {
	var record savedSnapshot
	file, err := os.Open(path)
	if err != nil {
		return record, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return record, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSnapshotBytes {
		return record, fmt.Errorf("snapshot must be a regular file of at most 32 MiB")
	}
	dec := json.NewDecoder(io.LimitReader(file, maxSnapshotBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&record); err != nil {
		return record, fmt.Errorf("invalid snapshot: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return record, fmt.Errorf("snapshot contains trailing data")
	}
	if record.Version != snapshotVersion || record.Timestamp.IsZero() || record.Sockets == nil {
		return record, fmt.Errorf("unsupported or incomplete snapshot (expected version %d)", snapshotVersion)
	}
	return record, nil
}

func snapshotDiff(args []string, out, errOut io.Writer) int {
	fs := newCommandFlags("snapshot-diff", errOut)
	asJSON := fs.Bool("json", false, "output JSON change record")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 2 {
		return fail(errOut, fmt.Errorf("usage: socklens snapshot diff [--json] BEFORE AFTER"))
	}
	before, err := readSnapshotFile(fs.Arg(0))
	if err != nil {
		return fail(errOut, fmt.Errorf("read before: %w", err))
	}
	after, err := readSnapshotFile(fs.Arg(1))
	if err != nil {
		return fail(errOut, fmt.Errorf("read after: %w", err))
	}
	delta := diffSockets(before.Sockets, after.Sockets)
	delta.Timestamp = after.Timestamp
	if err := writeChanges(out, delta, *asJSON); err != nil {
		return fail(errOut, err)
	}
	return 0
}
