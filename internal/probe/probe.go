package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/Thuya-Myint/socklens/internal/platform"
)

type Address struct {
	IP     string `json:"ip"`
	Family string `json:"family"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Result struct {
	Address        string  `json:"address"`
	Family         string  `json:"family"`
	Port           uint16  `json:"port"`
	Endpoint       string  `json:"endpoint"`
	Status         string  `json:"status"`
	Attempted      bool    `json:"attempted"`
	DurationMS     float64 `json:"duration_ms"`
	LocalEndpoint  string  `json:"local_endpoint,omitempty"`
	RemoteEndpoint string  `json:"remote_endpoint,omitempty"`
	Cause          string  `json:"cause,omitempty"`
	Message        string  `json:"message"`
}
type Summary struct {
	Planned       int `json:"planned"`
	Attempted     int `json:"attempted"`
	Connected     int `json:"connected"`
	Refused       int `json:"refused"`
	TimedOut      int `json:"timed_out"`
	NetworkErrors int `json:"network_errors"`
	Cancelled     int `json:"cancelled"`
	NotAttempted  int `json:"not_attempted"`
}
type Report struct {
	SchemaVersion    int       `json:"schema_version"`
	Timestamp        time.Time `json:"timestamp"`
	Host             string    `json:"host"`
	Protocol         string    `json:"protocol"`
	Family           string    `json:"family"`
	Ports            []uint16  `json:"ports"`
	Addresses        []Address `json:"addresses"`
	ConnectTimeoutMS int64     `json:"connect_timeout_ms"`
	TimeoutMS        int64     `json:"timeout_ms"`
	Concurrency      int       `json:"concurrency"`
	DurationMS       float64   `json:"duration_ms"`
	Status           string    `json:"status"`
	Summary          Summary   `json:"summary"`
	Error            *Failure  `json:"error,omitempty"`
	Results          []Result  `json:"results"`
	Notes            []string  `json:"notes"`
}

// Engine dependencies are injectable for tests. Production resolves a hostname
// once and dials numeric endpoints, preventing a second DNS lookup per attempt.
type Engine struct {
	LookupIP func(context.Context, string) ([]net.IPAddr, error)
	Dial     func(context.Context, string, string) (net.Conn, error)
}

func (engine Engine) Run(parent context.Context, options Options) (Report, error) {
	options, err := options.Normalize()
	if err != nil {
		return Report{}, err
	}
	started := time.Now()
	report := Report{SchemaVersion: 1, Timestamp: started.UTC(), Host: options.Host, Protocol: "tcp", Family: options.Family, Ports: options.Ports, Addresses: []Address{}, ConnectTimeoutMS: options.ConnectTimeout.Milliseconds(), TimeoutMS: options.Timeout.Milliseconds(), Concurrency: options.Concurrency, Status: "completed", Results: []Result{}, Notes: []string{
		"TCP reachability from this machine at the observation time; no application data is sent.",
		"Connected does not establish application health, service identity, authentication, TLS, or vulnerability status.",
		"Refusal may come from the target or an intermediary. A timeout does not identify a firewall or establish a closed port.",
		"A hostname is resolved once; each selected numeric IP/port is checked separately. DNS answers may later change.",
	}}
	finish := func() (Report, error) { report.DurationMS = elapsedMS(started); return report, nil }
	ctx, cancel := context.WithTimeout(parent, options.Timeout)
	defer cancel()
	if ctx.Err() != nil {
		stopReport(&report, ctx.Err(), "before resolution")
		return finish()
	}
	lookup := engine.LookupIP
	if lookup == nil {
		lookup = net.DefaultResolver.LookupIPAddr
	}
	addresses, err := resolve(ctx, options.Host, options.Family, lookup)
	if ctx.Err() != nil {
		stopReport(&report, ctx.Err(), "during resolution")
		return finish()
	}
	if err != nil {
		report.Status = "resolution-error"
		report.Error = &Failure{Code: "resolution-error", Message: err.Error()}
		return finish()
	}
	if len(addresses) > MaxAddresses {
		report.Status = "target-error"
		report.Error = &Failure{Code: "target-limit", Message: fmt.Sprintf("host selected %d addresses; maximum is %d; use --family or an explicit IP", len(addresses), MaxAddresses)}
		return finish()
	}
	if len(addresses)*len(options.Ports) > MaxAttempts {
		report.Status = "target-error"
		report.Error = &Failure{Code: "target-limit", Message: fmt.Sprintf("selected addresses and ports exceed %d TCP attempts; select fewer ports or an explicit IP", MaxAttempts)}
		return finish()
	}
	for _, address := range addresses {
		report.Addresses = append(report.Addresses, Address{IP: address.String(), Family: addressFamily(address)})
	}
	for _, port := range options.Ports {
		for _, address := range addresses {
			report.Results = append(report.Results, Result{Address: address.String(), Family: addressFamily(address), Port: port, Endpoint: net.JoinHostPort(address.String(), strconv.Itoa(int(port))), Status: "not-attempted"})
		}
	}
	report.Summary.Planned = len(report.Results)
	dial := engine.Dial
	if dial == nil {
		dial = (&net.Dialer{KeepAlive: -1}).DialContext
	}
	jobs := make(chan int, len(report.Results))
	for i := range report.Results {
		jobs <- i
	}
	close(jobs)
	var workers sync.WaitGroup
	for worker := 0; worker < min(options.Concurrency, len(report.Results)); worker++ {
		workers.Go(func() {
			for index := range jobs {
				result := report.Results[index]
				if ctx.Err() != nil {
					result.Cause = "interrupted"
					result.Message = "Not attempted because the probe was interrupted."
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						result.Cause = "total-timeout"
						result.Message = "Not attempted because the total deadline expired."
					}
					report.Results[index] = result
					continue
				}
				result.Attempted = true
				attemptStart := time.Now()
				attemptCtx, stop := context.WithTimeout(ctx, options.ConnectTimeout)
				network := "tcp4"
				if result.Family == "ipv6" {
					network = "tcp6"
				}
				connection, dialErr := dial(attemptCtx, network, result.Endpoint)
				if connection != nil {
					// Never Read or Write application payloads. Release each connection even
					// when cancellation races with successful establishment.
					if dialErr == nil {
						if address := connection.LocalAddr(); address != nil {
							result.LocalEndpoint = address.String()
						}
						if address := connection.RemoteAddr(); address != nil {
							result.RemoteEndpoint = address.String()
						}
					}
					_ = connection.Close()
				}
				result.DurationMS = elapsedMS(attemptStart)
				switch {
				case ctx.Err() != nil:
					result.Status = "cancelled"
					result.Cause = "interrupted"
					result.Message = "Interrupted before the attempt result could be confirmed."
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						result.Status = "timed-out"
						result.Cause = "total-timeout"
						result.Message = "The total deadline expired during this attempt; cause of non-completion is unknown."
					}
				case errors.Is(attemptCtx.Err(), context.DeadlineExceeded):
					result.Status = "timed-out"
					result.Cause = "connect-timeout"
					result.Message = "Connection deadline expired; filtering, routing and other causes are not distinguished."
				case dialErr == nil && connection != nil:
					result.Status = "connected"
					result.Message = "TCP connection completed; no application data sent."
				default:
					if dialErr == nil {
						dialErr = fmt.Errorf("dialer returned no connection")
					}
					result.Status, result.Cause, result.Message = classify(dialErr)
				}
				stop()
				report.Results[index] = result
			}
		})
	}
	workers.Wait()
	for _, result := range report.Results {
		if result.Attempted {
			report.Summary.Attempted++
		}
		switch result.Status {
		case "connected":
			report.Summary.Connected++
		case "refused":
			report.Summary.Refused++
		case "timed-out":
			report.Summary.TimedOut++
		case "network-error":
			report.Summary.NetworkErrors++
		case "cancelled":
			report.Summary.Cancelled++
		case "not-attempted":
			report.Summary.NotAttempted++
		}
		if result.Cause == "total-timeout" {
			report.Status = "timed-out"
		}
		if result.Cause == "interrupted" {
			report.Status = "interrupted"
		}
	}
	return finish()
}
func resolve(ctx context.Context, host, family string, lookup func(context.Context, string) ([]net.IPAddr, error)) ([]netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address.Unmap()}, nil
	}
	values, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	addresses := []netip.Addr{}
	seen := map[netip.Addr]bool{}
	for _, value := range values {
		address, ok := netip.AddrFromSlice(value.IP)
		if !ok {
			return nil, fmt.Errorf("resolver returned an invalid IP address")
		}
		address = address.Unmap()
		if !familyMatches(address, family) {
			continue
		}
		if value.Zone != "" {
			if address.Is4() {
				return nil, fmt.Errorf("resolver returned a zone for an IPv4 address")
			}
			address = address.WithZone(value.Zone)
		}
		if !unicast(address) {
			return nil, fmt.Errorf("resolver selected a non-unicast or unspecified address; no connections attempted")
		}
		if !seen[address] {
			addresses = append(addresses, address)
			seen[address] = true
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no %s addresses returned for %s", family, host)
	}
	slices.SortFunc(addresses, func(a, b netip.Addr) int { return a.Compare(b) })
	return addresses, nil
}
func addressFamily(address netip.Addr) string {
	if address.Unmap().Is4() {
		return "ipv4"
	}
	return "ipv6"
}
func elapsedMS(start time.Time) float64 { return float64(time.Since(start).Microseconds()) / 1000 }
func stopReport(report *Report, err error, phase string) {
	report.Status = "interrupted"
	code := "interrupted"
	message := "Probe interrupted " + phase + "."
	if errors.Is(err, context.DeadlineExceeded) {
		report.Status = "timed-out"
		code = "total-timeout"
		message = "Total deadline expired " + phase + "; no endpoint results were established."
	}
	report.Error = &Failure{Code: code, Message: message}
}
func classify(err error) (status, cause, message string) {
	if platform.IsConnectionRefused(err) {
		return "refused", "connection-refused", "TCP connection explicitly refused by the target or an intermediary."
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return "timed-out", "network-timeout", "TCP connection timed out; cause is unknown."
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled", "interrupted", "TCP connection attempt was cancelled."
	}
	return "network-error", "network-error", "TCP connection could not be completed: " + err.Error()
}
func (report Report) ExitCode() int {
	switch report.Status {
	case "interrupted":
		return 130
	case "resolution-error", "target-error":
		return 1
	case "timed-out":
		return 2
	}
	if report.Summary.Planned == 0 || report.Summary.Connected != report.Summary.Planned {
		return 2
	}
	return 0
}
