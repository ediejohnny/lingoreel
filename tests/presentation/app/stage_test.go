package app_test

import (
	"context"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The video inside the app's window: the screens place it, ask for the
// fullscreen and keep the panel beside it; closing the window ends the video
// first.

// fakeStage is the place for the video inside the app's window.
type fakeStage struct {
	mu          sync.Mutex
	places      []place
	fullscreens []bool
	chromes     []chrome
}

type chrome struct {
	bar  settings.BarPreferences
	edit appearance.EditStyle
}

func (s *fakeStage) SetChrome(bar settings.BarPreferences, edit appearance.EditStyle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chromes = append(s.chromes, chrome{bar, edit})
}

func (s *fakeStage) styled() []chrome {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.chromes)
}

type place struct {
	visible bool
	insets  layout.Insets
}

func (s *fakeStage) Place(visible bool, insets layout.Insets) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.places = append(s.places, place{visible, insets})
}

func (s *fakeStage) SetFullscreen(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fullscreens = append(s.fullscreens, on)
}

func (s *fakeStage) placed() []place {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.places)
}

func (s *fakeStage) asked() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.fullscreens)
}

// fakeAppWindow is the video's window inside the app's.
type fakeAppWindow struct {
	nopDisplay
	closed chan struct{}
}

func (w *fakeAppWindow) Handle() uintptr           { return 4321 }
func (w *fakeAppWindow) Closed() <-chan struct{}   { return w.closed }
func (w *fakeAppWindow) Attach(watch.Remote)       {}
func (w *fakeAppWindow) SetShortcuts(shortcut.Map) {}

// stageRig is a rig whose player draws into the app's window, on a stage.
func stageRig(t *testing.T) (*rig, *fakeStage) {
	t.Helper()
	stage := &fakeStage{}
	r := newRigWith(t, func(deps *app.Deps) {
		deps.Stage = stage
		deps.Ports.OpenWindow = func(context.Context, watch.WindowRequest) (watch.Window, error) {
			return &fakeAppWindow{closed: make(chan struct{})}, nil
		}
		deps.Ports.WindowHearsKeys = true
	})
	r.plugin.drawsInHost = true
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	return r, stage
}

func TestTheVideoLayoutReachesTheStage(t *testing.T) {
	r, stage := stageRig(t)
	if err := r.api.SetVideoLayout(app.VideoLayout{Visible: true, Top: 56, Right: 380}); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SetVideoLayout(app.VideoLayout{Visible: false, Top: 56}); err != nil {
		t.Fatal(err)
	}
	want := []place{{true, layout.Insets{Top: 56, Right: 380}}, {false, layout.Insets{Top: 56}}}
	if got := stage.placed(); !slices.Equal(got, want) {
		t.Fatalf("placed %+v, want %+v", got, want)
	}
	// What no screen measures is refused, and nothing moves.
	for _, hostile := range []app.VideoLayout{
		{Visible: true, Top: -1}, {Visible: true, Right: math.NaN()}, {Visible: true, Bottom: math.Inf(1)}, {Visible: true, Left: math.Inf(-1)},
	} {
		if err := r.api.SetVideoLayout(hostile); err == nil {
			t.Errorf("SetVideoLayout(%+v) was taken", hostile)
		}
	}
	if got := stage.placed(); len(got) != len(want) {
		t.Fatalf("a refused layout reached the stage: %+v", got[len(want):])
	}
}

func TestFullscreenReachesTheStageAndComesBackToTheScreens(t *testing.T) {
	r, stage := stageRig(t)
	r.api.SetFullscreen(true)
	r.api.SetFullscreen(false)
	if got := stage.asked(); !slices.Equal(got, []bool{true, false}) {
		t.Fatalf("the stage was asked %v", got)
	}
	// Whatever asked for it (a key on the video, two clicks), the screens
	// hear the fullscreen, to hide their controls.
	r.lifecycle.FullscreenChanged(true)
	if data, told := r.ui.last(app.EventFullscreen); !told || data != true {
		t.Fatalf("the screens heard %v (%v) of the fullscreen", data, told)
	}
}

