package audit

import (
	"github.com/Thuya-Myint/socklens/internal/platform"
	"testing"
	"time"
)

func TestServiceMetadataAssociation(t *testing.T) {
	report, err := Analyze([]platform.Socket{{Protocol: "udp", Local: "127.0.0.1:60817", State: "BOUND", PIDs: []int{8100}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &platform.ServiceSnapshot{Status: "read", Source: "windows-scm", Detail: "best effort", Services: []platform.Service{{PID: 99, Name: "unrelated"}, {PID: 8100, Name: "z", State: "running", ProcessType: "shared-host"}, {PID: 8100, Name: "a", State: "running", ProcessType: "shared-host"}, {PID: 8100, Name: "a", State: "running", ProcessType: "shared-host"}}}
	AddServices(&report, snapshot)
	if len(report.Listeners[0].Services) != 2 || report.Listeners[0].Services[0].Name != "a" || report.ServiceLookup == nil {
		t.Fatal(report)
	}
	report.ServiceLookup.Detail = "changed"
	report.Listeners[0].Services[0].Name = "changed"
	if snapshot.Detail != "best effort" || snapshot.Services[2].Name != "a" {
		t.Fatal("report mutation changed source observations")
	}
}
