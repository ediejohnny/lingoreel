//go:build windows

package win32

import "golang.org/x/sys/windows"

// idRetry is MessageBox's answer for Retry.
const idRetry = 4

// AskRetry shows Windows' own message box, in front of every window, with
// Retry and Cancel, and tells whether Retry was chosen. It needs no window
// of the app's, so it serves while the app closes.
func AskRetry(title, text string) bool {
	caption, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	body, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return false
	}
	answer, _ := windows.MessageBox(0, body, caption, windows.MB_RETRYCANCEL|windows.MB_ICONWARNING|windows.MB_TOPMOST|windows.MB_SETFOREGROUND)
	return answer == idRetry
}
