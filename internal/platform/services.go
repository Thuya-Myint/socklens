package platform

// Service is an observed Windows service associated with a process ID. A shared
// host can contain several services; this does not attribute a socket to one.
type Service struct {
	PID         int    `json:"pid"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	ProcessType string `json:"process_type"`
	Source      string `json:"source"`
}
type ServiceSnapshot struct {
	Status   string    `json:"status"`
	Source   string    `json:"source"`
	Detail   string    `json:"detail"`
	Services []Service `json:"-"`
}
