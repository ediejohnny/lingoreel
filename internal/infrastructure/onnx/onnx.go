// Package onnx runs ONNX models through ONNX Runtime's C API, loaded from
// its DLL at run time, without cgo: the DLL is an optional download, so the
// app opens and works without it. Only what a speech model needs is here:
// one float tensor in, one float tensor out, on the CPU.
package onnx

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrRuntime reports a DLL that is not a usable ONNX Runtime.
var ErrRuntime = errors.New("onnx: not a usable ONNX Runtime")

// apiVersion is the oldest C API with everything used here; a newer DLL
// still answers it, an older one refuses.
const apiVersion = 16

// The functions used, by their index in OrtApi (onnxruntime_c_api.h); the
// table only grows at its end, so the indexes hold in every later version.
const (
	fnGetErrorMessage                  = 2
	fnCreateEnv                        = 3
	fnDisableTelemetryEvents           = 6
	fnCreateSession                    = 7
	fnRun                              = 9
	fnCreateSessionOptions             = 10
	fnSetSessionExecutionMode          = 13
	fnSetSessionGraphOptimizationLevel = 23
	fnSetIntraOpNumThreads             = 24
	fnSetInterOpNumThreads             = 25
	fnCreateTensorWithDataAsOrtValue   = 49
	fnGetTensorMutableData             = 51
	fnGetDimensionsCount               = 61
	fnGetDimensions                    = 62
	fnGetTensorTypeAndShape            = 65
	fnCreateCpuMemoryInfo              = 69
	fnReleaseEnv                       = 92
	fnReleaseStatus                    = 93
	fnReleaseMemoryInfo                = 94
	fnReleaseSession                   = 95
	fnReleaseValue                     = 96
	fnReleaseTensorTypeAndShapeInfo    = 99
	fnReleaseSessionOptions            = 100

	apiFunctions = 101
)

// Values of the C API's enums.
const (
	loggingError     = 3  // ORT_LOGGING_LEVEL_ERROR
	sequential       = 0  // ORT_SEQUENTIAL
	optimizeAll      = 99 // ORT_ENABLE_ALL
	deviceAllocator  = 0  // OrtDeviceAllocator
	memTypeDefault   = 0  // OrtMemTypeDefault
	elementFloat     = 1  // ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT
	loadSearchDLLDir = 0x00000100
	loadSearchSystem = 0x00000800
)

// Runtime is a loaded ONNX Runtime with its environment. Its sessions may
// run at the same time.
type Runtime struct {
	dll     windows.Handle
	api     *[apiFunctions]uintptr
	env     uintptr
	memory  uintptr
	version string
	once    sync.Once
}

// Load opens the ONNX Runtime DLL at path (an absolute path: Windows looks
// for its own dependencies only beside it and in System32). It turns off
// the runtime's telemetry events.
func Load(path string) (*Runtime, error) {
	dll, err := windows.LoadLibraryEx(path, 0, loadSearchDLLDir|loadSearchSystem)
	if err != nil {
		return nil, fmt.Errorf("onnx: loading %s: %w", path, err)
	}
	r := &Runtime{dll: dll}
	if err := r.start(); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

func (r *Runtime) start() error {
	getBase, err := windows.GetProcAddress(r.dll, "OrtGetApiBase")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRuntime, err)
	}
	base, _, _ := syscall.SyscallN(getBase)
	if base == 0 {
		return ErrRuntime
	}
	functions := pointer[[2]uintptr](base) // OrtApiBase: GetApi, GetVersionString
	api, _, _ := syscall.SyscallN(functions[0], apiVersion)
	if api == 0 {
		return fmt.Errorf("%w: it does not offer C API %d", ErrRuntime, apiVersion)
	}
	r.api = pointer[[apiFunctions]uintptr](api)
	if version, _, _ := syscall.SyscallN(functions[1]); version != 0 {
		r.version = windows.BytePtrToString(pointer[byte](version))
	}
	logID, _ := windows.BytePtrFromString("lingoreel")
	err = r.call(fnCreateEnv, loggingError, uintptr(unsafe.Pointer(logID)), uintptr(unsafe.Pointer(&r.env)))
	runtime.KeepAlive(logID)
	if err != nil {
		return err
	}
	if err := r.call(fnDisableTelemetryEvents, r.env); err != nil {
		return err
	}
	return r.call(fnCreateCpuMemoryInfo, deviceAllocator, memTypeDefault, uintptr(unsafe.Pointer(&r.memory)))
}

// Version is the runtime's version, such as "1.30.0".
func (r *Runtime) Version() string { return r.version }

// Close releases the environment and the DLL; its sessions must be closed
// first.
func (r *Runtime) Close() {
	r.once.Do(func() {
		if r.memory != 0 {
			r.release(fnReleaseMemoryInfo, r.memory)
		}
		if r.env != 0 {
			r.release(fnReleaseEnv, r.env)
		}
		if r.dll != 0 {
			_ = windows.FreeLibrary(r.dll)
		}
	})
}

// call runs a C API function that answers with an OrtStatus: nil when it
// worked, its message otherwise.
func (r *Runtime) call(fn int, args ...uintptr) error {
	status, _, _ := syscall.SyscallN(r.api[fn], args...)
	if status == 0 {
		return nil
	}
	message, _, _ := syscall.SyscallN(r.api[fnGetErrorMessage], status)
	text := windows.BytePtrToString(pointer[byte](message))
	r.release(fnReleaseStatus, status)
	return fmt.Errorf("onnx: %s", text)
}

