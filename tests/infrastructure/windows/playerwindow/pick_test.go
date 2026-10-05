//go:build windows && amd64

package playerwindow_test

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// wmNull does nothing; sent after posted messages, it returns once they ran.
const wmNull = 0x0000

// clickDown clicks down the middle of the window, every few pixels.
func clickDown(t *testing.T, host windows.HWND) {
	t.Helper()
	for y := 0; y < offScreen.Height; y += 4 {
		post(t, host, win32.WM_LBUTTONDOWN, 0, at(offScreen.Width/2, y))
	}
}

func TestAClickPicksTheLineOnlyWhilePaused(t *testing.T) {
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Look:       overlay.Look{Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle()},
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime},
		NoActivate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	host := windows.HWND(w.Handle())
	remote := newRemote(&recordingPlayer{})
	w.Attach(remote)
	// Only the top line has text: a click can only pick it.
	if err := w.Show(session.Screen{Primary: "That's sick!", TranslationVisible: true}); err != nil {
		t.Fatal(err)
	}
	observe := func(status playback.Status) {
		remote.clock.Observe(playback.Sample{Position: 10 * time.Second, Duration: 100 * time.Second, Speed: 1, Status: status, ObservedAt: time.Now()})
	}

	// Paused, the line drawn answers a click.
	observe(playback.StatusPaused)
	rounds := 0
	eventually(t, "a pick", func() bool {
		clickDown(t, host)
		rounds++
		return len(remote.selections()) > 0
	})
	sendMessage.Call(uintptr(host), wmNull, 0, 0)
	time.Sleep(200 * time.Millisecond) // the worker running the picks
	got := remote.selections()
	pausedPicks := len(got)

	// Playing, the same clicks pick nothing.
	observe(playback.StatusPlaying)
	clickDown(t, host)
	sendMessage.Call(uintptr(host), wmNull, 0, 0)
	time.Sleep(200 * time.Millisecond)
	if after := remote.selections(); len(after) != pausedPicks {
		t.Fatalf("clicks while playing picked %v", after[pausedPicks:])
	}
	// The line is a few dozen pixels tall: some clicks, never all of them.
	if len(got) >= rounds*offScreen.Height/4 {
		t.Fatalf("every click picked a line: %v", got)
	}
	for _, line := range got {
		if line != session.LinePrimary {
			t.Fatalf("picked %v, want only the top line", got)
		}
	}
}
