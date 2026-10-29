//go:build windows && amd64

package playerwindow_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The seek preview over the track, in a 640x360 window at 100%: the panel
// from client y 302, the band (where the bar's window starts) from 268, the
// track's hit zone the panel's top 16 pixels (bar y 34..50) and its line
// from x 12 to 628. A 100 s film: x 320 is (320-12)/616 of it, 50 s.

// openPreviewing opens a window whose bar shows the seek preview, with the
// bar on screen.
func openPreviewing(t *testing.T) (host windows.HWND, remote *fakeRemote) {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Modifiers: noModifiers,
		Title:     ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, Moment: ptbr.Clock},
		NoActivate: true,
		Choose:     func(uintptr, int32, int32) uint32 { return 0 },
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	remote = newRemote(&recordingPlayer{})
	remote.clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()})
	w.Attach(remote)
	host = windows.HWND(w.Handle())
	post(t, host, win32.WM_MOUSEMOVE, 0, at(320, 340))
	testkit.Eventually(t, "the bar to show", func() bool { return windows.IsWindowVisible(controlBar(host)) })
	return host, remote
}

func (r *fakeRemote) asked() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.wanted)
}

func (r *fakeRemote) show(view thumbnail.View) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.thumbnail = view
}

// grey is a still as the session hands it: a 240x135 JPEG.
func grey(t *testing.T) thumbnail.Picture {
	t.Helper()
	picture := image.NewGray(image.Rect(0, 0, 240, 135))
	for i := range picture.Pix {
		picture.Pix[i] = 0x80
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, picture, nil); err != nil {
		t.Fatal(err)
	}
	return thumbnail.Picture{At: 50 * time.Second, Width: 240, Height: 135, JPEG: encoded.Bytes()}
}

// The pointer over the track asks the session for the moment under it,
// which is what starts the film's stills at once.
func TestTheHoverAsksForTheMomentUnderIt(t *testing.T) {
	testkit.Slow(t)
	host, remote := openPreviewing(t)
	post(t, controlBar(host), win32.WM_MOUSEMOVE, 0, at(320, 42))
	testkit.Eventually(t, "the moment under the pointer asked for", func() bool {
		for _, at := range remote.asked() {
			if at > 49500*time.Millisecond && at < 50500*time.Millisecond {
				return true
			}
		}
		return false
	})
}

// setForeground brings a window to the foreground.
var setForeground = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")

// anotherInFront puts a window of another thread in the foreground, as the
// program the user is in while the pointer passes over the bar.
func anotherInFront(t *testing.T) {
	t.Helper()
	other := open(t)
	front := windows.HWND(other.Handle())
	// Windows moves the foreground in its own time.
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		setForeground.Call(uintptr(front))
		if windows.GetForegroundWindow() == front {
			return
		}
	}
	t.Skip("Windows did not let the test bring a window to the foreground")
}

// Dragging along the track, the preview stands above the pointer, over the
// subtitles' window and the video, with the still over the time, also while
// another thread's window is in the foreground, and comes back on top when
// the subtitles' window (made on another thread, shown when it likes) comes
// over it; with no still to show the time stands alone.
func TestThePreviewFollowsADragAboveTheSubtitles(t *testing.T) {
	testkit.Slow(t)
	host, remote := openPreviewing(t)
	anotherInFront(t)
	remote.show(thumbnail.View{Status: thumbnail.Ready, Sharp: true, Picture: grey(t)})
	bar, preview := controlBar(host), ownedWindow(host, "LingoReelPreview")
	if preview == 0 {
		t.Fatal("no seek preview window")
	}
	// As in the app, the subtitles' window is up before the pointer comes to
	// the bar; coming over the preview later is what the push below plays.
	var subtitles windows.HWND
	testkit.Eventually(t, "the subtitles' window to show", func() bool {
		subtitles = ownedWindow(host, "LingoReelOverlay")
		return subtitles != 0 && windows.IsWindowVisible(subtitles)
	})
	origin, err := win32.ClientRectOnScreen(host)
	if err != nil {
		t.Fatal(err)
	}
	placed := func(x int32) func() bool {
		return func() bool {
			r, err := win32.WindowRect(preview)
			centre := (r.Left+r.Right)/2 - origin.Left
			return err == nil && windows.IsWindowVisible(preview) && centre >= x-2 && centre <= x+2
		}
	}
	post(t, bar, win32.WM_LBUTTONDOWN, 1, at(200, 42))
	testkit.Eventually(t, "the preview above x 200", placed(200))
	r, _ := win32.WindowRect(preview)
	if r.Bottom > origin.Top+308 || r.Bottom-r.Top < 68 {
		t.Fatalf("preview at %+v: want the still and the time above the track (client y 308)", r)
	}
	onTop := func() bool { return zOrder(preview) < zOrder(subtitles) && zOrder(preview) < zOrder(host) }
	testkit.Eventually(t, "the preview over the subtitles and the video", onTop)
	// Right over the preview, which "on top" would not reach from this
	// thread while another is in the foreground.
	if err := win32.SetWindowPos(subtitles, win32.WindowAbove(preview), win32.Rect{}, win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOACTIVATE|win32.SWP_NOOWNERZORDER); err != nil {
		t.Fatal(err)
	}
	if onTop() {
		t.Fatal("the subtitles' window did not come over the preview")
	}
	post(t, bar, win32.WM_MOUSEMOVE, 1, at(400, 42))
	testkit.Eventually(t, "the preview to follow to x 400", placed(400))
	testkit.Eventually(t, "the preview back over the subtitles", onTop)

	remote.show(thumbnail.View{}) // no stills for this film
	post(t, bar, win32.WM_MOUSEMOVE, 1, at(410, 42))
	testkit.Eventually(t, "the time alone", func() bool {
		r, err := win32.WindowRect(preview)
		return err == nil && placed(410)() && r.Bottom-r.Top < 40
	})
	post(t, bar, 0x0202, 0, at(410, 42)) // WM_LBUTTONUP
}
