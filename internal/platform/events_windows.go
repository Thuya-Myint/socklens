//go:build windows

package platform

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var eventAdvapi = syscall.NewLazyDLL("advapi32.dll")
var startTrace = eventAdvapi.NewProc("StartTraceW")
var controlTrace = eventAdvapi.NewProc("ControlTraceW")
var enableTrace = eventAdvapi.NewProc("EnableTraceEx2")
var openTrace = eventAdvapi.NewProc("OpenTraceW")
var processTrace = eventAdvapi.NewProc("ProcessTrace")
var closeTrace = eventAdvapi.NewProc("CloseTrace")

// ETW structures from evntrace.h/evntcons.h. The supported Windows releases
// are amd64 and arm64, both using the 64-bit ABI (TRACEHANDLE is always 64-bit).
type etwWnode struct {
	BufferSize, ProviderID       uint32
	HistoricalContext, Timestamp uint64
	GUID                         syscall.GUID
	ClientContext, Flags         uint32
}

type etwProperties struct {
	Wnode                                                       etwWnode
	BufferSize, MinimumBuffers, MaximumBuffers, MaximumFileSize uint32
	LogFileMode, FlushTimer, EnableFlags, AgeLimit              uint32
	NumberOfBuffers, FreeBuffers, EventsLost, BuffersWritten    uint32
	LogBuffersLost, RealTimeBuffersLost                         uint32
	LoggerThreadID                                              uintptr
	LogFileNameOffset, LoggerNameOffset                         uint32
}

type etwCurrentEvent struct {
	Header                       [48]byte
	InstanceID, ParentInstanceID uint32
	ParentGUID                   syscall.GUID
	MofData                      uintptr
	MofLength, BufferContext     uint32
}

type etwLogHeader struct {
	BufferSize, Version, ProviderVersion, NumberOfProcessors      uint32
	EndTime                                                       uint64
	TimerResolution, MaximumFileSize, LogFileMode, BuffersWritten uint32
	LogInstanceGUID                                               syscall.GUID
	LoggerName, LogFileName                                       *uint16
	TimeZone                                                      [172]byte
	BootTime, PerfFreq, StartTime                                 uint64
	ReservedFlags, BuffersLost                                    uint32
}

type etwLogfile struct {
	LogFileName, LoggerName        *uint16
	CurrentTime                    uint64
	BuffersRead, ProcessTraceMode  uint32
	CurrentEvent                   etwCurrentEvent
	LogfileHeader                  etwLogHeader
	BufferCallback                 uintptr
	BufferSize, Filled, EventsLost uint32
	EventRecordCallback            uintptr
	IsKernelTrace                  uint32
	Context                        uintptr
}

type etwRecord struct {
	Header                              [80]byte
	BufferContext                       uint32
	ExtendedDataCount, UserDataLength   uint16
	ExtendedData, UserData, UserContext uintptr
}

var etwSources sync.Map
var etwSourceID atomic.Uintptr
var etwCallbackOnce sync.Once
var etwCallback uintptr

// The runtime cannot free syscall callbacks. Use one callback for all sessions,
// routing through an integer context rather than retaining a Go pointer in ETW.
func socketETWCallback() uintptr {
	etwCallbackOnce.Do(func() {
		etwCallback = syscall.NewCallback(func(record *etwRecord) uintptr {
			notifySocketETW(record)
			return 0
		})
	})
	return etwCallback
}

func notifySocketETW(record *etwRecord) {
	if value, ok := etwSources.Load(record.UserContext); ok {
		source := value.(*windowsSocketEvents)
		select {
		case source.changes <- struct{}{}:
		default:
		}
	}
}

type windowsSocketEvents struct {
	changes           chan struct{}
	done              chan struct{}
	stop              chan struct{}
	once              sync.Once
	err               error // published by closing done
	session, consumer uint64
	properties        []uint64 // aligned EVENT_TRACE_PROPERTIES plus UTF-16 session name
	name              []uint16
	log               etwLogfile
}

func (s *windowsSocketEvents) propertiesPointer() uintptr {
	return uintptr(unsafe.Pointer(&s.properties[0]))
}

