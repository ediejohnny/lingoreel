//go:build windows

package potplayer

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// PotPlayer answers a WM_USER message carrying one of these codes in its
// wparam; the measurements behind each one are in docs/plugins-de-player.md.
const (
	codeGetVolume      = 0x5000 // answer: 0 to 100
	codeSetVolume      = 0x5001 // lparam: 0 to 100
	codeGetTotalTime   = 0x5002 // answer: milliseconds, 0 before the video opens
	codeGetCurrentTime = 0x5004 // answer: milliseconds, refreshed about twice a second
	codeSeek           = 0x5005 // lparam: milliseconds, exact
	codeGetStatus      = 0x5006 // answer: statusPlaying, statusPaused or stopped
	codeSetStatus      = 0x5007 // lparam: statusPlaying or statusPaused
	codeSendKey        = 0x5010 // lparam: a virtual key, for what has no code
)

// The play states PotPlayer reports and accepts; anything else is stopped.
const (
	statusPaused  = 1
	statusPlaying = 2
)

// The keys for what PotPlayer has no message code for.
const (
	keyNextFrame = 'F'
	keyFaster    = 'C' // one step of speedStep faster
	keySlower    = 'X' // one step slower
	keyNormal    = 'Z' // back to 1.00x
)

// answerTimeout bounds every message: a PotPlayer busy opening a big file
// still answers within it, and a hung one is given up on at once.
const answerTimeout = time.Second

// volumeScale is PotPlayer's loudest.
const volumeScale = 100

// ask sends one message and returns PotPlayer's answer. It reports false
// when the window is gone or stopped reading its messages, which is what
// tells a lost player from one that answered.
func ask(hwnd windows.HWND, code, value uintptr) (int64, bool) {
	if hwnd == 0 {
		return 0, false
	}
	answer, ok := win32.SendMessageTimeout(hwnd, win32.WM_USER, code, value, uint32(answerTimeout/time.Millisecond))
	if !ok {
		return 0, false
	}
	// PotPlayer answers with a 32-bit value; -1 marks what it cannot tell.
	return int64(int32(answer)), true
}

// milliseconds converts PotPlayer's times, never letting a negative answer
// ("unknown") become a position.
func milliseconds(value int64) time.Duration {
	if value < 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}

// mainClasses are the class names of PotPlayer's main window, 64-bit first.
var mainClasses = []string{"PotPlayer64", "PotPlayer"}

// enumeration carries the search across the Windows callbacks, which take
// no Go pointer; the lock makes one search at a time.
var enumeration struct {
	sync.Mutex
	processID  uint32
	main       windows.HWND
	candidates []AreaCandidate
}

var mainProc = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	var owner uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err != nil || owner != enumeration.processID {
		return 1 // keep enumerating
	}
	if !windows.IsWindowVisible(hwnd) || !isMainClass(classOf(hwnd)) {
		return 1
	}
	enumeration.main = hwnd
	return 0 // found
})

var childProc = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	candidate := AreaCandidate{Handle: uintptr(hwnd), Class: classOf(hwnd), Visible: windows.IsWindowVisible(hwnd)}
	if r, err := win32.ClientRectOnScreen(hwnd); err == nil {
		candidate.Width, candidate.Height = int(r.Right-r.Left), int(r.Bottom-r.Top)
	}
	enumeration.candidates = append(enumeration.candidates, candidate)
	return 1
})

// findMainWindow returns the visible main window of a PotPlayer process.
func findMainWindow(processID uint32) (windows.HWND, bool) {
	enumeration.Lock()
	defer enumeration.Unlock()
	enumeration.processID, enumeration.main = processID, 0
	// A failed enumeration leaves no window, which reads as "not yet".
	_ = windows.EnumWindows(mainProc, nil)
	return enumeration.main, enumeration.main != 0
}

// findVideoArea returns the child window of main that holds the picture.
func findVideoArea(main windows.HWND) (uintptr, bool) {
	if main == 0 || !windows.IsWindow(main) {
		return 0, false
	}
	enumeration.Lock()
	defer enumeration.Unlock()
	enumeration.candidates = nil
	windows.EnumChildWindows(main, childProc, nil)
	return ChooseArea(enumeration.candidates)
}

func classOf(hwnd windows.HWND) string {
	var buffer [128]uint16
	copied, err := windows.GetClassName(hwnd, &buffer[0], int32(len(buffer)))
	if err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:copied])
}

func isMainClass(class string) bool {
	for _, name := range mainClasses {
		if strings.EqualFold(class, name) {
			return true
		}
	}
	return false
}