func TestTheBarsEditorOpensThePanel(t *testing.T) {
	r, _ := stageRig(t)
	r.lifecycle.ShowEditor()
	data, told := r.ui.last(app.EventPanel)
	if request, ok := data.(app.PanelRequest); !told || !ok || request.Section != app.PanelEditor {
		t.Fatalf("the screens were asked %v (%v), want the panel on the editor", data, told)
	}
}

// A video inside the app's window says so, for the screens to place it; it
// ending on its own leaves the app open, back on the video's screen.
func TestAVideoInsideTheAppWindowStaysWithTheApp(t *testing.T) {
	r, _ := stageRig(t)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	if !r.api.PlaybackState().Embedded {
		t.Fatal("a video drawn inside the app's window is not said to be")
	}
	r.plugin.current().Close()
	testkit.Eventually(t, "the end told to the screens", func() bool {
		_, told := r.ui.last(app.EventEnded)
		return told && !r.api.PlaybackState().Active
	})
	if r.ui.quitCount() != 0 {
		t.Fatal("the video ending closed the app")
	}
	// Without a stage the same player has a window of its own.
	plain := newRig(t)
	plain.plugin.drawsInHost = true
	plain.attach(t)
	if _, err := plain.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := plain.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	if plain.api.PlaybackState().Embedded {
		t.Fatal("without a stage the video was said to be inside the app's window")
	}
}

// Closing the window with a video playing ends the video first (it is drawn
// inside the window), then quits; asked again meanwhile, the window still
// waits; with nothing playing it closes at once.
func TestTheSessionEndsBeforeTheAppCloses(t *testing.T) {
	r, _ := stageRig(t)
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	quits, activeAtQuit := 0, true
	quit := func() {
		mu.Lock()
		defer mu.Unlock()
		quits++
		activeAtQuit = r.api.PlaybackState().Active
	}
	if !r.lifecycle.EndBeforeClosing(quit) {
		t.Fatal("the window closed with the video still playing in it")
	}
	r.lifecycle.EndBeforeClosing(quit) // a second click on X
	testkit.Eventually(t, "the app to quit", func() bool { mu.Lock(); defer mu.Unlock(); return quits > 0 })
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if quits != 1 || activeAtQuit {
		t.Fatalf("quit %d times, the video still playing then: %v", quits, activeAtQuit)
	}
	if r.lifecycle.EndBeforeClosing(func() { t.Error("quit again with nothing playing") }) {
		t.Fatal("with nothing playing the window still waits")
	}
}

func TestThePanelIsRemembered(t *testing.T) {
	r, _ := stageRig(t)
	if err := r.api.SetPanel(app.PanelState{Open: true, Width: 420}); err != nil {
		t.Fatal(err)
	}
	if got := r.api.Preferences().Panel; got != (app.PanelState{Open: true, Width: 420}) {
		t.Fatalf("preferences say the panel is %+v", got)
	}
	r.settings.mu.Lock()
	saved := r.settings.config.Panel
	r.settings.mu.Unlock()
	if !saved.Open || saved.Width != 420 {
		t.Fatalf("saved panel %+v", saved)
	}
	// The default width is zero; a width no panel has is refused.
	if err := r.api.SetPanel(app.PanelState{Open: false}); err != nil {
		t.Fatal(err)
	}
	for _, width := range []float64{100, 279.9, 1600.1, math.NaN(), math.Inf(1), -420} {
		if err := r.api.SetPanel(app.PanelState{Open: true, Width: width}); err == nil {
			t.Errorf("a panel %v wide was taken", width)
		}
	}
	if got := r.api.Preferences().Panel; got != (app.PanelState{}) {
		t.Fatalf("a refused width changed the panel to %+v", got)
	}
}
