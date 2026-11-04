package audit

import (
	"cmp"
	"github.com/Thuya-Myint/socklens/internal/platform"
	"slices"
)

func AddServices(report *Report, snapshot *platform.ServiceSnapshot) {
	if snapshot == nil {
		return
	}
	metadata := *snapshot
	metadata.Services = nil
	report.ServiceLookup = &metadata
	for i := range report.Listeners {
		listener := &report.Listeners[i]
		for _, service := range snapshot.Services {
			if slices.Contains(listener.PIDs, service.PID) {
				listener.Services = append(listener.Services, service)
			}
		}
		slices.SortFunc(listener.Services, func(a, b platform.Service) int {
			if order := cmp.Compare(a.PID, b.PID); order != 0 {
				return order
			}
			if order := cmp.Compare(a.Name, b.Name); order != 0 {
				return order
			}
			if order := cmp.Compare(a.DisplayName, b.DisplayName); order != 0 {
				return order
			}
			return cmp.Compare(a.State, b.State)
		})
		listener.Services = slices.Compact(listener.Services)
	}
}