func WatchSocketEvents(ctx context.Context) (SocketEvents, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return nil, fmt.Errorf("Windows ETW requires a 64-bit build: %w", ErrUnsupported)
	}
	for _, proc := range []*syscall.LazyProc{startTrace, controlTrace, enableTrace, openTrace, processTrace, closeTrace} {
		if err := proc.Find(); err != nil {
			return nil, fmt.Errorf("Windows ETW unavailable: %w", err)
		}
	}
	id := etwSourceID.Add(1)
	name, _ := syscall.UTF16FromString(fmt.Sprintf("SockLens-%d-%d", os.Getpid(), id))
	size := int(unsafe.Sizeof(etwProperties{}))
	s := &windowsSocketEvents{changes: make(chan struct{}, 1), done: make(chan struct{}), stop: make(chan struct{}), name: name}
	s.properties = make([]uint64, (size+len(name)*2+7)/8)
	props := (*etwProperties)(unsafe.Pointer(&s.properties[0]))
	props.Wnode.BufferSize = uint32(len(s.properties) * 8)
	props.Wnode.Flags = 0x20000   // WNODE_FLAG_TRACED_GUID
	props.Wnode.ClientContext = 1 // performance counter timestamps
	props.BufferSize, props.MinimumBuffers, props.MaximumBuffers = 64, 2, 32
	props.LogFileMode = 0x100 // EVENT_TRACE_REAL_TIME_MODE
	props.FlushTimer = 1      // ETW buffer delivery latency; no socket-table polling
	props.LoggerNameOffset = uint32(size)
	copy(unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(&s.properties[0]), size)), len(name)), name)
	code, _, _ := startTrace.Call(uintptr(unsafe.Pointer(&s.session)), uintptr(unsafe.Pointer(&name[0])), s.propertiesPointer())
	if code != 0 {
		return nil, fmt.Errorf("start Windows ETW session (administrator or tracing permissions required): %w", syscall.Errno(code))
	}
	// Enable both lifecycle (AFD bind/listen/close) and TCP/IP network events.
	// A private, uniquely named session is stopped on every failure and Close;
	// existing system/user sessions are never modified or adopted.
	providers := []syscall.GUID{
		{Data1: 0xe53c6823, Data2: 0x7bb8, Data3: 0x44bb, Data4: [8]byte{0x90, 0xdc, 0x3f, 0x86, 0x09, 0x0d, 0x48, 0xa6}},
		{Data1: 0x7dd42a49, Data2: 0x5329, Data3: 0x4832, Data4: [8]byte{0x8d, 0xfd, 0x43, 0xd9, 0x79, 0x15, 0x3a, 0x88}},
	}
	stopSession := func() { controlTrace.Call(uintptr(s.session), 0, s.propertiesPointer(), 1) }
	for i := range providers {
		code, _, _ = enableTrace.Call(uintptr(s.session), uintptr(unsafe.Pointer(&providers[i])), 1, 5, 0, 0, 0, 0)
		if code != 0 {
			stopSession()
			return nil, fmt.Errorf("enable Windows socket ETW provider: %w", syscall.Errno(code))
		}
	}
	s.log.LoggerName = &s.name[0]
	s.log.ProcessTraceMode = 0x10000100 // EVENT_RECORD | REAL_TIME
	s.log.EventRecordCallback = socketETWCallback()
	s.log.Context = id
	consumer, _, openErr := openTrace.Call(uintptr(unsafe.Pointer(&s.log)))
	if consumer == ^uintptr(0) {
		stopSession()
		return nil, fmt.Errorf("open Windows ETW consumer: %w", openErr)
	}
	s.consumer = uint64(consumer)
	etwSources.Store(id, s)
	go func() {
		code, _, _ := processTrace.Call(uintptr(unsafe.Pointer(&s.consumer)), 1, 0, 0)
		if code == 0 {
			s.err = fmt.Errorf("Windows socket ETW consumer stopped")
		} else {
			s.err = fmt.Errorf("Windows socket ETW consumer stopped: %w", syscall.Errno(code))
		}
		etwSources.Delete(id)
		close(s.done)
		runtime.KeepAlive(s)
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-s.stop:
		}
	}()
	return s, nil
}

func (s *windowsSocketEvents) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return s.err
	case <-s.changes:
		return nil
	}
}

func (s *windowsSocketEvents) Close() error {
	s.once.Do(func() {
		close(s.stop)
		controlTrace.Call(uintptr(s.session), 0, s.propertiesPointer(), 1) // STOP
		closeTrace.Call(uintptr(s.consumer))
	})
	<-s.done
	return nil
}
