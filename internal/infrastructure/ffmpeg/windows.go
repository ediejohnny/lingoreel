//go:build windows

package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
)

// background runs FFmpeg with no console window and at the idle priority:
// it only takes what the video leaves free.
func background() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.IDLE_PRIORITY_CLASS}
}

// jobs are where every FFmpeg runs, one per kind of run; their handles live
// as long as the app, and Windows ends what is in them when the app ends.
var jobs struct {
	once sync.Once
	of   [runKinds]*job.Job
}

// limits are what each kind of run may take.
var limits = [runKinds]job.Limits{
	plainRun: {},
	stillRun: {Memory: stillMemory},
	pacedRun: {Memory: stillMemory, CPU: pacedShare},
}

// contain puts a running FFmpeg in the job of its kind, and tells whether
// it is there. One that ended at once is already gone, and a job that could
// not be made leaves it to its context.
func contain(process *os.Process, kind runKind) bool {
	jobs.once.Do(func() {
		for kind := range jobs.of {
			jobs.of[kind], _ = job.New(limits[kind])
		}
	})
	return jobs.of[kind] != nil && jobs.of[kind].Assign(process.Pid) == nil
}

var procGetProcessIoCounters = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessIoCounters")

// readPoll is how often a still's reads are looked at: a still reading
// from the cache reads 24 MB in about as long.
const readPoll = 20 * time.Millisecond

// readWatch looks at what a running FFmpeg reads.
type readWatch struct {
	process windows.Handle
	done    chan struct{}
	ended   sync.WaitGroup
	over    bool
	last    windows.IO_COUNTERS
}

// watchReads looks at what a running FFmpeg reads until stop, calling over
// (once) when it reads past its bounds.
func watchReads(process *os.Process, over func()) *readWatch {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(process.Pid))
	if err != nil {
		return &readWatch{}
	}
	w := &readWatch{process: handle, done: make(chan struct{})}
	w.ended.Add(1)
	go func() {
		defer w.ended.Done()
		ticker := time.NewTicker(readPoll)
		defer ticker.Stop()
		for {
			select {
			case <-w.done:
				return
			case <-ticker.C:
				if w.read() && !w.over && tooMuch(w.last.ReadTransferCount, w.last.ReadOperationCount) {
					w.over = true
					over()
				}
			}
		}
	}()
	return w
}

func (w *readWatch) read() bool {
	ok, _, _ := procGetProcessIoCounters.Call(uintptr(w.process), uintptr(unsafe.Pointer(&w.last)))
	return ok != 0
}

// stop ends the watch and tells what the process read, in bytes and in
// reads, and whether that went past its bounds.
func (w *readWatch) stop() reads {
	if w.process == 0 {
		return reads{}
	}
	close(w.done)
	w.ended.Wait()
	w.read()
	windows.CloseHandle(w.process)
	w.process = 0
	return reads{bytes: w.last.ReadTransferCount, count: w.last.ReadOperationCount, over: w.over}
}

// candidates are the places FFmpeg is usually installed on Windows: next to
// the app, on the PATH, where winget puts it, and the usual folders.
func candidates() []string {
	var found []string
	if app, err := os.Executable(); err == nil {
		found = append(found, filepath.Join(filepath.Dir(app), "ffmpeg.exe"))
	}
	if onPath, err := exec.LookPath("ffmpeg"); err == nil {
		found = append(found, onPath)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		found = append(found, filepath.Join(local, `Microsoft\WinGet\Links\ffmpeg.exe`))
		packages, _ := filepath.Glob(filepath.Join(local, `Microsoft\WinGet\Packages\*FFmpeg*\*\bin\ffmpeg.exe`))
		found = append(found, packages...)
	}
	if programs := os.Getenv("ProgramFiles"); programs != "" {
		found = append(found, filepath.Join(programs, `ffmpeg\bin\ffmpeg.exe`))
	}
	if drive := os.Getenv("SystemDrive"); drive != "" {
		found = append(found, filepath.Join(drive+`\`, `ffmpeg\bin\ffmpeg.exe`))
	}
	return found
}
