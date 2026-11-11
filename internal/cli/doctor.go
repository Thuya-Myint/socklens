package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

// doctor reports capabilities without invoking privileged tracing setup.
func doctorCommand(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := newCommandFlags("doctor", errOut)
	asJSON := fs.Bool("json", false, "output machine-readable capability report")
	if err := parseCommandFlags(fs, args, errOut); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		return fail(errOut, fmt.Errorf("doctor takes no arguments"))
	}
	report := struct {
		OS              string   `json:"os"`
		Arch            string   `json:"arch"`
		SocketDiscovery string   `json:"socket_discovery"`
		SocketCount     int      `json:"socket_count"`
		CollectionMS    float64  `json:"collection_ms"`
		EventBackend    string   `json:"event_backend"`
		EventStatus     string   `json:"event_status"`
		Notes           []string `json:"notes"`
	}{OS: runtime.GOOS, Arch: runtime.GOARCH, EventStatus: "requires runtime verification", Notes: []string{}}
	started := time.Now()
	rows, err := platform.List()
	report.CollectionMS = float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		report.SocketDiscovery = err.Error()
	} else {
		report.SocketDiscovery = "available"
		report.SocketCount = len(rows)
	}
	switch runtime.GOOS {
	case "linux":
		report.EventBackend = "bpftrace"
		if _, e := exec.LookPath("bpftrace"); e != nil {
			report.EventStatus = "helper missing"
		} else if os.Geteuid() != 0 {
			report.EventStatus = "helper installed; tracing permissions not verified"
		} else {
			report.EventStatus = "helper installed; tracing attachment not verified"
		}
	case "darwin":
		report.EventBackend = "dtrace"
		report.EventStatus = "DTrace privileges and SIP restrictions must be checked at runtime"
	case "windows":
		report.EventBackend = "ETW"
		report.EventStatus = "ETW session permissions must be checked at runtime"
	default:
		report.EventBackend = "unsupported"
		report.EventStatus = "unsupported"
	}
	if ctx.Err() != nil {
		return 0
	}
	if *asJSON {
		if e := json.NewEncoder(out).Encode(report); e != nil {
			return fail(errOut, e)
		}
	} else {
		fmt.Fprintf(out, "SockLens doctor (%s/%s)\nSocket discovery: %s\nSockets: %d\nCollection: %.2f ms\nEvent backend: %s\nEvent status: %s\n", report.OS, report.Arch, report.SocketDiscovery, report.SocketCount, report.CollectionMS, report.EventBackend, report.EventStatus)
	}
	if err != nil {
		return 1
	}
	return 0
}
