//go:build windows

// Package job ties child processes to the LingoReel through a Windows
// job object: when the job handle closes, including when this process
// crashes, Windows ends every process assigned to it.
package job

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job is a kill-on-close job object.
type Job struct {
	handle windows.Handle
}

// NewKillOnClose creates a job whose processes die with its last handle.
func NewKillOnClose() (*Job, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("configuring job object: %w", err)
	}
	return &Job{handle: handle}, nil
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