func (r *Runtime) release(fn int, handle uintptr) {
	_, _, _ = syscall.SyscallN(r.api[fn], handle)
}

// Session is a model ready to run, with one float input and one float
// output, named as the model names them.
type Session struct {
	r       *Runtime
	handle  uintptr
	input   *byte
	output  *byte
	closing sync.Once
}

// Open loads the model at path to run on threads of the CPU.
func (r *Runtime) Open(path, input, output string, threads int) (*Session, error) {
	var pinner runtime.Pinner
	defer pinner.Unpin()
	options := onHeap[uintptr](&pinner)
	if err := r.call(fnCreateSessionOptions, uintptr(unsafe.Pointer(options))); err != nil {
		return nil, err
	}
	defer r.release(fnReleaseSessionOptions, *options)
	for _, set := range []struct {
		fn    int
		value uintptr
	}{
		{fnSetIntraOpNumThreads, uintptr(max(threads, 1))},
		{fnSetInterOpNumThreads, 1},
		{fnSetSessionExecutionMode, sequential},
		{fnSetSessionGraphOptimizationLevel, optimizeAll},
	} {
		if err := r.call(set.fn, *options, set.value); err != nil {
			return nil, err
		}
	}
	model, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	s := &Session{r: r}
	if s.input, err = windows.BytePtrFromString(input); err != nil {
		return nil, err
	}
	if s.output, err = windows.BytePtrFromString(output); err != nil {
		return nil, err
	}
	pinner.Pin(model)
	err = r.call(fnCreateSession, r.env, uintptr(unsafe.Pointer(model)), *options, uintptr(unsafe.Pointer(&s.handle)))
	runtime.KeepAlive(model)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Close releases the model.
func (s *Session) Close() {
	s.closing.Do(func() { s.r.release(fnReleaseSession, s.handle) })
}

// run holds what one Run hands ONNX Runtime by address and what it writes
// back: on the heap and pinned, as the address of a variable on a
// goroutine's stack moves when the stack grows.
type run struct {
	tensor, values uintptr
	inputs         [1]uintptr
	outputs        [1]uintptr
	inputNames     [1]*byte
	outputNames    [1]*byte
}

// Run feeds data, of the given shape, to the model and returns its output
// and the output's shape.
func (s *Session) Run(data []float32, shape []int64) ([]float32, []int64, error) {
	if len(data) == 0 || len(shape) == 0 {
		return nil, nil, errors.New("onnx: nothing to run")
	}
	// ONNX Runtime keeps the data's address in the tensor until it is
	// released, and the names' while it runs: they must not move.
	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(&data[0])
	pinner.Pin(&shape[0])
	pinner.Pin(s.input)
	pinner.Pin(s.output)
	at := onHeap[run](&pinner)
	if err := s.r.call(fnCreateTensorWithDataAsOrtValue, s.r.memory,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data))*4,
		uintptr(unsafe.Pointer(&shape[0])), uintptr(len(shape)), elementFloat,
		uintptr(unsafe.Pointer(&at.tensor))); err != nil {
		return nil, nil, err
	}
	defer s.r.release(fnReleaseValue, at.tensor)
	at.inputs[0] = at.tensor
	at.inputNames[0], at.outputNames[0] = s.input, s.output
	if err := s.r.call(fnRun, s.handle, 0,
		uintptr(unsafe.Pointer(&at.inputNames[0])), uintptr(unsafe.Pointer(&at.inputs[0])), 1,
		uintptr(unsafe.Pointer(&at.outputNames[0])), 1, uintptr(unsafe.Pointer(&at.outputs[0]))); err != nil {
		return nil, nil, err
	}
	output := at.outputs[0]
	defer s.r.release(fnReleaseValue, output)
	dims, err := s.shape(output)
	if err != nil {
		return nil, nil, err
	}
	count := int64(1)
	for _, d := range dims {
		count *= d
	}
	if err := s.r.call(fnGetTensorMutableData, output, uintptr(unsafe.Pointer(&at.values))); err != nil {
		return nil, nil, err
	}
	result := make([]float32, count)
	if count > 0 {
		copy(result, unsafe.Slice(pointer[float32](at.values), count))
	}
	return result, dims, nil
}

func (s *Session) shape(value uintptr) ([]int64, error) {
	var pinner runtime.Pinner
	defer pinner.Unpin()
	info, count := onHeap[uintptr](&pinner), onHeap[uintptr](&pinner)
	if err := s.r.call(fnGetTensorTypeAndShape, value, uintptr(unsafe.Pointer(info))); err != nil {
		return nil, err
	}
	defer s.r.release(fnReleaseTensorTypeAndShapeInfo, *info)
	if err := s.r.call(fnGetDimensionsCount, *info, uintptr(unsafe.Pointer(count))); err != nil {
		return nil, err
	}
	dims := make([]int64, *count)
	if *count == 0 {
		return dims, nil
	}
	pinner.Pin(&dims[0])
	if err := s.r.call(fnGetDimensions, *info, uintptr(unsafe.Pointer(&dims[0])), *count); err != nil {
		return nil, err
	}
	return dims, nil
}

// onHeap is a new zero T that ONNX Runtime may write to by address: pinned,
// which also keeps it off the goroutine's stack.
func onHeap[T any](pinner *runtime.Pinner) *T {
	v := new(T)
	pinner.Pin(v)
	return v
}

// pointer turns an address ONNX Runtime handed out (memory it owns, outside
// Go's heap) into a pointer.
func pointer[T any](address uintptr) *T {
	return *(**T)(unsafe.Pointer(&address))
}
