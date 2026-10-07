//go:build windows

// Package job ties child processes to the LingoReel through a Windows
// job object: when the job handle closes, including when this process
// crashes, Windows ends every process assigned to it. A job may also hold
// its processes to a share of the processor and to some memory each.
package job

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job is a kill-on-close job object.
type Job struct {
	handle windows.Handle
}

// Limits bound what a job's processes take; zero fields bound nothing.
type Limits struct {
	// CPU is the share of one processor the job's processes may use
	// together, held by a hard cap (0.25: a quarter of one).
	CPU float64
	// Memory is the most each process may commit, in bytes: past it, its
	// allocations fail.
	Memory uintptr
}

// Windows' names for what the x/sys package leaves out.
const (
	jobObjectLimitProcessMemory        = 0x100 // JOB_OBJECT_LIMIT_PROCESS_MEMORY
	jobObjectCPURateControlInformation = 15    // JobObjectCpuRateControlInformation
	cpuRateControlEnable               = 0x1   // JOB_OBJECT_CPU_RATE_CONTROL_ENABLE
	cpuRateControlHardCap              = 0x4   // JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP
	allProcessorGroups                 = 0xFFFF
	// fullRate is the whole machine in a CpuRate: hundredths of a percent.
	fullRate = 10000
)

// cpuRateControl is Windows' JOBOBJECT_CPU_RATE_CONTROL_INFORMATION with
// its CpuRate.
type cpuRateControl struct {
	ControlFlags uint32
	CPURate      uint32
}

var procGetActiveProcessorCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetActiveProcessorCount")

// NewKillOnClose creates a job whose processes die with its last handle.
func NewKillOnClose() (*Job, error) { return New(Limits{}) }

// New creates a kill-on-close job with limits.
func New(limits Limits) (*Job, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if limits.Memory > 0 {
		info.BasicLimitInformation.LimitFlags |= jobObjectLimitProcessMemory
		info.ProcessMemoryLimit = limits.Memory
	}
	_, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err == nil && limits.CPU > 0 {
		rate := cpuRateControl{ControlFlags: cpuRateControlEnable | cpuRateControlHardCap, CPURate: cpuRate(limits.CPU)}
		_, err = windows.SetInformationJobObject(handle, jobObjectCPURateControlInformation,
			uintptr(unsafe.Pointer(&rate)), uint32(unsafe.Sizeof(rate)))
	}
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("configuring job object: %w", err)
	}
	return &Job{handle: handle}, nil
}

// cpuRate turns a share of one processor into Windows' CpuRate, which counts
// in hundredths of a percent of every processor of the machine together.
func cpuRate(share float64) uint32 {
	processors, _, _ := procGetActiveProcessorCount.Call(allProcessorGroups)
	if processors == 0 {
		processors = 1
	}
	return uint32(min(max(math.Round(share*fullRate/float64(processors)), 1), fullRate))
}

// Assign adds a running process to the job.
func (j *Job) Assign(pid int) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("opening process %d: %w", pid, err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(j.handle, process); err != nil {
		return fmt.Errorf("assigning process %d to the job: %w", pid, err)
	}
	return nil
}

// Close releases the job, which ends every process still assigned to it.
func (j *Job) Close() error {
	if j.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(j.handle)
	j.handle = 0
	return err
}
