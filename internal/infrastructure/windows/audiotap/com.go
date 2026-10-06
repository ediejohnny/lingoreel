//go:build windows

package audiotap

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WASAPI by hand, as go-webview2 does for WebView2: a COM object is a
// pointer to its table of methods, called with the object first.

var (
	ole32    = windows.NewLazySystemDLL("ole32.dll")
	mmdevapi = windows.NewLazySystemDLL("mmdevapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procActivateAudioInterfaceAsync = mmdevapi.NewProc("ActivateAudioInterfaceAsync")
	procQueryPerformanceCounter     = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency   = kernel32.NewProc("QueryPerformanceFrequency")
)

func guid(text string) windows.GUID {
	id, err := windows.GUIDFromString(text)
	if err != nil {
		panic(err)
	}
	return id
}

var (
	iidUnknown              = guid("{00000000-0000-0000-C000-000000000046}")
	iidAgileObject          = guid("{94EA2B94-E9CC-49E0-C0FF-EE64CA8F5B90}")
	iidCompletionHandler    = guid("{41D949AB-9862-444A-80F6-C261334DA5EB}")
	iidAudioClient          = guid("{1CB9AD4C-DBFA-4C32-B178-C2F568A703B2}")
	iidAudioCaptureClient   = guid("{C8ADBD64-E71E-48A0-A4DE-185C395CD317}")
	iidMMDeviceEnumerator   = guid("{A95664D2-9614-4F35-A746-DE8DB63617E6}")
	clsidMMDeviceEnumerator = guid("{BCDE0395-E52F-467C-8E3D-C4579291692E}")
)

// The methods used, by their place in each interface's table.
const (
	methodRelease = 2

	clientInitialize     = 3
	clientStart          = 10
	clientStop           = 11
	clientSetEventHandle = 13
	clientGetService     = 14

	captureGetBuffer         = 3
	captureReleaseBuffer     = 4
	captureGetNextPacketSize = 5

	enumeratorGetDefaultAudioEndpoint = 4
	deviceActivate                    = 3
	operationGetActivateResult        = 3
)

const (
	clsctxAll       = 0x17
	eRender         = 0
	eConsole        = 0
	vtBlob          = 65
	eNoInterface    = 0x80004002
	sFalse          = 1
	waveFloat       = 3
	shareModeShared = 0

	streamLoopback      = 0x00020000
	streamEventCallback = 0x00040000
	streamAutoConvert   = 0x80000000
	streamSRCDefault    = 0x08000000

	bufferSilent         = 0x2
	bufferTimestampError = 0x4

	activationProcessLoopback  = 1
	loopbackIncludeProcessTree = 0
)

// HRESULT is a failed COM call's code, which is what a probe needs to see.
type HRESULT uint32

func (h HRESULT) Error() string { return fmt.Sprintf("HRESULT 0x%08X", uint32(h)) }

func failed(r uintptr) error {
	if int32(r) < 0 {
		return HRESULT(r)
	}
	return nil
}

// object is the start of every COM object.
type object struct{ methods *[16]uintptr }

func (o *object) call(method int, args ...uintptr) error {
	r, _, _ := syscall.SyscallN(o.methods[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return failed(r)
}

func (o *object) release() {
	if o != nil {
		syscall.SyscallN(o.methods[methodRelease], uintptr(unsafe.Pointer(o)))
	}
}

// initCOM joins the calling thread, locked to it, to the multithreaded
// apartment WASAPI's objects live in.
func initCOM() (func(), error) {
	runtime.LockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil && !errors.Is(err, syscall.Errno(sFalse)) {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("joining COM: %w", err)
	}
	return func() { windows.CoUninitialize(); runtime.UnlockOSThread() }, nil
}

// activation is the completion handler ActivateAudioInterfaceAsync calls
// back on a thread of its own. Windows keeps a pointer to it, so a handler
// whose answer never came is never freed.
type activation struct {
	methods *[4]uintptr
	done    chan struct{}
	client  *object
	err     error
}

var (
	activations       sync.Map // the handler's address → *activation
	activationMethods = [4]uintptr{
		syscall.NewCallback(func(this uintptr, iid *windows.GUID, out *uintptr) uintptr {
			if *iid == iidUnknown || *iid == iidCompletionHandler || *iid == iidAgileObject {
				*out = this
				return 0
			}
			*out = 0
			return eNoInterface
		}),
		syscall.NewCallback(func(uintptr) uintptr { return 1 }), // the handler lives in Go's memory
		syscall.NewCallback(func(uintptr) uintptr { return 1 }),
		syscall.NewCallback(func(this uintptr, operation *object) uintptr {
			found, ok := activations.Load(this)
			if !ok {
				return 0
			}
			a := found.(*activation)
			var result int32
			var client *object
			a.err = operation.call(operationGetActivateResult, uintptr(unsafe.Pointer(&result)), uintptr(unsafe.Pointer(&client)))
			if a.err == nil {
				a.err = failed(uintptr(uint32(result)))
			}
			a.client = client
			close(a.done)
			return 0
		}),
	}
)

// activationParams is AUDIOCLIENT_ACTIVATION_PARAMS for a process.
type activationParams struct {
	activationType uint32
	processID      uint32
	loopbackMode   uint32
}

// blobVariant is a PROPVARIANT holding a BLOB.
type blobVariant struct {
	vt         uint16
	_, _, _    uint16
	size       uint32
	_          uint32
	activation *activationParams
}

// activationTimeout bounds the wait for Windows' answer.
const activationTimeout = 5 * time.Second

// processLoopback is an audio client that hears one process and its
// children (Windows 10 2004 and later).
func processLoopback(pid uint32) (*object, error) {
	params := &activationParams{activationType: activationProcessLoopback, processID: pid, loopbackMode: loopbackIncludeProcessTree}
	variant := &blobVariant{vt: vtBlob, size: uint32(unsafe.Sizeof(*params)), activation: params}
	handler := &activation{methods: &activationMethods, done: make(chan struct{})}
	address := uintptr(unsafe.Pointer(handler))
	activations.Store(address, handler)
	path, err := windows.UTF16PtrFromString(`VAD\Process_Loopback`)
	if err != nil {
		return nil, err
	}
	var operation *object
	if err := procActivateAudioInterfaceAsync.Find(); err != nil {
		return nil, err
	}
	r, _, _ := procActivateAudioInterfaceAsync.Call(uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&iidAudioClient)),
		uintptr(unsafe.Pointer(variant)), address, uintptr(unsafe.Pointer(&operation)))
	if err := failed(r); err != nil {
		activations.Delete(address)
		return nil, fmt.Errorf("activating the process loopback: %w", err)
	}
	defer operation.release()
	select {
	case <-handler.done:
	case <-time.After(activationTimeout):
		return nil, errors.New("activating the process loopback: no answer")
	}
	activations.Delete(address)
	runtime.KeepAlive(variant)
	if handler.err != nil {
		return nil, fmt.Errorf("activating the process loopback: %w", handler.err)
	}
	return handler.client, nil
}

// systemLoopback is an audio client that hears everything the default
// speakers play.
func systemLoopback() (*object, error) {
	var enumerator, device, client *object
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator)))
	if err := failed(r); err != nil {
		return nil, fmt.Errorf("listing the audio devices: %w", err)
	}
	defer enumerator.release()
	if err := enumerator.call(enumeratorGetDefaultAudioEndpoint, eRender, eConsole, uintptr(unsafe.Pointer(&device))); err != nil {
		return nil, fmt.Errorf("finding the speakers: %w", err)
	}
	defer device.release()
	if err := device.call(deviceActivate, uintptr(unsafe.Pointer(&iidAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&client))); err != nil {
		return nil, fmt.Errorf("opening the speakers: %w", err)
	}
	return client, nil
}

// qpcClock turns the performance counter's 100 ns stamps WASAPI gives each
// packet into the wall clock the film's clock runs on.
type qpcClock struct {
	base     uint64
	baseTime time.Time
}

func newQPCClock() qpcClock {
	var counter, frequency int64
	procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&frequency)))
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&counter)))
	now := time.Now()
	// In two steps: counter×10⁷ would overflow after some days running.
	ticks := uint64(counter/frequency)*10_000_000 + uint64(counter%frequency)*10_000_000/uint64(frequency)
	return qpcClock{base: ticks, baseTime: now}
}

func (c qpcClock) at(stamp uint64) time.Time {
	return c.baseTime.Add(time.Duration(int64(stamp)-int64(c.base)) * 100)
}
