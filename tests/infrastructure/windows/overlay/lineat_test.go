//go:build windows && amd64

package overlay_test

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// hits scans the player's client area and tells where each line answers.
type hits struct {
	count           [2]int
	top, bottom     [2]float64
	upperThirdLines int
}

func (h hits) String() string {
	return fmt.Sprintf("top line %d points (y %.0f–%.0f), bottom line %d points (y %.0f–%.0f), %d in the upper third",
		h.count[0], h.top[0], h.bottom[0], h.count[1], h.top[1], h.bottom[1], h.upperThirdLines)
}

func (s *scene) scan() hits {
	s.t.Helper()
	r, err := win32.ClientRectOnScreen(s.target)
	if err != nil {
		s.t.Fatal(err)
	}
	width, height := float64(r.Right-r.Left), float64(r.Bottom-r.Top)
	h := hits{top: [2]float64{height, height}}
	for y := 0.0; y < height; y += 2 {
		for x := 0.0; x < width; x += 2 {
			line, hit := s.screen.LineAt(x, y)
			if !hit {
				continue
			}
			h.count[line]++
			h.top[line] = min(h.top[line], y)
			h.bottom[line] = max(h.bottom[line], y)
			if y < height/3 {
				h.upperThirdLines++
			}
		}
	}
	return h
}

// settled captures the player's client area once two captures agree.
func settled(t *testing.T, target windows.HWND) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	time.Sleep(300 * time.Millisecond) // let the posted drawing start
	skipIfCovered(t, target)
	previous := captureClient(t, target)
	for {
		time.Sleep(150 * time.Millisecond)
		shot := captureClient(t, target)
		if changedPixels(previous, shot) == 0 {
			return shot
		}
		if time.Now().After(deadline) {
			t.Fatal("the screen over the player never settled")
		}
		previous = shot
	}
}

// The line being edited is outlined on the video, and only while it shows.
func TestTheOutlineReachesTheScreen(t *testing.T) {
	testkit.Slow(t)
	player := launchFakePlayer(t, true)
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: player.pid(), Look: defaultLook(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer screen.Close()
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	target := windows.HWND(owner)

	plain := session.Screen{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true}
	if err := screen.Show(plain); err != nil {
		t.Fatal(err)
	}
	before := settled(t, target)
	outlined := plain
	outlined.Highlighted, outlined.Highlight = true, session.LineTranslation
	if err := screen.Show(outlined); err != nil {
		t.Fatal(err)
	}
	if changed := changedPixels(before, settled(t, target)); changed < 50 {
		t.Fatalf("only %d pixels changed with the outline", changed)
	}
	// The outlined line hidden: no outline left, the picture of a hidden line.
	outlined.TranslationVisible = false
	plain.TranslationVisible = false
	if err := screen.Show(plain); err != nil {
		t.Fatal(err)
	}
	hidden := settled(t, target)
	if err := screen.Show(outlined); err != nil {
		t.Fatal(err)
	}
	if changed := changedPixels(hidden, settled(t, target)); changed != 0 {
		t.Fatalf("%d pixels changed: an outline around a hidden line", changed)
	}
}

func TestLineAtFindsTheDrawnLines(t *testing.T) {
	testkit.Slow(t)
	player := startFakePlayer(t)
	s := openOver(t, player.pid()) // shows both lines

	var both hits
	s.waitUntil("both lines to answer", func() bool { both = s.scan(); return both.count[0] > 0 && both.count[1] > 0 })
	// The English line is above the Portuguese one, near the bottom of the video.
	if both.bottom[session.LinePrimary] >= both.top[session.LineTranslation] || both.upperThirdLines != 0 {
		t.Fatalf("%v", both)
	}
	// Nothing answers outside the drawn lines: not the corners.
	for _, corner := range [][2]float64{{0, 0}, {1, 1}} {
		if line, hit := s.screen.LineAt(corner[0], corner[1]); hit {
			t.Fatalf("the corner %v answered line %d", corner, line)
		}
	}

	steps := []struct {
		name    string
		content session.Screen
		want    [2]bool
	}{
		{"hidden translation", session.Screen{Primary: "That's sick!", Translation: "Que loucura!"}, [2]bool{true, false}},
		{"hidden primary", session.Screen{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true, PrimaryHidden: true}, [2]bool{false, true}},
		{"no text", session.Screen{TranslationVisible: true}, [2]bool{false, false}},
	}
	for _, step := range steps {
		if err := s.screen.Show(step.content); err != nil {
			t.Fatal(err)
		}
		var got hits
		s.waitUntil(step.name, func() bool {
			got = s.scan()
			return (got.count[0] > 0) == step.want[0] && (got.count[1] > 0) == step.want[1]
		})
	}
}
