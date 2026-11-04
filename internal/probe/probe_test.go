package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func testOptions() Options {
	return Options{Host: "127.0.0.1", Ports: []uint16{8080}, Family: "any", ConnectTimeout: 200 * time.Millisecond, Timeout: 2 * time.Second, Concurrency: 2}
}

type fakeConnection struct {
	net.Conn
	endpoint string
	closed   *atomic.Int32
}

func (c fakeConnection) Close() error { c.closed.Add(1); return nil }
func (c fakeConnection) LocalAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:30000"))
}
func (c fakeConnection) RemoteAddr() net.Addr {
	return net.TCPAddrFromAddrPort(netip.MustParseAddrPort(c.endpoint))
}
func (c fakeConnection) Read([]byte) (int, error)  { panic("probe must not read application data") }
func (c fakeConnection) Write([]byte) (int, error) { panic("probe must not write application data") }

func TestTargetsAndOptions(t *testing.T) {
	ports, err := ParsePorts("443,80,443, 8080")
	if err != nil || !reflect.DeepEqual(ports, []uint16{80, 443, 8080}) {
		t.Fatal(ports, err)
	}
	for _, value := range []string{"", "0", "65536", "-1", "+80", "80,", "80,,443", "http", "80-90"} {
		if _, err := ParsePorts(value); err == nil {
			t.Fatal("accepted invalid ports", value)
		}
	}
	for _, host := range []string{"https://example.com", "example.com:443", "127.0.0.1/24", "0.0.0.0", "::", "224.0.0.1", "255.255.255.255", "example..com", "-example.com", "example.com\x1b[31m", "host name"} {
		options := testOptions()
		options.Host = host
		if _, err := options.Normalize(); err == nil {
			t.Fatal("accepted invalid host", host)
		}
	}
	for _, host := range []string{"localhost", "example.com.", "127.0.0.1", "::1", "[::1]", "::ffff:127.0.0.1", "fe80::1%eth0"} {
		options := testOptions()
		options.Host = host
		if _, err := options.Normalize(); err != nil {
			t.Fatal(host, err)
		}
	}
	for _, change := range []func(*Options){func(o *Options) { o.Family = "udp" }, func(o *Options) { o.Family = "ipv6" }, func(o *Options) { o.ConnectTimeout = 0 }, func(o *Options) { o.ConnectTimeout = time.Minute }, func(o *Options) { o.Timeout = 0 }, func(o *Options) { o.Timeout = 6 * time.Minute }, func(o *Options) { o.Concurrency = 0 }, func(o *Options) { o.Concurrency = 33 }, func(o *Options) { o.Ports = nil }, func(o *Options) { o.Ports = []uint16{0} }} {
		options := testOptions()
		change(&options)
		if _, err := options.Normalize(); err == nil {
			t.Fatal("accepted invalid configuration", options)
		}
	}
	many := []string{}
	for n := 1; n <= 65; n++ {
		many = append(many, strconv.Itoa(n))
	}
	if _, err := ParsePorts(strings.Join(many, ",")); err == nil {
		t.Fatal("unbounded port list")
	}
}
func TestResolveOncePinnedAddressesAndConcurrency(t *testing.T) {
	options := testOptions()
	options.Host = "example.test"
	options.Ports = []uint16{443, 80, 443}
	original := append([]uint16{}, options.Ports...)
	var lookups, active, peak, closed atomic.Int32
	var gateOnce sync.Once
	gate := make(chan struct{})
	engine := Engine{LookupIP: func(ctx context.Context, host string) ([]net.IPAddr, error) {
		lookups.Add(1)
		if host != "example.test" {
			t.Fatal(host)
		}
		return []net.IPAddr{{IP: net.ParseIP("::1")}, {IP: net.ParseIP("127.0.0.1")}, {IP: net.ParseIP("::ffff:127.0.0.1")}}, nil
	}, Dial: func(ctx context.Context, network, endpoint string) (net.Conn, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if previous >= current || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		if current == 2 {
			gateOnce.Do(func() { close(gate) })
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		ip, err := netip.ParseAddrPort(endpoint)
		if err != nil {
			t.Error("dialed hostname instead of numeric IP", endpoint)
		}
		if ip.Addr().Is4() && network != "tcp4" || ip.Addr().Is6() && network != "tcp6" {
			t.Error("wrong address family", network, endpoint)
		}
		return fakeConnection{endpoint: endpoint, closed: &closed}, nil
	}}
	report, err := engine.Run(context.Background(), options)
	if err != nil || report.ExitCode() != 0 || lookups.Load() != 1 || peak.Load() != 2 || closed.Load() != 4 {
		t.Fatalf("%+v err=%v lookups=%d peak=%d closed=%d", report, err, lookups.Load(), peak.Load(), closed.Load())
	}
	if !reflect.DeepEqual(options.Ports, original) {
		t.Fatal("caller options were mutated")
	}
	want := []string{"127.0.0.1:80", "[::1]:80", "127.0.0.1:443", "[::1]:443"}
	for i, result := range report.Results {
		if result.Endpoint != want[i] || result.Status != "connected" || !result.Attempted || result.LocalEndpoint == "" || result.RemoteEndpoint != result.Endpoint {
			t.Fatal(result)
		}
	}
}
func TestPlanningFailuresNeverDial(t *testing.T) {
	for _, name := range []string{"dns-error", "family-empty", "too-many-addresses", "too-many-attempts", "invalid-answer"} {
		t.Run(name, func(t *testing.T) {
			options := testOptions()
			options.Host = "example.test"
			calls := 0
			engine := Engine{Dial: func(context.Context, string, string) (net.Conn, error) {
				calls++
				return nil, errors.New("must not dial")
			}, LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
				switch name {
				case "dns-error":
					return nil, &net.DNSError{Name: "example.test", IsNotFound: true, Err: "no such host"}
				case "family-empty":
					return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
				case "invalid-answer":
					return []net.IPAddr{{IP: net.ParseIP("0.0.0.0")}}, nil
				default:
					count := 17
					if name == "too-many-attempts" {
						count = 5
					}
					values := []net.IPAddr{}
					for n := 1; n <= count; n++ {
						values = append(values, net.IPAddr{IP: net.ParseIP(fmt.Sprintf("192.0.2.%d", n))})
					}
					return values, nil
				}
			}}
			if name == "family-empty" {
				options.Family = "ipv6"
			}
			if name == "too-many-attempts" {
				options.Ports = nil
				for n := 1; n <= 64; n++ {
					options.Ports = append(options.Ports, uint16(n))
				}
			}
			report, err := engine.Run(context.Background(), options)
			if err != nil || report.ExitCode() != 1 || report.Error == nil || len(report.Results) != 0 || calls != 0 {
				t.Fatal(report, err, calls)
			}
		})
	}
}
func TestFailureClassificationAndConnectionCleanup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status string
	}{
		{"refused", &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, "refused"},
		{"timeout", &net.DNSError{Err: "i/o timeout", IsTimeout: true}, "timed-out"},
		{"permission", os.NewSyscallError("connect", syscall.EACCES), "network-error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var closed atomic.Int32
			engine := Engine{Dial: func(_ context.Context, _, endpoint string) (net.Conn, error) {
				return fakeConnection{endpoint: endpoint, closed: &closed}, tc.err
			}}
			report, err := engine.Run(context.Background(), testOptions())
			if err != nil || report.Results[0].Status != tc.status || report.ExitCode() != 2 || closed.Load() != 1 {
				t.Fatal(report, err, closed.Load())
			}
			if strings.Contains(report.Results[0].Message, "port is closed") || strings.Contains(report.Results[0].Message, "firewall blocked") {
				t.Fatal("unsupported interpretation")
			}
		})
	}
}
func TestDeadlinesAndQueuedWork(t *testing.T) {
	options := testOptions()
	options.Timeout = time.Second
	options.ConnectTimeout = 100 * time.Millisecond
	options.Concurrency = 1
	options.Ports = nil
	for n := 1; n <= 32; n++ {
		options.Ports = append(options.Ports, uint16(10000+n))
	}
	var calls atomic.Int32
	engine := Engine{Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		calls.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	started := time.Now()
	report, err := engine.Run(context.Background(), options)
	if err != nil || report.Status != "timed-out" || report.ExitCode() != 2 || report.Summary.NotAttempted == 0 || report.Summary.TimedOut == 0 || report.Summary.Attempted+report.Summary.NotAttempted != 32 {
		t.Fatal(report, err)
	}
	if time.Since(started) > 3*time.Second || calls.Load() > 12 {
		t.Fatal("total deadline/concurrency not enforced", calls.Load())
	}
	for _, result := range report.Results {
		if !result.Attempted && result.Cause != "total-timeout" {
			t.Fatal(result)
		}
	}
}
func TestCancellationBeforeDNSDuringDNSAndAfterConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	options := testOptions()
	options.Host = "example.test"
	engine := Engine{LookupIP: func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("lookup on pre-cancelled probe")
		return nil, nil
	}}
	report, err := engine.Run(ctx, options)
	if err != nil || report.ExitCode() != 130 || len(report.Results) != 0 {
		t.Fatal(report, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan Report, 1)
	engine = Engine{LookupIP: func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	go func() { report, _ := engine.Run(ctx, options); done <- report }()
	<-started
	cancel()
	select {
	case report = <-done:
		if report.Status != "interrupted" || report.ExitCode() != 130 {
			t.Fatal(report)
		}
	case <-time.After(time.Second):
		t.Fatal("DNS cancellation hung")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	var closed atomic.Int32
	options = testOptions()
	options.Ports = []uint16{80, 443}
	options.Concurrency = 1
	engine = Engine{Dial: func(_ context.Context, _, endpoint string) (net.Conn, error) {
		cancel()
		return fakeConnection{endpoint: endpoint, closed: &closed}, nil
	}}
	report, err = engine.Run(ctx, options)
	if err != nil || report.ExitCode() != 130 || report.Summary.Connected != 0 || report.Summary.Cancelled != 1 || report.Summary.NotAttempted != 1 || closed.Load() != 1 {
		t.Fatal(report, err, closed.Load())
	}
}
func TestNativeTCPConnectAndRefusal(t *testing.T) {
	for _, tc := range []struct{ network, host, address string }{{"tcp4", "127.0.0.1", "127.0.0.1:0"}, {"tcp6", "::1", "[::1]:0"}} {
		t.Run(tc.network, func(t *testing.T) {
			listener, err := net.Listen(tc.network, tc.address)
			if err != nil {
				if tc.network == "tcp6" {
					t.Skipf("IPv6 unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer listener.Close()
			_, portText, _ := net.SplitHostPort(listener.Addr().String())
			port, _ := strconv.Atoi(portText)
			observed := make(chan error, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					observed <- err
					return
				}
				defer connection.Close()
				_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
				var data [1]byte
				n, err := connection.Read(data[:])
				if n > 0 {
					observed <- fmt.Errorf("application payload was sent")
					return
				}
				var timeout net.Error
				if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
					observed <- fmt.Errorf("connection not closed after probe")
					return
				}
				observed <- nil
			}()
			options := testOptions()
			options.ConnectTimeout = time.Second
			options.Timeout = 5 * time.Second
			options.Host = tc.host
			options.Ports = []uint16{uint16(port)}
			report, err := (Engine{}).Run(context.Background(), options)
			if err != nil || report.ExitCode() != 0 || report.Results[0].Endpoint != listener.Addr().String() || report.Results[0].RemoteEndpoint != listener.Addr().String() || report.Results[0].LocalEndpoint == "" {
				t.Fatal(report, err)
			}
			select {
			case err := <-observed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("listener did not observe connection closure")
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			report, err = (Engine{}).Run(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			if report.Summary.Connected > 0 {
				t.Skip("closed port was reused before the refusal check")
			}
			if report.Results[0].Status != "refused" || report.ExitCode() != 2 {
				t.Fatal(report)
			}
		})
	}
}
